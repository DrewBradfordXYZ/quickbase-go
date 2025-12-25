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
	Name           string      // e.g., "GetAppData"
	WrapperName    string      // e.g., "AppResult"
	EmbeddedField  string      // e.g., "GetAppData" (for embedding)
	Fields         []FieldInfo // All fields in the type
	HasMetadata    bool        // Whether type has a Metadata field
	HasRecordData  bool        // Whether type has Data *[]QuickbaseRecord
	IsArrayItem    bool        // Whether this is an array item type (ends in "Item")
	IsNested       bool        // Whether this is a nested type (has _ in name)
}

// FieldInfo holds information about a field
type FieldInfo struct {
	Name             string // Field name, e.g., "Description"
	Type             string // Go type, e.g., "*string"
	BaseType         string // Base type without pointer, e.g., "string"
	IsPtr            bool   // Whether field is a pointer
	IsSimple         bool   // Whether the base type is simple enough for scalar accessor (pointer)
	IsSimpleRequired bool   // Whether this is a required (non-pointer) simple type
	IsPrimitiveSlice bool   // Whether this is a slice of primitives ([]string, []int, etc)
	IsWrappedType    bool   // Whether the base type has a wrapper we generated
	WrapperTypeName  string // Name of the wrapper type for this field
	IsSlice          bool   // Whether field is a slice
	SliceElement     string // Element type if slice
	IsEnum           bool   // Whether this is an enum type (string-based typedef)
}

// Global map of all struct types found in the AST
var allStructTypes = make(map[string]*ast.StructType)

// Map of generated type name -> wrapper type name
var wrapperNames = make(map[string]string)

// Map of enum type names (string-based type aliases like "type SomeEnum string")
var enumTypes = make(map[string]bool)

func main() {
	// Parse the generated file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated/quickbase.gen.go", nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file: %v\n", err)
		os.Exit(1)
	}

	// First pass: collect all struct types and enum types
	collectAllStructTypes(file)
	collectEnumTypes(file)

	// Find all Data and Item types (top-level wrappers)
	types := extractTypes(file)

	// Second pass: find nested types that need wrappers
	nestedTypes := findNestedTypesToWrap(types)
	types = append(types, nestedTypes...)

	// Sort by name for consistent output
	sort.Slice(types, func(i, j int) bool {
		return types[i].Name < types[j].Name
	})

	// Now resolve which fields reference wrapped types
	resolveWrappedFields(types)

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

// collectAllStructTypes builds a map of all struct types in the file
func collectAllStructTypes(file *ast.File) {
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

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			allStructTypes[typeSpec.Name.Name] = structType
		}
	}
}

// collectEnumTypes finds all string-based type aliases (enum types)
// These are types like "type GetAppEventsItemType string"
func collectEnumTypes(file *ast.File) {
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

			// Check if this is a type alias to string (e.g., "type SomeEnum string")
			ident, ok := typeSpec.Type.(*ast.Ident)
			if !ok {
				continue
			}

			if ident.Name == "string" {
				enumTypes[typeSpec.Name.Name] = true
			}
		}
	}
}

// extractTypes finds all Data and Item types in the AST (top-level types to wrap)
func extractTypes(file *ast.File) []TypeInfo {
	var types []TypeInfo

	// Patterns for types we want to wrap
	dataPattern := regexp.MustCompile(`^[A-Z]\w+Data$`)
	itemPattern := regexp.MustCompile(`^(Get\w+Item|[A-Z]\w+Item)$`)

	// Skip patterns - internal/nested types we don't want at top level
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

			// Skip internal/nested types at top level
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

			wrapperName := generateWrapperName(name, isItem, false)
			wrapperNames[name] = wrapperName

			typeInfo := TypeInfo{
				Name:          name,
				WrapperName:   wrapperName,
				EmbeddedField: name,
				IsArrayItem:   isItem,
				IsNested:      false,
			}

			// Extract fields
			typeInfo.Fields = extractFields(structType, name)

			// Check for special fields
			for _, f := range typeInfo.Fields {
				if f.Name == "Metadata" {
					typeInfo.HasMetadata = true
				}
				if f.Name == "Data" && strings.Contains(f.Type, "QuickbaseRecord") {
					typeInfo.HasRecordData = true
				}
			}

			types = append(types, typeInfo)
		}
	}

	return types
}

// findNestedTypesToWrap finds nested types referenced by top-level types
func findNestedTypesToWrap(topLevelTypes []TypeInfo) []TypeInfo {
	var nestedTypes []TypeInfo
	seen := make(map[string]bool)

	// Mark top-level types as seen
	for _, t := range topLevelTypes {
		seen[t.Name] = true
	}

	var findNested func(typeName string)
	findNested = func(typeName string) {
		structType, exists := allStructTypes[typeName]
		if !exists {
			return
		}

		for _, field := range structType.Fields.List {
			if len(field.Names) == 0 {
				continue
			}

			fieldType := typeToString(field.Type)
			baseType := strings.TrimPrefix(fieldType, "*")
			baseType = strings.TrimPrefix(baseType, "[]")

			// Check if this references a nested struct type (has _ in name)
			if strings.Contains(baseType, "_") && !seen[baseType] {
				if nestedStruct, exists := allStructTypes[baseType]; exists {
					seen[baseType] = true

					wrapperName := generateWrapperName(baseType, false, true)
					wrapperNames[baseType] = wrapperName

					nestedType := TypeInfo{
						Name:          baseType,
						WrapperName:   wrapperName,
						EmbeddedField: baseType,
						IsNested:      true,
						Fields:        extractFields(nestedStruct, baseType),
					}
					nestedTypes = append(nestedTypes, nestedType)

					// Recurse into this nested type
					findNested(baseType)
				}
			}
		}
	}

	// Find nested types for all top-level types
	for _, t := range topLevelTypes {
		findNested(t.Name)
	}

	return nestedTypes
}

// extractFields extracts field info from a struct type
func extractFields(structType *ast.StructType, parentTypeName string) []FieldInfo {
	var fields []FieldInfo

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
		isSlice := strings.HasPrefix(fieldType, "[]") || strings.HasPrefix(fieldType, "*[]")
		baseType := strings.TrimPrefix(fieldType, "*")
		baseType = strings.TrimPrefix(baseType, "[]")

		sliceElement := ""
		if isSlice {
			// Extract element type from slice
			if strings.HasPrefix(fieldType, "*[]") {
				sliceElement = strings.TrimPrefix(fieldType, "*[]")
			} else {
				sliceElement = strings.TrimPrefix(fieldType, "[]")
			}
		}

		// Check if this is a pointer to a slice of primitives (*[]string, *[]int)
		isPrimitiveSlice := false
		if isSlice && isPrimitive(sliceElement) && strings.HasPrefix(fieldType, "*[]") {
			isPrimitiveSlice = true
		}

		// Check if this is a required (non-pointer) simple type
		isSimpleRequired := !isPtr && !isSlice && isPrimitive(baseType)

		// Check if this is an enum type (string-based typedef)
		isEnum := isPtr && !isSlice && enumTypes[baseType]

		fieldInfo := FieldInfo{
			Name:             fieldName,
			Type:             fieldType,
			BaseType:         baseType,
			IsPtr:            isPtr,
			IsSimple:         isSimpleType(baseType) && !isPrimitiveSlice && isPtr,
			IsSimpleRequired: isSimpleRequired,
			IsPrimitiveSlice: isPrimitiveSlice,
			IsSlice:          isSlice,
			SliceElement:     sliceElement,
			IsEnum:           isEnum,
		}

		fields = append(fields, fieldInfo)
	}

	return fields
}

// resolveWrappedFields updates fields to mark which ones reference wrapped types
func resolveWrappedFields(types []TypeInfo) {
	for i := range types {
		for j := range types[i].Fields {
			field := &types[i].Fields[j]

			// Check if the base type (or slice element) has a wrapper
			typeToCheck := field.BaseType
			if field.IsSlice {
				typeToCheck = field.SliceElement
			}

			if wrapperName, exists := wrapperNames[typeToCheck]; exists {
				field.IsWrappedType = true
				field.WrapperTypeName = wrapperName
			}
		}
	}
}

// generateWrapperName creates a wrapper type name from a generated type name
func generateWrapperName(name string, isItem bool, isNested bool) string {
	if isNested {
		// GetTableReportsItem_Query -> TableReportsQuery
		// GetFieldData_Properties -> FieldProperties
		// First remove Get prefix if present
		name = strings.TrimPrefix(name, "Get")

		// Replace _ with nothing, but handle the parts
		parts := strings.Split(name, "_")
		result := ""
		for i, part := range parts {
			if i == 0 {
				// First part: remove Data/Item suffix
				part = strings.TrimSuffix(part, "Data")
				part = strings.TrimSuffix(part, "Item")
			}
			result += part
		}
		return result
	}

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

// isSimpleType returns true if the type is simple enough for scalar accessor methods.
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
	"github.com/DrewBradfordXYZ/quickbase-go/v2/generated"
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
{{end}}{{if $field.IsSimpleRequired}}
// {{$field.Name}} returns the {{$field.Name}} field value.
func (r *{{$type.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.{{$type.EmbeddedField}} == nil {
		return {{zeroValue $field.BaseType}}
	}
	return r.{{$type.EmbeddedField}}.{{$field.Name}}
}
{{end}}{{if $field.IsPrimitiveSlice}}
// {{$field.Name}} returns the {{$field.Name}} field value, or nil if not set.
func (r *{{$type.WrapperName}}) {{$field.Name}}() []{{$field.SliceElement}} {
	if r == nil || r.{{$type.EmbeddedField}} == nil || r.{{$type.EmbeddedField}}.{{$field.Name}} == nil {
		return nil
	}
	return *r.{{$type.EmbeddedField}}.{{$field.Name}}
}
{{end}}{{if and $field.IsPtr $field.IsWrappedType (not $field.IsSlice)}}
// {{$field.Name}} returns the {{$field.Name}} field as a wrapped type, or nil if not set.
func (r *{{$type.WrapperName}}) {{$field.Name}}() *{{$field.WrapperTypeName}} {
	if r == nil || r.{{$type.EmbeddedField}} == nil || r.{{$type.EmbeddedField}}.{{$field.Name}} == nil {
		return nil
	}
	return &{{$field.WrapperTypeName}}{r.{{$type.EmbeddedField}}.{{$field.Name}}}
}
{{end}}{{if and $field.IsSlice $field.IsWrappedType}}
// {{$field.Name}} returns the {{$field.Name}} field as wrapped types, or nil if not set.
func (r *{{$type.WrapperName}}) {{$field.Name}}() []*{{$field.WrapperTypeName}} {
	if r == nil || r.{{$type.EmbeddedField}} == nil {
		return nil
	}
	{{if hasPrefix $field.Type "*[]"}}if r.{{$type.EmbeddedField}}.{{$field.Name}} == nil {
		return nil
	}
	items := make([]*{{$field.WrapperTypeName}}, len(*r.{{$type.EmbeddedField}}.{{$field.Name}}))
	for i := range *r.{{$type.EmbeddedField}}.{{$field.Name}} {
		items[i] = &{{$field.WrapperTypeName}}{&(*r.{{$type.EmbeddedField}}.{{$field.Name}})[i]}
	}{{else}}items := make([]*{{$field.WrapperTypeName}}, len(r.{{$type.EmbeddedField}}.{{$field.Name}}))
	for i := range r.{{$type.EmbeddedField}}.{{$field.Name}} {
		items[i] = &{{$field.WrapperTypeName}}{&r.{{$type.EmbeddedField}}.{{$field.Name}}[i]}
	}{{end}}
	return items
}
{{end}}{{if $field.IsEnum}}
// {{$field.Name}} returns the {{$field.Name}} field value as a string, or empty string if nil.
func (r *{{$type.WrapperName}}) {{$field.Name}}() string {
	if r == nil || r.{{$type.EmbeddedField}} == nil || r.{{$type.EmbeddedField}}.{{$field.Name}} == nil {
		return ""
	}
	return string(*r.{{$type.EmbeddedField}}.{{$field.Name}})
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
		"hasPrefix": strings.HasPrefix,
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
