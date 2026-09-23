package rootly

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func TestNoEmptyInterfaceTypes(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "schema.gen.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse schema.gen.go: %v", err)
	}

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			ast.Walk(&interfaceVisitor{
				t:              t,
				fset:           fset,
				path:           []string{typeSpec.Name.Name},
				enclosingField: typeSpec.Type,
			}, typeSpec.Type)
		}
	}
}

type interfaceVisitor struct {
	t              *testing.T
	fset           *token.FileSet
	path           []string
	enclosingField ast.Expr
}

func (v *interfaceVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.Field:
		if len(n.Names) == 0 {
			return &interfaceVisitor{t: v.t, fset: v.fset, path: v.path, enclosingField: n.Type}
		}
		for _, name := range n.Names {
			sub := append(append([]string{}, v.path...), name.Name)
			ast.Walk(&interfaceVisitor{t: v.t, fset: v.fset, path: sub, enclosingField: n.Type}, n.Type)
		}
		return nil
	case *ast.InterfaceType:
		if n.Methods.NumFields() != 0 {
			return v
		}
		v.report(n.Pos(), "interface{}")
		return nil
	case *ast.Ident:
		if n.Name == "any" {
			v.report(n.Pos(), "any")
		}
		return nil
	}
	return v
}

func (v *interfaceVisitor) report(pos token.Pos, kind string) {
	p := v.fset.Position(pos)
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, v.fset, v.enclosingField)
	v.t.Errorf("%s:%d: %s is `%s` - replace bare %s with a concrete type",
		p.Filename, p.Line, strings.Join(v.path, "."), buf.String(), kind)
}
