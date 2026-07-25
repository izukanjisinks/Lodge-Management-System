// Command add-ctx rewrites internal/services/*_service.go, adding
// `ctx context.Context` as the first parameter of every exported *XService method
// — except DI wiring methods (Set*) and any method that already has a
// context.Context first parameter. It also ensures the "context" import exists.
//
// It only touches *method declarations*; call sites are fixed afterwards by
// following compiler errors. Uses go/ast + go/printer to preserve formatting as
// much as possible (gofmt is applied on write).
//
// Run from the repo root:  go run ./cmd/add-ctx
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
	"strings"
)

const servicesDir = "internal/services"

func main() {
	entries, err := os.ReadDir(servicesDir)
	must(err)

	total := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, "_service.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(servicesDir, name)
		n := rewriteFile(path)
		if n > 0 {
			fmt.Printf("%-45s +ctx on %d methods\n", name, n)
			total += n
		}
	}
	fmt.Printf("\nadded ctx to %d methods\n", total)
}

func rewriteFile(path string) int {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	must(err)

	changed := 0
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 {
			continue
		}
		if !recvIsService(fd.Recv.List[0].Type) {
			continue
		}
		if !ast.IsExported(fd.Name.Name) {
			continue
		}
		if firstParamIsContext(fd.Type) {
			continue // already has ctx
		}
		if referencesLocalNamedType(fd.Type) {
			continue // wiring/approver methods excluded from the interface too
		}
		if isVoidSetter(fd) {
			continue // DI-wiring setter (Set<X>(dep) with no return) — excluded too
		}
		// Prepend `ctx context.Context`.
		ctxField := &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("ctx")},
			Type:  &ast.SelectorExpr{X: ast.NewIdent("context"), Sel: ast.NewIdent("Context")},
		}
		fd.Type.Params.List = append([]*ast.Field{ctxField}, fd.Type.Params.List...)
		changed++
	}

	if changed == 0 {
		return 0
	}

	ensureContextImport(f)

	var buf bytes.Buffer
	must(format.Node(&buf, fset, f))
	formatted, err := format.Source(buf.Bytes())
	must(err)
	must(os.WriteFile(path, formatted, 0o644))
	return changed
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

func recvIsService(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := star.X.(*ast.Ident)
	return ok && strings.HasSuffix(id.Name, "Service")
}

func firstParamIsContext(ft *ast.FuncType) bool {
	if len(ft.Params.List) == 0 {
		return false
	}
	sel, ok := ft.Params.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == "context" && sel.Sel.Name == "Context"
}

var builtinIdents = map[string]bool{
	"string": true, "bool": true, "error": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"byte": true, "rune": true, "float32": true, "float64": true, "any": true,
}

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

// ensureContextImport adds "context" to the import block if missing.
func ensureContextImport(f *ast.File) {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) == "context" {
			return
		}
	}
	newImport := &ast.ImportSpec{
		Path: &ast.BasicLit{Kind: token.STRING, Value: `"context"`},
	}
	// Find the first import GenDecl and prepend.
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if ok && gd.Tok == token.IMPORT {
			gd.Specs = append([]ast.Spec{newImport}, gd.Specs...)
			f.Imports = append(f.Imports, newImport)
			return
		}
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
