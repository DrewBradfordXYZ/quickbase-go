// Command generate-results generates wrapper types for Response types
// with convenience methods for nil-safe field access.
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

// ResponseInfo holds information about a Response type
type ResponseInfo struct {
	Name           string      // e.g., "GetAppResponse"
	WrapperName    string      // e.g., "AppResult"
	OperationName  string      // e.g., "GetApp"
	Fields         []FieldInfo // Fields from JSON200 struct
	HasJSON200     bool        // Whether response has JSON200 field
	JSON200IsArray bool        // Whether JSON200 is an array type
	ItemTypeName   string      // For arrays, the item type name (if named)
	HasRecordData  bool        // Whether response has Data *[]QuickbaseRecord field
}

// FieldInfo holds information about a field
type FieldInfo struct {
	Name             string // Field name, e.g., "Description"
	Type             string // Go type, e.g., "*string"
	BaseType         string // Base type without pointer, e.g., "string"
	IsPtr            bool   // Whether field is a pointer
	IsSimple         bool   // Whether the base type is simple enough for scalar accessor
	IsSimpleRequired bool   // Whether this is a required (non-pointer) simple type
	IsPrimitiveSlice bool   // Whether this is a slice of primitives
	IsWrappedType    bool   // Whether the base type has a wrapper we generated
	WrapperTypeName  string // Name of the wrapper type for this field
	IsSlice          bool   // Whether field is a slice
	SliceElement     string // Element type if slice
	IsEnum           bool   // Whether this is an enum type
	IsNestedStruct   bool   // Whether this is a nested named struct
	NestedTypeName   string // The named type for nested structs
}

// NestedTypeInfo holds info about nested types that need wrappers
type NestedTypeInfo struct {
	Name        string
	WrapperName string
	Fields      []FieldInfo
}

// ItemTypeInfo holds info about array item types that need wrappers
type ItemTypeInfo struct {
	Name        string
	WrapperName string
	Fields      []FieldInfo
}

// Global maps
var allStructTypes = make(map[string]*ast.StructType)
var enumTypes = make(map[string]bool)
var wrapperNames = make(map[string]string)
var nestedTypes []NestedTypeInfo
var itemTypes []ItemTypeInfo

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

	// Find all Response types
	responses := extractResponses(file)

	// Sort by name for consistent output
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Name < responses[j].Name
	})

	// Sort nested types
	sort.Slice(nestedTypes, func(i, j int) bool {
		return nestedTypes[i].Name < nestedTypes[j].Name
	})

	// Sort item types
	sort.Slice(itemTypes, func(i, j int) bool {
		return itemTypes[i].Name < itemTypes[j].Name
	})

	// Now resolve which fields reference wrapped types
	resolveWrappedFields(responses)

	// Generate wrapper code
	code, err := generateWrappers(responses)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating wrappers: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if err := os.WriteFile("client/results_generated.go", []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d wrapper types + %d item types + %d nested types in client/results_generated.go\n",
		len(responses), len(itemTypes), len(nestedTypes))
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

// extractResponses finds all Response types and extracts their JSON200 field info
func extractResponses(file *ast.File) []ResponseInfo {
	var responses []ResponseInfo
	seenNested := make(map[string]bool)

	// Pattern for Response types we want to wrap
	responsePattern := regexp.MustCompile(`^[A-Z]\w+Response$`)

	// No skip patterns - generate wrappers for all Response types

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

			// Must match Response pattern
			if !responsePattern.MatchString(name) {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Find JSON200 field
			var json200Field *ast.Field
			for _, field := range structType.Fields.List {
				if len(field.Names) > 0 && field.Names[0].Name == "JSON200" {
					json200Field = field
					break
				}
			}

			if json200Field == nil {
				continue
			}

			operationName := strings.TrimSuffix(name, "Response")
			wrapperName := generateWrapperName(operationName)
			wrapperNames[name] = wrapperName

			resp := ResponseInfo{
				Name:          name,
				WrapperName:   wrapperName,
				OperationName: operationName,
				HasJSON200:    true,
			}

			// Parse JSON200 field type
			json200Type := json200Field.Type

			// Check if it's a pointer
			if starExpr, ok := json200Type.(*ast.StarExpr); ok {
				json200Type = starExpr.X
			}

			// Check if it's an array
			if arrayType, ok := json200Type.(*ast.ArrayType); ok {
				resp.JSON200IsArray = true
				// Get the element type
				var itemTypeName string
				if starExpr, ok := arrayType.Elt.(*ast.StarExpr); ok {
					if ident, ok := starExpr.X.(*ast.Ident); ok {
						itemTypeName = ident.Name
					}
				} else if ident, ok := arrayType.Elt.(*ast.Ident); ok {
					itemTypeName = ident.Name
				}

				if itemTypeName != "" {
					resp.ItemTypeName = itemTypeName
					itemWrapperName := generateItemWrapperName(itemTypeName)
					wrapperNames[itemTypeName] = itemWrapperName

					// Add item type to the list if we haven't seen it
					if itemStruct, exists := allStructTypes[itemTypeName]; exists {
						itemTypes = append(itemTypes, ItemTypeInfo{
							Name:        itemTypeName,
							WrapperName: itemWrapperName,
							Fields:      extractFields(itemStruct, itemTypeName, seenNested),
						})
					}
				}
			}

			// Check if it's an inline struct
			if structExpr, ok := json200Type.(*ast.StructType); ok {
				resp.Fields = extractFields(structExpr, operationName, seenNested)
				// Check for Data *[]QuickbaseRecord field
				resp.HasRecordData = hasQuickbaseRecordData(structExpr)
			}

			// Check if it references a named type
			if ident, ok := json200Type.(*ast.Ident); ok {
				if namedStruct, exists := allStructTypes[ident.Name]; exists {
					resp.Fields = extractFields(namedStruct, operationName, seenNested)
					resp.HasRecordData = hasQuickbaseRecordData(namedStruct)
				}
			}

			responses = append(responses, resp)
		}
	}

	return responses
}

// extractFields extracts field info from a struct type
func extractFields(structType *ast.StructType, parentName string, seenNested map[string]bool) []FieldInfo {
	var fields []FieldInfo

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue // Skip embedded fields
		}

		fieldName := field.Names[0].Name
		if fieldName == "AdditionalProperties" {
			continue
		}

		fieldType := typeToString(field.Type)
		isPtr := strings.HasPrefix(fieldType, "*")
		isSlice := strings.HasPrefix(fieldType, "[]") || strings.HasPrefix(fieldType, "*[]")
		baseType := strings.TrimPrefix(fieldType, "*")
		baseType = strings.TrimPrefix(baseType, "[]")

		sliceElement := ""
		if isSlice {
			if strings.HasPrefix(fieldType, "*[]") {
				sliceElement = strings.TrimPrefix(fieldType, "*[]")
			} else {
				sliceElement = strings.TrimPrefix(fieldType, "[]")
			}
		}

		isPrimitiveSlice := isSlice && isPrimitive(sliceElement) && strings.HasPrefix(fieldType, "*[]")
		isSimpleRequired := !isPtr && !isSlice && isPrimitive(baseType)
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

		// Check if this references a named nested struct (like GetApp_200_SecurityProperties)
		typeToCheck := baseType
		if isSlice {
			typeToCheck = sliceElement
		}
		if strings.Contains(typeToCheck, "_200_") && !seenNested[typeToCheck] {
			if nestedStruct, exists := allStructTypes[typeToCheck]; exists {
				seenNested[typeToCheck] = true
				nestedWrapperName := generateNestedWrapperName(typeToCheck)
				wrapperNames[typeToCheck] = nestedWrapperName

				nestedTypes = append(nestedTypes, NestedTypeInfo{
					Name:        typeToCheck,
					WrapperName: nestedWrapperName,
					Fields:      extractFields(nestedStruct, typeToCheck, seenNested),
				})

				fieldInfo.IsNestedStruct = true
				fieldInfo.NestedTypeName = typeToCheck
				fieldInfo.IsWrappedType = true
				fieldInfo.WrapperTypeName = nestedWrapperName
			}
		}

		fields = append(fields, fieldInfo)
	}

	return fields
}

// resolveWrappedFields updates fields to mark which ones reference wrapped types
func resolveWrappedFields(responses []ResponseInfo) {
	for i := range responses {
		for j := range responses[i].Fields {
			field := &responses[i].Fields[j]

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

	// Also resolve for nested types
	for i := range nestedTypes {
		for j := range nestedTypes[i].Fields {
			field := &nestedTypes[i].Fields[j]

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

// generateWrapperName creates wrapper name from operation name
func generateWrapperName(operationName string) string {
	// GetApp -> AppResult, RunQuery -> RunQueryResult
	name := strings.TrimPrefix(operationName, "Get")
	return name + "Result"
}

// generateItemWrapperName creates wrapper name for array item types
func generateItemWrapperName(typeName string) string {
	// GetFields_200_Item -> FieldsItem
	name := strings.TrimPrefix(typeName, "Get")
	parts := strings.Split(name, "_")
	if len(parts) >= 1 {
		return parts[0] + "Item"
	}
	return name + "Item"
}

// generateNestedWrapperName creates wrapper name for nested types
func generateNestedWrapperName(typeName string) string {
	// GetApp_200_SecurityProperties -> AppSecurityProperties
	name := strings.TrimPrefix(typeName, "Get")
	name = strings.ReplaceAll(name, "_200_", "")
	name = strings.ReplaceAll(name, "_", "")
	return name
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

// isSimpleType returns true if the type is simple enough for scalar accessor methods
func isSimpleType(t string) bool {
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

// hasQuickbaseRecordData checks if a struct has a Data field of type *[]QuickbaseRecord
func hasQuickbaseRecordData(structType *ast.StructType) bool {
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		if field.Names[0].Name != "Data" {
			continue
		}
		fieldType := typeToString(field.Type)
		if fieldType == "*[]QuickbaseRecord" {
			return true
		}
	}
	return false
}

// generateWrappers generates the wrapper code
func generateWrappers(responses []ResponseInfo) (string, error) {
	const tmpl = `// Code generated by generate-results. DO NOT EDIT.

package client

import (
	"github.com/DrewBradfordXYZ/quickbase-go/v2/generated"
)

{{range $resp := .Responses}}
// {{$resp.WrapperName}} wraps {{$resp.Name}} with convenience methods.
type {{$resp.WrapperName}} struct {
	resp *generated.{{$resp.Name}}
}

// Raw returns the underlying generated response.
func (r *{{$resp.WrapperName}}) Raw() *generated.{{$resp.Name}} {
	if r == nil {
		return nil
	}
	return r.resp
}
{{range $field := $resp.Fields}}{{if and $field.IsPtr $field.IsSimple}}
// {{$field.Name}} returns the {{$field.Name}} field value, or zero value if nil.
func (r *{{$resp.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.resp == nil || r.resp.JSON200 == nil || r.resp.JSON200.{{$field.Name}} == nil {
		return {{zeroValue $field.BaseType}}
	}
	return *r.resp.JSON200.{{$field.Name}}
}
{{end}}{{if $field.IsSimpleRequired}}
// {{$field.Name}} returns the {{$field.Name}} field value.
func (r *{{$resp.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.resp == nil || r.resp.JSON200 == nil {
		return {{zeroValue $field.BaseType}}
	}
	return r.resp.JSON200.{{$field.Name}}
}
{{end}}{{if $field.IsPrimitiveSlice}}
// {{$field.Name}} returns the {{$field.Name}} field value, or nil if not set.
func (r *{{$resp.WrapperName}}) {{$field.Name}}() []{{$field.SliceElement}} {
	if r == nil || r.resp == nil || r.resp.JSON200 == nil || r.resp.JSON200.{{$field.Name}} == nil {
		return nil
	}
	return *r.resp.JSON200.{{$field.Name}}
}
{{end}}{{if and $field.IsPtr $field.IsWrappedType (not $field.IsSlice)}}
// {{$field.Name}} returns the {{$field.Name}} field as a wrapped type, or nil if not set.
func (r *{{$resp.WrapperName}}) {{$field.Name}}() *{{$field.WrapperTypeName}} {
	if r == nil || r.resp == nil || r.resp.JSON200 == nil || r.resp.JSON200.{{$field.Name}} == nil {
		return nil
	}
	return &{{$field.WrapperTypeName}}{r.resp.JSON200.{{$field.Name}}}
}
{{end}}{{if and $field.IsSlice $field.IsWrappedType}}
// {{$field.Name}} returns the {{$field.Name}} field as wrapped types, or nil if not set.
func (r *{{$resp.WrapperName}}) {{$field.Name}}() []*{{$field.WrapperTypeName}} {
	if r == nil || r.resp == nil || r.resp.JSON200 == nil {
		return nil
	}
	{{if hasPrefix $field.Type "*[]"}}if r.resp.JSON200.{{$field.Name}} == nil {
		return nil
	}
	items := make([]*{{$field.WrapperTypeName}}, len(*r.resp.JSON200.{{$field.Name}}))
	for i := range *r.resp.JSON200.{{$field.Name}} {
		items[i] = &{{$field.WrapperTypeName}}{&(*r.resp.JSON200.{{$field.Name}})[i]}
	}{{else}}items := make([]*{{$field.WrapperTypeName}}, len(r.resp.JSON200.{{$field.Name}}))
	for i := range r.resp.JSON200.{{$field.Name}} {
		items[i] = &{{$field.WrapperTypeName}}{&r.resp.JSON200.{{$field.Name}}[i]}
	}{{end}}
	return items
}
{{end}}{{if $field.IsEnum}}
// {{$field.Name}} returns the {{$field.Name}} field value as a string, or empty string if nil.
func (r *{{$resp.WrapperName}}) {{$field.Name}}() string {
	if r == nil || r.resp == nil || r.resp.JSON200 == nil || r.resp.JSON200.{{$field.Name}} == nil {
		return ""
	}
	return string(*r.resp.JSON200.{{$field.Name}})
}
{{end}}{{end}}
{{if $resp.HasRecordData}}
// Records returns the record data as unwrapped maps.
// Returns nil if Data is nil.
func (r *{{$resp.WrapperName}}) Records() []Record {
	if r == nil || r.resp == nil || r.resp.JSON200 == nil || r.resp.JSON200.Data == nil {
		return nil
	}
	return unwrapRecords(*r.resp.JSON200.Data)
}
{{end}}
{{end}}

{{range $item := .ItemTypes}}
// {{$item.WrapperName}} wraps {{$item.Name}} with convenience methods.
type {{$item.WrapperName}} struct {
	*generated.{{$item.Name}}
}
{{range $field := $item.Fields}}{{if and $field.IsPtr $field.IsSimple}}
// {{$field.Name}} returns the {{$field.Name}} field value, or zero value if nil.
func (r *{{$item.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.{{$item.Name}} == nil || r.{{$item.Name}}.{{$field.Name}} == nil {
		return {{zeroValue $field.BaseType}}
	}
	return *r.{{$item.Name}}.{{$field.Name}}
}
{{end}}{{if $field.IsSimpleRequired}}
// {{$field.Name}} returns the {{$field.Name}} field value.
func (r *{{$item.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.{{$item.Name}} == nil {
		return {{zeroValue $field.BaseType}}
	}
	return r.{{$item.Name}}.{{$field.Name}}
}
{{end}}{{if $field.IsEnum}}
// {{$field.Name}} returns the {{$field.Name}} field value as a string, or empty string if nil.
func (r *{{$item.WrapperName}}) {{$field.Name}}() string {
	if r == nil || r.{{$item.Name}} == nil || r.{{$item.Name}}.{{$field.Name}} == nil {
		return ""
	}
	return string(*r.{{$item.Name}}.{{$field.Name}})
}
{{end}}{{end}}
{{end}}

{{range $nested := .NestedTypes}}
// {{$nested.WrapperName}} wraps {{$nested.Name}} with convenience methods.
type {{$nested.WrapperName}} struct {
	*generated.{{$nested.Name}}
}
{{range $field := $nested.Fields}}{{if and $field.IsPtr $field.IsSimple}}
// {{$field.Name}} returns the {{$field.Name}} field value, or zero value if nil.
func (r *{{$nested.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.{{$nested.Name}} == nil || r.{{$nested.Name}}.{{$field.Name}} == nil {
		return {{zeroValue $field.BaseType}}
	}
	return *r.{{$nested.Name}}.{{$field.Name}}
}
{{end}}{{if $field.IsSimpleRequired}}
// {{$field.Name}} returns the {{$field.Name}} field value.
func (r *{{$nested.WrapperName}}) {{$field.Name}}() {{$field.BaseType}} {
	if r == nil || r.{{$nested.Name}} == nil {
		return {{zeroValue $field.BaseType}}
	}
	return r.{{$nested.Name}}.{{$field.Name}}
}
{{end}}{{if $field.IsEnum}}
// {{$field.Name}} returns the {{$field.Name}} field value as a string, or empty string if nil.
func (r *{{$nested.WrapperName}}) {{$field.Name}}() string {
	if r == nil || r.{{$nested.Name}} == nil || r.{{$nested.Name}}.{{$field.Name}} == nil {
		return ""
	}
	return string(*r.{{$nested.Name}}.{{$field.Name}})
}
{{end}}{{end}}
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

	data := struct {
		Responses   []ResponseInfo
		ItemTypes   []ItemTypeInfo
		NestedTypes []NestedTypeInfo
	}{
		Responses:   responses,
		ItemTypes:   itemTypes,
		NestedTypes: nestedTypes,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
