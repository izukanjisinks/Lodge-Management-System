// Command gen-interfaces parses each internal/services/*_service.go file, extracts
// the exported methods of the *XService type, and generates for each service:
//
//   internal/interfaces/<svc>_interface.go        the interface (ctx-first)
//   internal/middleware/logger/<svc>_logger.go    a zap logging decorator
//   internal/middleware/telemetry/<svc>_telemetry.go an OTel span decorator
//
// It injects `ctx context.Context` as the first parameter of every method. Types
// are rendered from the AST and any selector-qualified package (models., repository.,
// etc.) is collected so the generated files import exactly what they use.
//
// This generator only READS service signatures; it never rewrites service bodies.
// Threading ctx through the service implementations is a separate step.
//
// Run from the repo root:  go run ./cmd/gen-interfaces
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	servicesDir  = "internal/services"
	ifaceDir     = "internal/interfaces"
	loggerDir    = "internal/middleware/logger"
	telemetryDir = "internal/middleware/telemetry"
	modulePath   = "lodge-system"
)

// param is one parameter or result: an optional name and its rendered type.
type param struct {
	name string
	typ  string
}

// method is one exported service method.
type method struct {
	name    string
	params  []param
	results []param
	hasCtx  bool
}

// service is a parsed *XService with its exported methods.
type service struct {
	structName string // e.g. "OrderService"
	base       string // e.g. "Order"  (structName without the Service suffix)
	fileBase   string // e.g. "order_service" (source file basename without .go)
	methods    []method
	// import paths referenced by param/result types, keyed by the local package
	// name used in the source (e.g. "models" -> "lodge-system/internal/models").
	imports map[string]string
}

func main() {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(servicesDir)
	must(err)

	// A service struct's methods can be split across several files in the package
	// (e.g. MealCollectionService lives in meal_session/_card/_collect_service.go),
	// so we do two passes: parse every file once, then aggregate methods per struct
	// across all files. Imports are resolved per-file as methods are collected.
	type parsed struct {
		f       *ast.File
		imports map[string]string
	}
	var files []parsed
	byStruct := map[string]*service{}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, "_service.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(servicesDir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		must(err)
		files = append(files, parsed{f: f, imports: collectImports(f)})
		for _, st := range structTypesInFile(f) {
			if _, seen := byStruct[st]; !seen {
				byStruct[st] = &service{
					structName: st,
					base:       strings.TrimSuffix(st, "Service"),
					fileBase:   strings.TrimSuffix(name, ".go"),
					imports:    map[string]string{},
				}
			}
		}
	}

	// Second pass: collect methods for each known struct from every file.
	for _, p := range files {
		for _, svc := range byStruct {
			collectMethods(p.f, svc, p.imports)
		}
	}

	var services []*service
	for _, svc := range byStruct {
		if len(svc.methods) > 0 {
			services = append(services, svc)
		}
	}
	sort.Slice(services, func(i, j int) bool { return services[i].structName < services[j].structName })

	for _, svc := range services {
		writeInterface(svc)
		writeLogger(svc)
		writeTelemetry(svc)
		fmt.Printf("generated %-30s (%d methods)\n", svc.structName, len(svc.methods))
	}
	fmt.Printf("\n%d services generated\n", len(services))
}

// collectImports maps local package name -> import path for a parsed file.
func collectImports(f *ast.File) map[string]string {
	m := map[string]string{}
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			name = p[strings.LastIndex(p, "/")+1:]
		}
		m[name] = p
	}
	return m
}

// structTypesInFile returns the names of top-level types ending in "Service".
func structTypesInFile(f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := ts.Type.(*ast.StructType); ok && strings.HasSuffix(ts.Name.Name, "Service") {
				out = append(out, ts.Name.Name)
			}
		}
	}
	return out
}

// collectMethods fills svc.methods with exported methods whose receiver is *svc.structName.
func collectMethods(f *ast.File, svc *service, importsByName map[string]string) {
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 {
			continue
		}
		if !recvIs(fd.Recv.List[0].Type, svc.structName) {
			continue
		}
		if !ast.IsExported(fd.Name.Name) {
			continue
		}
		// Skip methods that reference a same-package (unqualified, non-builtin) named
		// type in their params/results — e.g. Set*(svc *BookingService) DI wiring
		// methods, or RegisterApprover(BookingRequestApprover). These can't live in
		// the interfaces package (import cycle) and aren't called by handlers.
		if referencesLocalNamedType(fd.Type) {
			fmt.Printf("  skip %s.%s (references a services-package type)\n", svc.structName, fd.Name.Name)
			continue
		}
		// Skip DI-wiring setters: Set<X>(dep) with no return value. These are called
		// once at startup in main.go, never through the interface. A real operation
		// that happens to start with "Set" (SetStatus, SetAvailability) always
		// returns something (error, etc.), which distinguishes it from a wiring setter.
		if isVoidSetter(fd) {
			fmt.Printf("  skip %s.%s (void Set* DI wiring method)\n", svc.structName, fd.Name.Name)
			continue
		}
		m := method{name: fd.Name.Name}
		hasCtx := false
		for _, p := range fd.Type.Params.List {
			typ := renderType(p.Type, svc, importsByName)
			if typ == "context.Context" {
				hasCtx = true
			}
			names := p.Names
			if len(names) == 0 {
				m.params = append(m.params, param{typ: typ})
			}
			for _, n := range names {
				m.params = append(m.params, param{name: n.Name, typ: typ})
			}
		}
		// Record whether the source already had ctx; we always emit ctx-first in
		// the interface, so skip re-adding if present.
		m.hasCtx = hasCtx
		if fd.Type.Results != nil {
			for _, r := range fd.Type.Results.List {
				typ := renderType(r.Type, svc, importsByName)
				if len(r.Names) == 0 {
					m.results = append(m.results, param{typ: typ})
				}
				for range r.Names {
					m.results = append(m.results, param{typ: typ})
				}
			}
		}
		svc.methods = append(svc.methods, m)
	}
	sort.Slice(svc.methods, func(i, j int) bool { return svc.methods[i].name < svc.methods[j].name })
}

// builtinIdents are the only bare (unqualified) identifiers we accept in a
// signature. Anything else that's an unqualified named type is a services-package
// type, which the interfaces package can't reference.
var builtinIdents = map[string]bool{
	"string": true, "bool": true, "error": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"byte": true, "rune": true, "float32": true, "float64": true, "any": true,
}

// referencesLocalNamedType reports whether any param/result type is an unqualified
// Capitalized identifier that isn't a Go builtin (i.e. a services-package type).
func referencesLocalNamedType(ft *ast.FuncType) bool {
	bad := false
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch t := e.(type) {
		case *ast.Ident:
			if !builtinIdents[t.Name] && t.Name != "" && t.Name[0] >= 'A' && t.Name[0] <= 'Z' {
				bad = true
			}
		case *ast.StarExpr:
			walk(t.X)
		case *ast.ArrayType:
			walk(t.Elt)
		case *ast.Ellipsis:
			walk(t.Elt)
		case *ast.MapType:
			walk(t.Key)
			walk(t.Value)
		case *ast.SelectorExpr:
			// qualified (pkg.Type) — always fine
		}
	}
	for _, f := range ft.Params.List {
		walk(f.Type)
	}
	if ft.Results != nil {
		for _, f := range ft.Results.List {
			walk(f.Type)
		}
	}
	return bad
}

// isVoidSetter reports whether fd looks like a DI-wiring setter: named Set<X>,
// takes exactly one parameter, and returns nothing.
func isVoidSetter(fd *ast.FuncDecl) bool {
	if !strings.HasPrefix(fd.Name.Name, "Set") {
		return false
	}
	if fd.Type.Results != nil && len(fd.Type.Results.List) > 0 {
		return false
	}
	return len(fd.Type.Params.List) == 1
}

func recvIs(expr ast.Expr, name string) bool {
	if star, ok := expr.(*ast.StarExpr); ok {
		if id, ok := star.X.(*ast.Ident); ok {
			return id.Name == name
		}
	}
	return false
}

// renderType renders an ast type expression to Go source, recording any imported
// package it references onto svc.imports.
func renderType(expr ast.Expr, svc *service, importsByName map[string]string) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + renderType(t.X, svc, importsByName)
	case *ast.ArrayType:
		return "[]" + renderType(t.Elt, svc, importsByName)
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			if path, found := importsByName[pkg.Name]; found {
				svc.imports[pkg.Name] = path
			}
			return pkg.Name + "." + t.Sel.Name
		}
		return renderType(t.X, svc, importsByName) + "." + t.Sel.Name
	case *ast.MapType:
		return "map[" + renderType(t.Key, svc, importsByName) + "]" + renderType(t.Value, svc, importsByName)
	case *ast.Ellipsis:
		return "..." + renderType(t.Elt, svc, importsByName)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)" // not expected in our services
	default:
		return "interface{}"
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func writeFormatted(path string, src []byte) {
	formatted, err := format.Source(src)
	if err != nil {
		// Write unformatted for debugging, then fail loudly.
		_ = os.WriteFile(path, src, 0o644)
		must(fmt.Errorf("format %s: %w", path, err))
	}
	must(os.WriteFile(path, formatted, 0o644))
}

// ---- rendering helpers ------------------------------------------------------

// paramList renders a method's params for a signature, always ctx-first. When
// forSignature is true, names are included (ctx, a, b); otherwise just types.
func (m method) sigParams() string {
	parts := []string{"ctx context.Context"}
	i := 0
	for _, p := range m.params {
		if p.typ == "context.Context" {
			continue // dropped; we re-add ctx-first
		}
		name := p.name
		if name == "" || name == "_" {
			name = fmt.Sprintf("a%d", i)
		}
		parts = append(parts, name+" "+p.typ)
		i++
	}
	return strings.Join(parts, ", ")
}

// callArgs renders the argument list to delegate to next, ctx-first, using the
// same synthesized names as sigParams.
func (m method) callArgs() string {
	parts := []string{"ctx"}
	i := 0
	for _, p := range m.params {
		if p.typ == "context.Context" {
			continue
		}
		name := p.name
		if name == "" || name == "_" {
			name = fmt.Sprintf("a%d", i)
		}
		if strings.HasPrefix(p.typ, "...") {
			name += "..."
		}
		parts = append(parts, name)
		i++
	}
	return strings.Join(parts, ", ")
}

func (m method) resultTypes() string {
	if len(m.results) == 0 {
		return ""
	}
	var parts []string
	for _, r := range m.results {
		parts = append(parts, r.typ)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// namedResults renders results with an `err error` name on the trailing error so
// the logger decorator can inspect it via defer. Non-error results are blanked.
func (m method) namedResultsForLogger() (sig string, hasErr bool) {
	if len(m.results) == 0 {
		return "", false
	}
	var parts []string
	for i, r := range m.results {
		if r.typ == "error" && i == len(m.results)-1 {
			parts = append(parts, "err error")
			hasErr = true
		} else {
			parts = append(parts, "_ "+r.typ)
		}
	}
	return "(" + strings.Join(parts, ", ") + ")", hasErr
}

func (m method) returnsError() bool {
	return len(m.results) > 0 && m.results[len(m.results)-1].typ == "error"
}

// importBlock renders sorted import lines for the packages the service's types use,
// plus the always-needed ones passed in extra.
func (svc *service) importBlock(extra map[string]string) string {
	all := map[string]string{}
	for k, v := range svc.imports {
		all[k] = v
	}
	for k, v := range extra {
		all[k] = v
	}
	var std, thirdparty, local []string
	for _, path := range all {
		switch {
		case path == "context" || path == "time" || path == "fmt":
			std = append(std, fmt.Sprintf("\t%q", path))
		case strings.HasPrefix(path, modulePath):
			local = append(local, fmt.Sprintf("\t%q", path))
		default:
			thirdparty = append(thirdparty, fmt.Sprintf("\t%q", path))
		}
	}
	sort.Strings(std)
	sort.Strings(thirdparty)
	sort.Strings(local)
	var b strings.Builder
	b.WriteString("import (\n")
	writeGroup := func(g []string, trailingBlank bool) {
		for _, l := range g {
			b.WriteString(l + "\n")
		}
		if trailingBlank && len(g) > 0 {
			b.WriteString("\n")
		}
	}
	writeGroup(std, len(thirdparty)+len(local) > 0)
	writeGroup(thirdparty, len(local) > 0)
	writeGroup(local, false)
	b.WriteString(")")
	return b.String()
}

func snake(structName string) string {
	// OrderService -> order ; CorProfileService -> cor_profile
	base := strings.TrimSuffix(structName, "Service")
	var b strings.Builder
	for i, r := range base {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ---- file writers -----------------------------------------------------------

func writeInterface(svc *service) {
	extra := map[string]string{"context": "context"}
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by cmd/gen-interfaces. DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package interfaces\n\n")
	fmt.Fprintf(&b, "%s\n\n", svc.importBlock(extra))
	fmt.Fprintf(&b, "// %sInterface is the contract implemented by *services.%s and its\n", svc.base, svc.structName)
	fmt.Fprintf(&b, "// logging / telemetry decorators.\n")
	fmt.Fprintf(&b, "type %sInterface interface {\n", svc.base)
	for _, m := range svc.methods {
		res := m.resultTypes()
		if res != "" {
			res = " " + res
		}
		fmt.Fprintf(&b, "\t%s(%s)%s\n", m.name, m.sigParams(), res)
	}
	fmt.Fprintf(&b, "}\n")
	writeFormatted(filepath.Join(ifaceDir, snake(svc.structName)+"_interface.go"), b.Bytes())
}

func writeLogger(svc *service) {
	extra := map[string]string{
		"context":    "context",
		"time":       "time",
		"zap":        "go.uber.org/zap",
		"interfaces": modulePath + "/internal/interfaces",
	}
	recv := "m"
	mwType := svc.base + "LoggerMiddleware"
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by cmd/gen-interfaces. DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package logger\n\n")
	fmt.Fprintf(&b, "%s\n\n", svc.importBlock(extra))
	fmt.Fprintf(&b, "type %s struct {\n\tnext interfaces.%sInterface\n}\n\n", mwType, svc.base)
	fmt.Fprintf(&b, "func New%s(next interfaces.%sInterface) interfaces.%sInterface {\n\treturn &%s{next: next}\n}\n\n", mwType, svc.base, svc.base, mwType)
	for _, m := range svc.methods {
		sig, hasErr := m.namedResultsForLogger()
		res := sig
		if res != "" {
			res = " " + res
		}
		fmt.Fprintf(&b, "func (%s *%s) %s(%s)%s {\n", recv, mwType, m.name, m.sigParams(), res)
		fmt.Fprintf(&b, "\tstart := time.Now()\n")
		if hasErr {
			fmt.Fprintf(&b, "\tdefer func() {\n")
			fmt.Fprintf(&b, "\t\tif err != nil {\n")
			fmt.Fprintf(&b, "\t\t\tzap.L().Error(%q, zap.Duration(\"took\", time.Since(start)), zap.Error(err))\n\t\t\treturn\n\t\t}\n", svc.base+"."+m.name)
			fmt.Fprintf(&b, "\t\tzap.L().Info(%q, zap.Duration(\"took\", time.Since(start)))\n", svc.base+"."+m.name)
			fmt.Fprintf(&b, "\t}()\n")
		} else {
			fmt.Fprintf(&b, "\tdefer func() {\n\t\tzap.L().Info(%q, zap.Duration(\"took\", time.Since(start)))\n\t}()\n", svc.base+"."+m.name)
		}
		ret := ""
		if len(m.results) > 0 {
			ret = "return "
		}
		fmt.Fprintf(&b, "\t%s%s.next.%s(%s)\n", ret, recv, m.name, m.callArgs())
		fmt.Fprintf(&b, "}\n\n")
	}
	writeFormatted(filepath.Join(loggerDir, snake(svc.structName)+"_logger.go"), b.Bytes())
}

func writeTelemetry(svc *service) {
	extra := map[string]string{
		"context":    "context",
		"trace":      "go.opentelemetry.io/otel/trace",
		"interfaces": modulePath + "/internal/interfaces",
	}
	recv := "m"
	mwType := svc.base + "TelemetryMiddleware"
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by cmd/gen-interfaces. DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package telemetry\n\n")
	fmt.Fprintf(&b, "%s\n\n", svc.importBlock(extra))
	fmt.Fprintf(&b, "type %s struct {\n\tnext interfaces.%sInterface\n}\n\n", mwType, svc.base)
	fmt.Fprintf(&b, "func New%s(next interfaces.%sInterface) interfaces.%sInterface {\n\treturn &%s{next: next}\n}\n\n", mwType, svc.base, svc.base, mwType)
	for _, m := range svc.methods {
		res := m.resultTypes()
		if res != "" {
			res = " " + res
		}
		fmt.Fprintf(&b, "func (%s *%s) %s(%s)%s {\n", recv, mwType, m.name, m.sigParams(), res)
		fmt.Fprintf(&b, "\ttrace.SpanFromContext(ctx).AddEvent(%q)\n", svc.base+"."+m.name)
		ret := ""
		if len(m.results) > 0 {
			ret = "return "
		}
		fmt.Fprintf(&b, "\t%s%s.next.%s(%s)\n", ret, recv, m.name, m.callArgs())
		fmt.Fprintf(&b, "}\n\n")
	}
	writeFormatted(filepath.Join(telemetryDir, snake(svc.structName)+"_telemetry.go"), b.Bytes())
}
