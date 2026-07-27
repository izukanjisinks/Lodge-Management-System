// Command fix-handlers rewrites internal/handlers/*.go, inserting the request
// context as the first argument of every call to a service method that gained a
// context.Context first parameter.
//
// It only rewrites calls of the form `h.<field>.<Method>(...)` (a selector on the
// handler receiver) whose <Method> is in the ctx-method set, and only inside a
// function that has an *http.Request parameter (whose name — usually r — supplies
// r.Context()).
//
// The ctx-method set is read from internal/interfaces/*.go so it always matches
// what the generator produced.
//
// Run from the repo root:  go run ./cmd/fix-handlers
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
	"regexp"
	"strings"
)

const handlersDir = "internal/handlers"
const interfacesDir = "internal/interfaces"

func main() {
	ctxMethods := loadCtxMethods()
	fmt.Printf("loaded %d ctx methods\n", len(ctxMethods))

	entries, err := os.ReadDir(handlersDir)
	must(err)

	total := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		n := fixFile(filepath.Join(handlersDir, name), ctxMethods)
		if n > 0 {
			fmt.Printf("%-45s fixed %d calls\n", name, n)
			total += n
		}
	}
	fmt.Printf("\ninserted r.Context() into %d handler calls\n", total)
}

var ifaceMethodRe = regexp.MustCompile(`([A-Z][A-Za-z0-9]+)\(ctx context\.Context`)

func loadCtxMethods() map[string]bool {
	set := map[string]bool{}
	entries, err := os.ReadDir(interfacesDir)
	must(err)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(interfacesDir, e.Name()))
		must(err)
		for _, m := range ifaceMethodRe.FindAllStringSubmatch(string(data), -1) {
			set[m[1]] = true
		}
	}
	return set
}

func fixFile(path string, ctxMethods map[string]bool) int {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	must(err)

	changed := 0
	// Walk each function; find its *http.Request param name; rewrite eligible calls.
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		reqName := requestParamName(fd)
		if reqName == "" {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if !ctxMethods[sel.Sel.Name] {
				return true
			}
			// The receiver must itself be a selector on the handler receiver, i.e.
			// h.<field>.Method(...). This avoids touching unrelated calls that
			// happen to share a method name.
			inner, ok := sel.X.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// Require the base to be the handler receiver `h` — i.e. h.<field>.Method(...).
			base, ok := inner.X.(*ast.Ident)
			if !ok || base.Name != "h" {
				return true
			}
			// Skip repository fields — handlers occasionally hold a repo directly, and
			// repo methods (GetByID/List/Create/…) share names with service methods but
			// never take a context. Heuristic: field name ends in "Repo"/"Repository".
			fld := inner.Sel.Name
			if strings.HasSuffix(fld, "Repo") || strings.HasSuffix(fld, "Repository") {
				return true
			}
			// Skip if first arg already looks like a context (idempotency).
			if len(call.Args) > 0 && isContextArg(call.Args[0], reqName) {
				return true
			}
			ctxArg := &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent(reqName),
					Sel: ast.NewIdent("Context"),
				},
			}
			call.Args = append([]ast.Expr{ctxArg}, call.Args...)
			changed++
			return true
		})
	}

	if changed == 0 {
		return 0
	}
	var buf bytes.Buffer
	must(format.Node(&buf, fset, f))
	formatted, err := format.Source(buf.Bytes())
	must(err)
	must(os.WriteFile(path, formatted, 0o644))
	return changed
}

// requestParamName returns the name of the *http.Request parameter of fd, or "".
func requestParamName(fd *ast.FuncDecl) string {
	if fd.Type.Params == nil {
		return ""
	}
	for _, p := range fd.Type.Params.List {
		star, ok := p.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		x, ok := sel.X.(*ast.Ident)
		if ok && x.Name == "http" && sel.Sel.Name == "Request" {
			if len(p.Names) > 0 {
				return p.Names[0].Name
			}
		}
	}
	return ""
}

// isContextArg reports whether expr already looks like r.Context() / ctx / context.*.
func isContextArg(expr ast.Expr, reqName string) bool {
	switch t := expr.(type) {
	case *ast.CallExpr:
		if sel, ok := t.Fun.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == reqName && sel.Sel.Name == "Context" {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "context" {
				return true // context.Background()/TODO()
			}
		}
	case *ast.Ident:
		if t.Name == "ctx" {
			return true
		}
	}
	return false
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
