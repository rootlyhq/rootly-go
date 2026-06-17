package rootly

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// jsonTagName extracts the JSON property name from a struct field's tag.
func jsonTagName(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	tag := strings.Trim(field.Tag.Value, "`")
	_, rest, ok := strings.Cut(tag, `json:"`)
	if !ok {
		return ""
	}
	value, _, ok := strings.Cut(rest, `"`)
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(value, ",")
	return name
}

// schemaNameFromDoc extracts the OpenAPI schema name from a "defines model for X." doc comment.
func schemaNameFromDoc(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	const pattern = " defines model for "
	for _, comment := range doc.List {
		if _, after, ok := strings.Cut(comment.Text, pattern); ok {
			return strings.TrimSuffix(after, ".")
		}
	}
	return ""
}

// fieldDescription returns the doc comment text for a field, excluding the Deprecated line,
// and strips the leading "GoFieldName " prefix that oapi-codegen adds.
func fieldDescription(field *ast.Field, marker string) string {
	if field.Doc == nil {
		return ""
	}
	prefix := ""
	if len(field.Names) > 0 {
		prefix = field.Names[0].Name + " "
	}
	var parts []string
	for _, comment := range field.Doc.List {
		if strings.Contains(comment.Text, marker) || strings.HasPrefix(comment.Text, "// Deprecated:") {
			continue
		}
		text := strings.TrimPrefix(comment.Text, "// ")
		text = strings.TrimPrefix(text, prefix)
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

func walkDeprecatedFields(expr ast.Expr, jsonPath, marker string, failures *[]string) {
	switch t := expr.(type) {
	case *ast.StructType:
		for _, field := range t.Fields.List {
			goName := "(embedded)"
			if len(field.Names) > 0 {
				goName = field.Names[0].Name
			}
			jsonName := jsonTagName(field)
			if jsonName == "" {
				jsonName = goName
			}
			fieldJsonPath := jsonPath + "/properties/" + jsonName
			if field.Doc != nil {
				for _, comment := range field.Doc.List {
					if strings.Contains(comment.Text, marker) {
						overlayTarget := "$." + strings.ReplaceAll(strings.TrimPrefix(fieldJsonPath, "#/"), "/", ".")
						desc := fieldDescription(field, marker)
						snippet := fmt.Sprintf(
							"  - target: %s\n    update:\n      description: \"%s\" # TODO: amend, or leave it as an empty string to unset it\n      x-deprecated-reason: %q # TODO: amend",
							overlayTarget, desc, desc,
						)
						*failures = append(*failures, snippet)
						break
					}
				}
			}
			walkDeprecatedFields(field.Type, fieldJsonPath, marker, failures)
		}
	case *ast.ArrayType:
		walkDeprecatedFields(t.Elt, jsonPath, marker, failures)
	case *ast.StarExpr:
		walkDeprecatedFields(t.X, jsonPath, marker, failures)
	}
}

// When generating code using `oapi-codegen`, if a field is `deprecated: true`, but no `x-deprecated-reason` is set, we get a `Deprecated: ...` Go Doc comment, but it's not as targeted as we should do, given there's no usage of the `x-deprecated-reason` extension
// To make sure that we're always providing useful deprecation messages for our users, validate that there are no deprecations without `x-deprecated-reason`
func Test_NoDeprecatedUpstreamPropertiesWithoutReason(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "schema.gen.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse schema.gen.go: %v", err)
	}

	const marker = "this property has been marked as deprecated upstream, but no `x-deprecated-reason` was set"

	var failures []string
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			schemaName := schemaNameFromDoc(genDecl.Doc)
			if schemaName == "" {
				schemaName = typeSpec.Name.Name
			}
			jsonPath := "#/components/schemas/" + schemaName
			walkDeprecatedFields(typeSpec.Type, jsonPath, marker, &failures)
		}
	}

	if len(failures) > 0 {
		t.Errorf("schema.gen.go contains %d deprecated fields without an x-deprecated-reason.\nAdd the following to overlay.yaml:\n\n%s\n",
			len(failures), strings.Join(failures, "\n\n"))
	}
}

// When generating code using `oapi-codegen`, if a type is `deprecated: true`, but no `x-deprecated-reason` is set, we get a `Deprecated: ...` Go Doc comment on the type, but it's not as targeted as we should do
// To make sure that we're always providing useful deprecation messages for our users, validate that there are no type-level deprecations without `x-deprecated-reason`
func Test_NoDeprecatedUpstreamTypesWithoutReason(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "schema.gen.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse schema.gen.go: %v", err)
	}

	const marker = "this type has been marked as deprecated upstream, but no `x-deprecated-reason` was set"

	var failures []string
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if genDecl.Doc == nil {
			continue
		}
		for _, comment := range genDecl.Doc.List {
			if strings.Contains(comment.Text, marker) {
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					schemaName := schemaNameFromDoc(genDecl.Doc)
					if schemaName == "" {
						schemaName = typeSpec.Name.Name
					}
					overlayTarget := "$.components.schemas." + schemaName
					snippet := fmt.Sprintf(
						"  - target: %s\n    update:\n      x-deprecated-reason: %q # TODO: amend",
						overlayTarget, "",
					)
					failures = append(failures, snippet)
				}
				break
			}
		}
	}

	if len(failures) > 0 {
		t.Errorf("schema.gen.go contains %d deprecated types without an x-deprecated-reason.\nAdd the following to overlay.yaml:\n\n%s\n",
			len(failures), strings.Join(failures, "\n\n"))
	}
}

// When generating code using `oapi-codegen`, if an operation is `deprecated: true`, but no `x-deprecated-reason` is set, we get a `Deprecated: ...` Go Doc comment on the interface method, but it's not as targeted as we should do
// To make sure that we're always providing useful deprecation messages for our users, validate that there are no operation-level deprecations without `x-deprecated-reason`
func Test_NoDeprecatedUpstreamOperationsWithoutReason(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "schema.gen.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse schema.gen.go: %v", err)
	}

	const marker = "this operation has been marked as deprecated upstream, but no `x-deprecated-reason` was set"

	var failures []string
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			iface, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, method := range iface.Methods.List {
				if method.Doc == nil {
					continue
				}
				for _, comment := range method.Doc.List {
					if strings.Contains(comment.Text, marker) {
						methodName := "(unknown)"
						if len(method.Names) > 0 {
							methodName = method.Names[0].Name
						}
						snippet := fmt.Sprintf(
							"  - # method: %s\n    target: \"$.paths['TODO'].TODO\"\n    update:\n      x-deprecated-reason: %q # TODO: amend",
							methodName, "",
						)
						failures = append(failures, snippet)
						break
					}
				}
			}
		}
	}

	if len(failures) > 0 {
		t.Errorf("schema.gen.go contains %d deprecated operations without an x-deprecated-reason.\nAdd the following to overlay.yaml (filling in the correct path and HTTP method):\n\n%s\n",
			len(failures), strings.Join(failures, "\n\n"))
	}
}

// https://github.com/rootlyhq/rootly-go/issues/39
func Test_OptionalQueryParameterIsNotSerializedIntoURL(t *testing.T) {
	params := ListShiftsParams{
		ScheduleIDs: []string{"schedule-1", "schedule-2"},
		UserIDs:     nil,
	}

	req, err := NewListShiftsRequest(ServerURLProduction, &params)
	if err != nil {
		t.Errorf("Expected no error when calling NewListShiftsRequest, but received %v", err)
	}

	want := "https://api.rootly.com/v1/shifts?schedule_ids%5B%5D=schedule-1&schedule_ids%5B%5D=schedule-2"
	got := req.URL.String()

	if want != got {
		t.Errorf("ListShift API URL was not correct. Expected:\n%v\nReceived:\n%v\n", want, got)
	}
}
