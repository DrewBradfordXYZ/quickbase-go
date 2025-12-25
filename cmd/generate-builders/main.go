// Package main generates fluent builder methods from the OpenAPI spec
//
//go:generate go run .
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// OpenAPI spec structures
type OpenAPI struct {
	Paths map[string]PathItem `json:"paths"`
}

type PathItem map[string]Operation

type Operation struct {
	OperationID string               `json:"operationId"`
	Summary     string               `json:"summary"`
	Description string               `json:"description"`
	Parameters  []Parameter          `json:"parameters"`
	RequestBody *RequestBody         `json:"requestBody"`
	Responses   map[string]*Response `json:"responses"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // path, query, header
	Required    bool    `json:"required"`
	Description string  `json:"description"`
	Schema      *Schema `json:"schema"`
}

type RequestBody struct {
	Required bool               `json:"required"`
	Content  map[string]Content `json:"content"`
}

type Content struct {
	Schema *Schema `json:"schema"`
}

type Response struct {
	Description string             `json:"description"`
	Content     map[string]Content `json:"content"`
}

type Schema struct {
	Type                 string             `json:"type"`
	Format               string             `json:"format"`
	Description          string             `json:"description"`
	Properties           map[string]*Schema `json:"properties"`
	Items                *Schema            `json:"items"`
	Required             []string           `json:"required"`
	Enum                 []interface{}      `json:"enum"`
	AdditionalProperties interface{}        `json:"additionalProperties"`
}

// BuilderSpec represents a builder to be generated
type BuilderSpec struct {
	OperationID      string
	BuilderName      string
	MethodName       string
	Summary          string
	ConstructorArgs  []ConstructorArg
	BodyFields       []FieldSpec     // Request body fields
	QueryParams      []QueryParam    // Query parameters
	PathParams       []PathParam     // Path parameters
	HasTable         bool            // Whether this builder uses table alias resolution
	TableParamName   string          // The parameter name for the table (e.g., "from", "to", "tableId")
	TableParamIn     string          // Where the table param is: "body", "query", or "path"
	HasQueryParams   bool            // Whether there are query/header params
	HasBody          bool            // Whether there's a request body
	RequiredFields   []string        // Required body fields
	ResponseFields   []ResponseField // Fields in the response (auto-extracted from spec)
	ResultTypeName   string          // Name of the result type (e.g., "RunQueryResult")
	ResponseIsArray  bool            // Whether the response is an array at root level
	ResponseItemType string          // For array responses, the item type
	HasPagination    bool            // Whether this operation supports pagination
	Transform        TransformSpec   // Response transformation config
}

type ConstructorArg struct {
	Name string
	Type string
}

type PathParam struct {
	Name string
	Type string
}

type QueryParam struct {
	Name        string
	GoName      string  // PascalCase name for Go
	Type        string  // The Go type (may be generated enum type)
	GenType     string  // The generated type name (e.g., "generated.XxxParamsFormat")
	Required    bool
	IsEnum      bool
	Description string
}

type FieldSpec struct {
	MethodName  string
	ParamName   string
	ParamType   string
	Description string
	IsArray     bool
	IsNested    bool   // For nested objects like securityProperties
	ParentField string // Parent field name for nested properties
}

type ResponseField struct {
	Name        string // PascalCase Go field name
	JSONName    string // Original JSON field name
	GoType      string // Full Go type (e.g., "int64", "string", "generated.XxxEnum")
	GenAccess   string // How to access from resp.JSON200 (e.g., ".Id", ".AccessToken")
	IsPointer   bool   // Whether the field is a pointer in the generated type
	IsArray     bool
	IsEnum      bool   // Whether this is an enum type
	NeedsCast   bool   // Whether we need to cast (e.g., enum to string)
	Description string
}

// TransformSpec holds information about response transformation for an operation
type TransformSpec struct {
	Enabled         bool
	ResultType      string
	IsArrayResponse bool
	Fields          []TransformField
}

// TransformField describes a field transformation
type TransformField struct {
	Source      string // Path in the generated response
	Target      string // Field name in the result struct
	Type        string // Go type
	Dereference bool
	TypeCast    string
}

func main() {
	spec, err := readSpec()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading spec: %v\n", err)
		os.Exit(1)
	}

	builders := extractBuilders(spec)

	// Sort by operation ID for consistent output
	sort.Slice(builders, func(i, j int) bool {
		return builders[i].OperationID < builders[j].OperationID
	})

	if err := generateCode(builders); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated client/builders_generated.go with %d builders\n", len(builders))
}

func readSpec() (*OpenAPI, error) {
	paths := []string{
		"spec/output/quickbase-patched.json",
		"../../spec/output/quickbase-patched.json",
	}

	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	var spec OpenAPI
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func extractBuilders(spec *OpenAPI) []BuilderSpec {
	var builders []BuilderSpec

	for path, pathItem := range spec.Paths {
		for _, op := range pathItem {
			if op.OperationID == "" {
				continue
			}

			// Skip operations with manual implementations in api.go
			if hasManualImplementation(op.OperationID) {
				continue
			}

			builder := extractBuilder(path, op)
			builders = append(builders, builder)
		}
	}

	return builders
}

func extractBuilder(path string, op Operation) BuilderSpec {
	opName := toPascalCase(op.OperationID)

	builder := BuilderSpec{
		OperationID:    op.OperationID,
		BuilderName:    opName + "Builder",
		MethodName:     opName,
		Summary:        op.Summary,
		ResultTypeName: opName + "Result",
		HasBody:        op.RequestBody != nil,
		HasPagination:  isPaginatedOperation(op.OperationID),
	}

	// Extract all parameters
	var pathParamNames []string
	for _, param := range op.Parameters {
		switch param.In {
		case "path":
			builder.PathParams = append(builder.PathParams, PathParam{
				Name: param.Name,
				Type: inferGoTypeForParam(param.Schema),
			})
			pathParamNames = append(pathParamNames, param.Name)
		case "query":
			builder.HasQueryParams = true
			qp := QueryParam{
				Name:        param.Name,
				GoName:      toPascalCase(param.Name),
				Required:    param.Required,
				Description: param.Description,
			}
			// Check for enum types
			if param.Schema != nil && len(param.Schema.Enum) > 0 {
				qp.IsEnum = true
				qp.GenType = fmt.Sprintf("generated.%sParams%s", opName, toPascalCase(param.Name))
				qp.Type = qp.GenType
			} else if param.Schema != nil && param.Schema.Format == "date" {
				qp.Type = "types.Date"
				qp.GenType = "types.Date"
			} else {
				qp.Type = inferGoTypeForParam(param.Schema)
				qp.GenType = qp.Type
			}
			builder.QueryParams = append(builder.QueryParams, qp)

			if isTableParam(param.Name) {
				builder.HasTable = true
				builder.TableParamName = param.Name
				builder.TableParamIn = "query"
			}
		case "header":
			builder.HasQueryParams = true
		}
	}

	// Determine constructor args based on config
	constructorParamNames := getConstructorParams(op.OperationID, pathParamNames)

	// Build constructor args
	for _, paramName := range constructorParamNames {
		if isTableParam(paramName) {
			if builder.TableParamIn == "" {
				builder.HasTable = true
				builder.TableParamName = paramName
				builder.TableParamIn = "body"
			}
			builder.ConstructorArgs = append(builder.ConstructorArgs, ConstructorArg{
				Name: "table",
				Type: "string",
			})
		} else {
			paramType := "string"
			for _, pp := range builder.PathParams {
				if pp.Name == paramName {
					paramType = pp.Type
					break
				}
			}
			builder.ConstructorArgs = append(builder.ConstructorArgs, ConstructorArg{
				Name: toCamelCase(paramName),
				Type: paramType,
			})
		}
	}

	// Extract fields from request body
	if op.RequestBody != nil {
		if content, ok := op.RequestBody.Content["application/json"]; ok && content.Schema != nil {
			builder.BodyFields = extractFields(content.Schema, "", builder.OperationID)
			builder.RequiredFields = content.Schema.Required
		}
	}

	// Extract response structure
	if resp, ok := op.Responses["200"]; ok && resp != nil {
		if content, ok := resp.Content["application/json"]; ok && content.Schema != nil {
			// Check if response is array at root level
			if content.Schema.Type == "array" {
				builder.ResponseIsArray = true
				builder.ResponseItemType = fmt.Sprintf("[]generated.%s_200_Item", opName)
			} else {
				builder.ResponseFields = extractResponseFields(content.Schema, opName)
			}
		}
	}

	// Skip result type generation for operations with manual result types
	if shouldSkipResultType(op.OperationID) {
		builder.ResponseFields = nil
		builder.ResponseIsArray = false
	}

	// Check for response transformation config
	if transform, ok := getResponseTransform(op.OperationID); ok {
		builder.Transform = TransformSpec{
			Enabled:         true,
			ResultType:      transform.ResultType,
			IsArrayResponse: transform.IsArrayResponse,
		}
		for _, ft := range transform.ResultFields {
			builder.Transform.Fields = append(builder.Transform.Fields, TransformField{
				Source:      ft.Source,
				Target:      ft.Target,
				Type:        ft.Type,
				Dereference: ft.Dereference,
				TypeCast:    ft.TypeCast,
			})
		}
	}

	return builder
}

func extractFields(schema *Schema, parentPath string, operationID string) []FieldSpec {
	var fields []FieldSpec

	if schema.Properties == nil {
		return fields
	}

	for propName, propSchema := range schema.Properties {
		constructorParams := getConstructorParams(operationID, nil)
		isConstructorParam := false
		for _, cp := range constructorParams {
			if cp == propName {
				isConstructorParam = true
				break
			}
		}
		if isConstructorParam {
			continue
		}

		field := FieldSpec{
			MethodName:  toPascalCase(propName),
			ParamName:   propName,
			Description: propSchema.Description,
		}

		switch propSchema.Type {
		case "array":
			field.IsArray = true
			if propSchema.Items != nil {
				field.ParamType = inferGoType(propSchema.Items)
			} else {
				field.ParamType = "any"
			}
		case "object":
			if propSchema.Properties != nil {
				nestedFields := extractNestedFields(propName, propSchema)
				fields = append(fields, nestedFields...)
				continue
			}
			field.ParamType = "map[string]any"
		default:
			field.ParamType = inferGoType(propSchema)
		}

		fields = append(fields, field)
	}

	return fields
}

func extractNestedFields(parentName string, schema *Schema) []FieldSpec {
	var fields []FieldSpec

	for propName, propSchema := range schema.Properties {
		field := FieldSpec{
			MethodName:  toPascalCase(propName),
			ParamName:   propName,
			ParamType:   inferGoType(propSchema),
			Description: propSchema.Description,
			IsNested:    true,
			ParentField: parentName,
		}
		fields = append(fields, field)
	}

	return fields
}

// snakeToPascal converts snake_case to PascalCase
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func extractResponseFields(schema *Schema, opName string) []ResponseField {
	var fields []ResponseField

	if schema.Properties == nil {
		return fields
	}

	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	for propName, propSchema := range schema.Properties {
		isRequired := requiredSet[propName]

		// Convert JSON name to Go field name (handle snake_case)
		goFieldName := snakeToPascal(propName)

		field := ResponseField{
			Name:      goFieldName,
			JSONName:  propName,
			GenAccess: "." + goFieldName,
			IsPointer: !isRequired,
		}

		// Only extract primitive types - complex/nested types should use manual transforms
		// This is conservative but reliable - oapi-codegen generates unpredictable types
		switch propSchema.Type {
		case "array":
			// Only include arrays of primitives
			if propSchema.Items == nil {
				continue
			}
			switch propSchema.Items.Type {
			case "string":
				field.IsArray = true
				field.GoType = "[]string"
			case "integer":
				field.IsArray = true
				if propSchema.Items.Format == "int64" {
					field.GoType = "[]int64"
				} else {
					field.GoType = "[]int"
				}
			case "number":
				field.IsArray = true
				field.GoType = "[]float64"
			case "boolean":
				field.IsArray = true
				field.GoType = "[]bool"
			default:
				// Skip arrays of objects - too complex, use manual transform
				continue
			}
		case "object":
			// Skip objects - generated types vary too much, use manual transform
			continue
		case "integer":
			if propSchema.Format == "int64" {
				field.GoType = "int64"
			} else {
				field.GoType = "int"
			}
		case "number":
			// Use float32 since that's what oapi-codegen typically generates
			field.GoType = "float32"
		case "boolean":
			field.GoType = "bool"
		case "string":
			// Check for enum - cast to string
			if len(propSchema.Enum) > 0 {
				field.IsEnum = true
				field.GoType = "string"
				field.NeedsCast = true
			} else if propSchema.Format == "date-time" {
				field.GoType = "time.Time"
			} else if propSchema.Format == "date" {
				field.GoType = "types.Date"
			} else {
				field.GoType = "string"
			}
		default:
			continue
		}

		fields = append(fields, field)
	}

	return fields
}

func inferGoType(schema *Schema) string {
	if schema == nil {
		return "any"
	}
	switch schema.Type {
	case "integer":
		if schema.Format == "int64" {
			return "int64"
		}
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "string":
		return "string"
	case "array":
		if schema.Items != nil {
			return "[]" + inferGoType(schema.Items)
		}
		return "[]any"
	case "object":
		return "map[string]any"
	default:
		return "any"
	}
}

func inferGoTypeForParam(schema *Schema) string {
	if schema == nil {
		return "string"
	}
	switch schema.Type {
	case "integer":
		return "int"
	case "number":
		return "float32" // oapi-codegen uses float32 for number params
	case "boolean":
		return "bool"
	default:
		return "string"
	}
}

func toPascalCase(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func toCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// isPaginatedOperation returns true if the operation supports pagination
func isPaginatedOperation(opID string) bool {
	return paginatedOperations[opID]
}

// resultTypeGoName generates the Go type to use in result struct
// For enums, we use string. For other types, we use the actual type.
func resultTypeGoName(rf ResponseField) string {
	if rf.NeedsCast {
		return "string"
	}
	// For complex generated types, just use the direct type
	if strings.HasPrefix(rf.GoType, "generated.") || strings.HasPrefix(rf.GoType, "[]generated.") {
		return rf.GoType
	}
	return rf.GoType
}

// assignmentCode generates the code to assign a response field to result
func assignmentCode(rf ResponseField, opName string) string {
	accessor := fmt.Sprintf("resp.JSON200%s", rf.GenAccess)

	if rf.IsPointer {
		if rf.NeedsCast {
			return fmt.Sprintf("if %s != nil {\n\t\tresult.%s = string(*%s)\n\t}", accessor, rf.Name, accessor)
		}
		return fmt.Sprintf("if %s != nil {\n\t\tresult.%s = *%s\n\t}", accessor, rf.Name, accessor)
	}

	if rf.NeedsCast {
		return fmt.Sprintf("result.%s = string(%s)", rf.Name, accessor)
	}
	return fmt.Sprintf("result.%s = %s", rf.Name, accessor)
}

func generateCode(builders []BuilderSpec) error {
	titleCaser := cases.Title(language.English)
	_ = titleCaser

	funcMap := template.FuncMap{
		"lower":                   strings.ToLower,
		"toCamel":                 toCamelCase,
		"toPascal":                toPascalCase,
		"split":                   strings.Split,
		"isTableParam":            isTableParam,
		"getManualResultType":     func(opID string) string { t, _ := getManualResultType(opID); return t },
		"hasManualResultType":     func(opID string) bool { _, ok := getManualResultType(opID); return ok },
		"isFieldParam":            isFieldParam,
		"isFieldArrayParam":       isFieldArrayParam,
		"needsResultType":         needsResultType,
		"hasTransform":            hasTransform,
		"transformResultType":     transformResultType,
		"transformAssignmentCode": transformAssignmentCode,
		"resultTypeGoName":        resultTypeGoName,
		"assignmentCode":          assignmentCode,
		"hasImportTime":           hasImportTime,
		"hasImportTypes":          hasImportTypes,
		"uniqueTransformTypes":    uniqueTransformTypes,
		"getDataTypeName":         getDataTypeName,
		"getWrapperTypeName":      getWrapperTypeName,
		"shouldReturnRawResponse": shouldReturnRawResponse,
	}

	tmpl := template.Must(template.New("builders").Funcs(funcMap).Parse(buildersTemplate))

	outputPath := findOutputPath()
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, struct {
		Builders []BuilderSpec
	}{
		Builders: builders,
	})
}

// needsResultType returns true if we should generate a friendly result type.
// In v2.0, this always returns false - all builders return raw generated types.
// Users can use opt-in helpers when they want friendlier access patterns.
func needsResultType(b BuilderSpec) bool {
	return false
}

// getDataTypeName returns the generated data type name for a builder's response.
// For object responses: {PascalCaseOperationId}Data (e.g., GetAppData)
// For array responses: {PascalCaseOperationId}Item (e.g., GetFieldsItem)
func getDataTypeName(b BuilderSpec) string {
	pascalOp := toPascalCase(b.OperationID)
	if b.ResponseIsArray {
		return pascalOp + "Item"
	}
	return pascalOp + "Data"
}

// getWrapperTypeName returns the wrapper type name for a builder's response.
// For object responses: {MethodName}Result (e.g., AppResult from GetApp)
// For array responses: {MethodName}Item (e.g., FieldsItem from GetFields)
func getWrapperTypeName(b BuilderSpec) string {
	methodName := toPascalCase(b.OperationID)
	// Strip "Get" prefix for cleaner names
	name := strings.TrimPrefix(methodName, "Get")
	if b.ResponseIsArray {
		return name + "Item"
	}
	return name + "Result"
}

// hasTransform returns true if the builder has a transform config
func hasTransform(b BuilderSpec) bool {
	return b.Transform.Enabled
}

// transformResultType returns the result type name for a transform
func transformResultType(b BuilderSpec) string {
	if b.Transform.IsArrayResponse {
		return "[]" + b.Transform.ResultType
	}
	return b.Transform.ResultType
}

// transformAssignmentCode generates the code to assign a transformed response field
func transformAssignmentCode(tf TransformField, isArray bool) string {
	parts := strings.Split(tf.Source, ".")
	var accessor string
	if isArray {
		accessor = "item"
	} else {
		accessor = "resp.JSON200"
	}
	for _, part := range parts {
		accessor += "." + toPascalCase(part)
	}

	if tf.Dereference {
		if tf.TypeCast != "" {
			return fmt.Sprintf("if %s != nil {\n\t\t\titem.%s = %s(*%s)\n\t\t}", accessor, tf.Target, tf.TypeCast, accessor)
		}
		return fmt.Sprintf("if %s != nil {\n\t\t\titem.%s = *%s\n\t\t}", accessor, tf.Target, accessor)
	}
	if tf.TypeCast != "" {
		return fmt.Sprintf("item.%s = %s(%s)", tf.Target, tf.TypeCast, accessor)
	}
	return fmt.Sprintf("item.%s = %s", tf.Target, accessor)
}

// UniqueTransformType represents a unique result type to generate
type UniqueTransformType struct {
	Name   string
	Fields []TransformField
}

// uniqueTransformTypes returns deduplicated transform result types from all builders
func uniqueTransformTypes(builders []BuilderSpec) []UniqueTransformType {
	seen := make(map[string]bool)
	var result []UniqueTransformType

	for _, b := range builders {
		if b.Transform.Enabled && !b.Transform.IsArrayResponse {
			if !seen[b.Transform.ResultType] {
				seen[b.Transform.ResultType] = true
				result = append(result, UniqueTransformType{
					Name:   b.Transform.ResultType,
					Fields: b.Transform.Fields,
				})
			}
		}
	}

	// Also get array element types (non-array result types from array responses)
	for _, b := range builders {
		if b.Transform.Enabled && b.Transform.IsArrayResponse {
			if !seen[b.Transform.ResultType] {
				seen[b.Transform.ResultType] = true
				result = append(result, UniqueTransformType{
					Name:   b.Transform.ResultType,
					Fields: b.Transform.Fields,
				})
			}
		}
	}

	return result
}

// hasImportTime checks if any builder needs the time package
func hasImportTime(builders []BuilderSpec) bool {
	for _, b := range builders {
		for _, rf := range b.ResponseFields {
			if rf.GoType == "time.Time" {
				return true
			}
		}
	}
	return false
}

// hasImportTypes checks if any builder needs the types package
func hasImportTypes(builders []BuilderSpec) bool {
	for _, b := range builders {
		for _, rf := range b.ResponseFields {
			if rf.GoType == "types.Date" {
				return true
			}
		}
		for _, qp := range b.QueryParams {
			if qp.Type == "types.Date" {
				return true
			}
		}
	}
	return false
}

func findOutputPath() string {
	if _, err := os.Stat("client"); err == nil {
		return "client/builders_generated.go"
	}
	return "../../client/builders_generated.go"
}

// sanitizeGoName ensures the name is a valid Go identifier
func sanitizeGoName(name string) string {
	// Replace invalid characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	return reg.ReplaceAllString(name, "_")
}

const buildersTemplate = `// Code generated by cmd/generate-builders. DO NOT EDIT.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
{{if hasImportTime .Builders}}
	"time"
{{end}}
	"github.com/DrewBradfordXYZ/quickbase-go/v2/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v2/generated"
{{if hasImportTypes .Builders}}
	"github.com/oapi-codegen/runtime/types"
{{end}}
)

// --- Auto-generated builder types ---
// These provide a fluent API for building and executing API requests.
// Method names match QuickBase API operation IDs for consistency with official documentation.

// SortSpec specifies a sort field and order, supporting both field IDs and aliases.
type SortSpec struct {
	Field any                      // Field ID (int) or alias (string)
	Order generated.SortFieldOrder // ASC or DESC
}

// --- Result types for transformed responses ---
{{range $t := uniqueTransformTypes .Builders}}
// {{$t.Name}} contains friendly result data with dereferenced fields.
type {{$t.Name}} struct {
{{- range $t.Fields}}
	{{.Target}} {{.Type}}
{{- end}}
}
{{end}}

{{range $b := .Builders}}
{{- if and (needsResultType $b) (not (hasTransform $b))}}
// {{$b.ResultTypeName}} contains the result of a {{$b.OperationID}} call.
type {{$b.ResultTypeName}} struct {
{{- if $b.ResponseIsArray}}
	Items {{$b.ResponseItemType}}
{{- else}}
{{- range $b.ResponseFields}}
	{{.Name}} {{resultTypeGoName .}}
{{- end}}
{{- end}}
}
{{end}}

// {{$b.BuilderName}} provides a fluent API for the {{$b.OperationID}} operation.
// {{$b.Summary}}
type {{$b.BuilderName}} struct {
	client  *Client
{{- if $b.HasTable}}
	table   string
	tableID string
{{- end}}
{{- range $b.PathParams}}
{{- if not (isTableParam .Name)}}
	{{toCamel .Name}} {{.Type}}
{{- end}}
{{- end}}
{{- range $b.QueryParams}}
{{- if not (isTableParam .Name)}}
	qp{{.GoName}} *{{.GenType}}
{{- end}}
{{- end}}
	params  map[string]any
	err     error
}

// {{$b.MethodName}} starts building a {{$b.OperationID}} request.
{{- if $b.HasTable}}
// The table parameter can be a table ID or an alias (if schema is configured).
{{- end}}
func (c *Client) {{$b.MethodName}}({{range $i, $arg := $b.ConstructorArgs}}{{if $i}}, {{end}}{{$arg.Name}} {{$arg.Type}}{{end}}) *{{$b.BuilderName}} {
	b := &{{$b.BuilderName}}{
		client: c,
		params: make(map[string]any),
	}
{{- range $b.ConstructorArgs}}
{{- if ne .Name "table"}}
	b.{{toCamel .Name}} = {{toCamel .Name}}
{{- end}}
{{- end}}
{{- if $b.HasTable}}
	b.table = table
	if c.schema != nil {
		tableID, err := core.ResolveTableAlias(c.schema, table)
		if err != nil {
			b.err = err
			return b
		}
		b.tableID = tableID
	} else {
		b.tableID = table
	}
{{- end}}
	return b
}

{{range $f := $b.BodyFields}}
// {{$f.MethodName}} {{if $f.Description}}{{$f.Description}}{{else}}sets the {{$f.ParamName}} parameter.{{end}}
{{- if $f.IsArray}}
func (b *{{$b.BuilderName}}) {{$f.MethodName}}(values ...{{$f.ParamType}}) *{{$b.BuilderName}} {
{{- else}}
func (b *{{$b.BuilderName}}) {{$f.MethodName}}(value {{$f.ParamType}}) *{{$b.BuilderName}} {
{{- end}}
	if b.err != nil {
		return b
	}
{{- if $f.IsNested}}
	nested, _ := b.params["{{$f.ParentField}}"].(map[string]any)
	if nested == nil {
		nested = make(map[string]any)
	}
	nested["{{$f.ParamName}}"] = value
	b.params["{{$f.ParentField}}"] = nested
{{- else if $f.IsArray}}
	b.params["{{$f.ParamName}}"] = values
{{- else}}
	b.params["{{$f.ParamName}}"] = value
{{- end}}
	return b
}

{{end}}
{{- /* Generate query param setters */}}
{{range $qp := $b.QueryParams}}
{{- if not (isTableParam $qp.Name)}}
// {{$qp.GoName}} {{if $qp.Description}}{{$qp.Description}}{{else}}sets the {{$qp.Name}} query parameter.{{end}}
func (b *{{$b.BuilderName}}) {{$qp.GoName}}(value {{$qp.GenType}}) *{{$b.BuilderName}} {
	if b.err != nil {
		return b
	}
	b.qp{{$qp.GoName}} = &value
	return b
}

{{end}}
{{- end}}
// Run executes the {{$b.OperationID}} request and returns the response data directly.
{{- if hasTransform $b}}
{{- if $b.Transform.IsArrayResponse}}
func (b *{{$b.BuilderName}}) Run(ctx context.Context) ([]{{$b.Transform.ResultType}}, error) {
{{- else}}
func (b *{{$b.BuilderName}}) Run(ctx context.Context) (*{{$b.Transform.ResultType}}, error) {
{{- end}}
{{- else if needsResultType $b}}
func (b *{{$b.BuilderName}}) Run(ctx context.Context) (*{{$b.ResultTypeName}}, error) {
{{- else if hasManualResultType $b.OperationID}}
func (b *{{$b.BuilderName}}) Run(ctx context.Context) (*{{getManualResultType $b.OperationID}}, error) {
{{- else if shouldReturnRawResponse $b.OperationID}}
func (b *{{$b.BuilderName}}) Run(ctx context.Context) (*generated.{{$b.MethodName}}Response, error) {
{{- else if $b.ResponseIsArray}}
func (b *{{$b.BuilderName}}) Run(ctx context.Context) ([]*{{getWrapperTypeName $b}}, error) {
{{- else}}
func (b *{{$b.BuilderName}}) Run(ctx context.Context) (*{{getWrapperTypeName $b}}, error) {
{{- end}}
	if b.err != nil {
		return nil, b.err
	}

{{- /* Validate required body fields */}}
{{- range $b.RequiredFields}}
{{- if not (isTableParam .)}}
	if _, ok := b.params["{{.}}"]; !ok {
		return nil, &core.ValidationError{
			QuickbaseError: core.QuickbaseError{
				Message: "{{.}} is required for {{$b.OperationID}}",
			},
		}
	}
{{- end}}
{{- end}}

{{- /* Validate required query params */}}
{{- range $qp := $b.QueryParams}}
{{- if $qp.Required}}
{{- if isTableParam $qp.Name}}
	// {{$qp.Name}} is validated via table resolution
{{- else}}
	if b.qp{{$qp.GoName}} == nil {
		return nil, &core.ValidationError{
			QuickbaseError: core.QuickbaseError{
				Message: "{{$qp.Name}} is required for {{$b.OperationID}}",
			},
		}
	}
{{- end}}
{{- end}}
{{- end}}

{{- if $b.HasBody}}
	// Build request body from params
	body := make(map[string]any)
{{- if and $b.HasTable (eq $b.TableParamIn "body")}}
	body["{{$b.TableParamName}}"] = b.tableID
{{- end}}
	for k, v := range b.params {
		body[k] = v
	}

	// Convert to JSON and back to the generated type
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	var reqBody generated.{{$b.MethodName}}JSONRequestBody
	if err := json.Unmarshal(jsonBytes, &reqBody); err != nil {
		return nil, err
	}
{{- end}}

{{- if $b.HasQueryParams}}
	// Build query params
	params := &generated.{{$b.MethodName}}Params{}
{{- if and $b.HasTable (eq $b.TableParamIn "query")}}
	params.{{toPascal $b.TableParamName}} = b.tableID
{{- end}}
{{- range $qp := $b.QueryParams}}
{{- if not (isTableParam $qp.Name)}}
	if b.qp{{$qp.GoName}} != nil {
		params.{{$qp.GoName}} = {{if not $qp.Required}}b.qp{{$qp.GoName}}{{else}}*b.qp{{$qp.GoName}}{{end}}
	}
{{- end}}
{{- end}}
{{- end}}

	resp, err := b.client.API().{{$b.MethodName}}WithResponse(ctx{{range $b.PathParams}}, {{if isTableParam .Name}}b.tableID{{else}}b.{{toCamel .Name}}{{end}}{{end}}{{if $b.HasQueryParams}}, params{{end}}{{if $b.HasBody}}, reqBody{{end}})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, parseAPIError(resp.StatusCode(), resp.Body, resp.HTTPResponse)
	}

{{- if hasTransform $b}}
{{- if $b.Transform.IsArrayResponse}}
	// Build friendly result from array response
	var results []{{$b.Transform.ResultType}}
	for _, src := range *resp.JSON200 {
		item := {{$b.Transform.ResultType}}{}
{{- range $tf := $b.Transform.Fields}}
{{- if $tf.Dereference}}
		if src.{{toPascal $tf.Source}} != nil {
{{- if $tf.TypeCast}}
			item.{{$tf.Target}} = {{$tf.TypeCast}}(*src.{{toPascal $tf.Source}})
{{- else}}
			item.{{$tf.Target}} = *src.{{toPascal $tf.Source}}
{{- end}}
		}
{{- else if $tf.TypeCast}}
		item.{{$tf.Target}} = {{$tf.TypeCast}}(src.{{toPascal $tf.Source}})
{{- else}}
		item.{{$tf.Target}} = src.{{toPascal $tf.Source}}
{{- end}}
{{- end}}
		results = append(results, item)
	}
	return results, nil
{{- else}}
	// Build friendly result
	result := &{{$b.Transform.ResultType}}{}
{{- range $tf := $b.Transform.Fields}}
{{- $parts := split $tf.Source "."}}
{{- if eq (len $parts) 1}}
{{- /* Simple field access */}}
{{- if $tf.Dereference}}
	if resp.JSON200.{{toPascal $tf.Source}} != nil {
{{- if $tf.TypeCast}}
		result.{{$tf.Target}} = {{$tf.TypeCast}}(*resp.JSON200.{{toPascal $tf.Source}})
{{- else}}
		result.{{$tf.Target}} = *resp.JSON200.{{toPascal $tf.Source}}
{{- end}}
	}
{{- else if $tf.TypeCast}}
	result.{{$tf.Target}} = {{$tf.TypeCast}}(resp.JSON200.{{toPascal $tf.Source}})
{{- else}}
	result.{{$tf.Target}} = resp.JSON200.{{toPascal $tf.Source}}
{{- end}}
{{- else}}
{{- /* Nested field access - needs more complex handling */}}
{{- if $tf.Dereference}}
	if resp.JSON200.{{toPascal (index $parts 0)}}.{{toPascal (index $parts 1)}} != nil {
{{- if $tf.TypeCast}}
		result.{{$tf.Target}} = {{$tf.TypeCast}}(*resp.JSON200.{{toPascal (index $parts 0)}}.{{toPascal (index $parts 1)}})
{{- else}}
		result.{{$tf.Target}} = *resp.JSON200.{{toPascal (index $parts 0)}}.{{toPascal (index $parts 1)}}
{{- end}}
	}
{{- else if $tf.TypeCast}}
	result.{{$tf.Target}} = {{$tf.TypeCast}}(resp.JSON200.{{toPascal (index $parts 0)}}.{{toPascal (index $parts 1)}})
{{- else}}
	result.{{$tf.Target}} = resp.JSON200.{{toPascal (index $parts 0)}}.{{toPascal (index $parts 1)}}
{{- end}}
{{- end}}
{{- end}}
	return result, nil
{{- end}}
{{- else if needsResultType $b}}
	// Build friendly result
	result := &{{$b.ResultTypeName}}{}
{{- if $b.ResponseIsArray}}
	result.Items = *resp.JSON200
{{- else}}
{{- range $rf := $b.ResponseFields}}
	{{assignmentCode $rf $b.MethodName}}
{{- end}}
{{- end}}

	return result, nil
{{- else if hasManualResultType $b.OperationID}}
	return &{{getManualResultType $b.OperationID}}{raw: resp}, nil
{{- else if shouldReturnRawResponse $b.OperationID}}
	return resp, nil
{{- else if $b.ResponseIsArray}}
	// Wrap each item in the result wrapper
	items := make([]*{{getWrapperTypeName $b}}, len(*resp.JSON200))
	for i := range *resp.JSON200 {
		items[i] = &{{getWrapperTypeName $b}}{&(*resp.JSON200)[i]}
	}
	return items, nil
{{- else}}
	return &{{getWrapperTypeName $b}}{resp: resp}, nil
{{- end}}
}

{{end}}
// Ensure imports are used
var (
	_ = fmt.Sprintf
	_ = json.Marshal
	_ = strconv.Itoa
	_ = core.ValidationError{}
)
`
