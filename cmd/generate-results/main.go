// Command generate-results generates wrapper types that embed generated types
// and add convenience methods for nil-safe field access.
//
// Usage:
//
//	go run ./cmd/generate-results
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

// TypeInfo holds information about a generated type
type TypeInfo struct {
	Name           string       // e.g., "GetAppData"
	WrapperName    string       // e.g., "AppResult"
	EmbeddedField  string       // e.g., "GetAppData" (for embedding)
	Fields         []FieldInfo  // All fields in the type
	HasMetadata    bool         // Whether type has a Metadata field
	MetadataFields []FieldInfo  // Fields within Metadata struct
	HasRecordData  bool         // Whether type has Data *[]QuickbaseRecord
	IsArrayItem    bool         // Whether this is an array item type (ends in "Item")
}

// FieldInfo holds information about a field
type FieldInfo struct {
	Name       string // Field name, e.g., "Description"
	Type       string // Go type, e.g., "*string"
	BaseType   string // Base type without pointer, e.g., "string"
	IsPtr      bool   // Whether field is a pointer
	IsSimple   bool   // Whether the base type is simple enough for accessor methods
}

func main() {
	// Parse the generated file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated/quickbase.gen.go", nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file: %v\n", err)
		os.Exit(1)
	}

	// Find all Data and Item types
	types := extractTypes(file)

	// Generate wrapper code
	code, err := generateWrappers(types)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating wrappers: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if err := os.WriteFile("client/results_generated.go", []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d wrapper types in client/results_generated.go\n", len(types))
}

// extractTypes finds all Data and Item types in the AST
func extractTypes(file *ast.File) []TypeInfo {
	var types []TypeInfo

	// Patterns for types we want to wrap
	dataPattern := regexp.MustCompile(`^[A-Z]\w+Data$`)
	itemPattern := regexp.MustCompile(`^(Get\w+Item|[A-Z]\w+Item)$`)

	// Skip patterns - internal/nested types we don't want to wrap
	skipPattern := regexp.MustCompile(`^(\w+Data_\w+|\w+Item_\w+|\w+JSONBody\w*)$`)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			name := typeSpec.Name.Name

			// Skip internal/nested types
			if skipPattern.MatchString(name) {
				continue
			}

			// Check if it's a Data or Item type
			isData := dataPattern.MatchString(name)
			isItem := itemPattern.MatchString(name) && !strings.Contains(name, "_")

			if !isData && !isItem {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			typeInfo := TypeInfo{
				Name:          name,
				WrapperName:   generateWrapperName(name, isItem),
				EmbeddedField: name,
				IsArrayItem:   isItem,
			}

			// Extract fields
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					continue // Skip embedded fields
				}

				fieldName := field.Names[0].Name
				if fieldName == "AdditionalProperties" {
					continue // Skip this internal field
				}

				fieldType := typeToString(field.Type)
				isPtr := strings.HasPrefix(fieldType, "*")
				baseType := strings.TrimPrefix(fieldType, "*")

				fieldInfo := FieldInfo{
					Name:     fieldName,
					Type:     fieldType,
					BaseType: baseType,
					IsPtr:    isPtr,
					IsSimple: isSimpleType(baseType),
				}

				typeInfo.Fields = append(typeInfo.Fields, fieldInfo)

				// Check for special fields
				if fieldName == "Metadata" {
					typeInfo.HasMetadata = true
				}
				if fieldName == "Data" && strings.Contains(fieldType, "QuickbaseRecord") {
					typeInfo.HasRecordData = true
				}
			}

			types = append(types, typeInfo)
		}
	}

	// Sort by name for consistent output
	sort.Slice(types, func(i, j int) bool {
		return types[i].Name < types[j].Name
	})

	return types
}

// generateWrapperName creates a wrapper type name from a generated type name
func generateWrapperName(name string, isItem bool) string {
	if isItem {
		// GetFieldsItem -> FieldsItem, GetAppTablesItem -> AppTablesItem
		name = strings.TrimPrefix(name, "Get")
		return name
	}

	// GetAppData -> AppResult, RunQueryData -> QueryResult
	name = strings.TrimPrefix(name, "Get")
	name = strings.TrimSuffix(name, "Data")
	return name + "Result"
}

// typeToString converts an AST type to a string representation
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.SelectorExpr:
		return typeToString(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{...}"
	default:
		return "unknown"
	}
}

// isSimpleType returns true if the type is simple enough for accessor methods.
// Only primitives and slices of primitives qualify - named types from generated
// package are accessible directly via embedding.
func isSimpleType(t string) bool {
	// Check for slices of primitives
	if strings.HasPrefix(t, "[]") {
		elementType := strings.TrimPrefix(t, "[]")
		return isPrimitive(elementType)
	}
	return isPrimitive(t)
}

// isPrimitive returns true for Go primitive types
func isPrimitive(t string) bool {
	switch t {
	case "string", "bool",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return true
	default:
		return false
	}
}

// generateWrappers generates the wrapper code
func generateWrappers(types []TypeInfo) (string, error) {
	const tmpl = `// Code generated by generate-results. DO NOT EDIT.

package client

import (
	"github.com/DrewBradfordXYZ/quickbase-go/generated"
)

{{range $type := .}}
// {{$type.WrapperName}} wraps {{$type.Name}} with convenience methods.
// All fields from {{$type.Name}} are accessible via embedding.
type {{$type.WrapperName}} struct {
	*generated.{{$type.Name}}
}
{{range $field := $type.Fields}}{{if and $field.IsPtr $field.IsSimple}}
// {{$field.Name}} returns the {{$field.Name}} field value, or zero value if nil.
func (r *{{$type.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.{{$type.EmbeddedField}} == nil || r.{{$type.EmbeddedField}}.{{$field.Name}} == nil {
		return {{zeroValue $field.BaseType}}
	}
	return *r.{{$type.EmbeddedField}}.{{$field.Name}}
}
{{end}}{{end}}
{{if $type.HasRecordData}}
// Records returns the record data as unwrapped maps.
// Returns nil if Data is nil.
func (r *{{$type.WrapperName}}) Records() []map[string]any {
	if r == nil || r.{{$type.EmbeddedField}} == nil || r.{{$type.EmbeddedField}}.Data == nil {
		return nil
	}
	return unwrapRecords(*r.{{$type.EmbeddedField}}.Data)
}
{{end}}
{{end}}
`

	funcMap := template.FuncMap{
		"zeroValue": func(t string) string {
			switch t {
			case "string":
				return `""`
			case "int", "int32", "int64", "float32", "float64":
				return "0"
			case "bool":
				return "false"
			default:
				if strings.HasPrefix(t, "[]") {
					return "nil"
				}
				return "nil"
			}
		},
	}

	t, err := template.New("wrappers").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, types); err != nil {
		return "", err
	}

	return buf.String(), nil
}
