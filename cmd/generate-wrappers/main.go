// Package main generates wrapper methods from the OpenAPI spec
//
//go:generate go run main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// OpenAPI spec structures
type OpenAPI struct {
	Paths map[string]PathItem `json:"paths"`
}

type PathItem map[string]Operation // key is HTTP method

type Operation struct {
	OperationID string               `json:"operationId"`
	Summary     string               `json:"summary"`
	Parameters  []Parameter          `json:"parameters"`
	RequestBody *RequestBody         `json:"requestBody"`
	Responses   map[string]*Response `json:"responses"`
}

type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"` // path, query, header
	Required bool   `json:"required"`
	Schema   *Schema
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
	Type       string             `json:"type"`
	Properties map[string]*Schema `json:"properties"`
	Items      *Schema            `json:"items"`
	Required   []string           `json:"required"`
}

// Wrapper method definition for code generation
type WrapperMethod struct {
	Name          string   // Method name (e.g., "GetApp")
	Summary       string   // Brief description
	PathParams    []Param  // Path parameters
	QueryParams   []Param  // Query parameters
	HasBody       bool     // Whether it has a request body
	BodyType      string   // Request body type name (e.g., "generated.CreateAppJSONRequestBody")
	ResponseType  string   // Response type name
	APIMethod     string   // The generated API method to call
	HasParams     bool     // Whether it has query/header params struct
	ParamsType    string   // The params struct type
	ResultFields  []Field  // Fields to extract from response
	HasPagination bool     // Whether this is a paginated endpoint
}

type Param struct {
	Name     string
	Type     string
	Required bool
	In       string // path, query
}

type Field struct {
	Name     string
	JSONName string
	Type     string
}

func main() {
	// Read the OpenAPI spec
	specPath := "spec/output/quickbase-patched.json"
	data, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading spec: %v\n", err)
		os.Exit(1)
	}

	var spec OpenAPI
	if err := json.Unmarshal(data, &spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing spec: %v\n", err)
		os.Exit(1)
	}

	// Collect wrapper methods
	var methods []WrapperMethod
	for path, pathItem := range spec.Paths {
		for httpMethod, op := range pathItem {
			if op.OperationID == "" {
				continue
			}

			method := buildWrapperMethod(path, httpMethod, op)
			if method != nil {
				methods = append(methods, *method)
			}
		}
	}

	// Sort methods by name for consistent output
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})

	// Generate the output
	if err := generateCode(methods); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated client/api_generated.go")
}

func buildWrapperMethod(path, httpMethod string, op Operation) *WrapperMethod {
	name := toPascalCase(op.OperationID)

	method := &WrapperMethod{
		Name:         name,
		Summary:      op.Summary,
		APIMethod:    name + "WithResponse",
		ResponseType: name + "Response",
	}

	// Parse path parameters
	for _, param := range op.Parameters {
		p := Param{
			Name:     param.Name,
			Type:     inferGoType(param.Schema),
			Required: param.Required,
			In:       param.In,
		}

		if param.In == "path" {
			method.PathParams = append(method.PathParams, p)
		} else if param.In == "query" || param.In == "header" {
			method.QueryParams = append(method.QueryParams, p)
		}
	}

	// Check for params struct (query/header params)
	if len(method.QueryParams) > 0 {
		method.HasParams = true
		method.ParamsType = "generated." + name + "Params"
	}

	// Check for request body
	if op.RequestBody != nil {
		method.HasBody = true
		method.BodyType = "generated." + name + "JSONRequestBody"
	}

	return method
}

func inferGoType(schema *Schema) string {
	if schema == nil {
		return "string"
	}
	switch schema.Type {
	case "integer":
		return "int"
	case "number":
		return "float32"
	case "boolean":
		return "bool"
	case "array":
		if schema.Items != nil {
			return "[]" + inferGoType(schema.Items)
		}
		return "[]interface{}"
	default:
		return "string"
	}
}

func toPascalCase(s string) string {
	// Handle operationId like "getApp" -> "GetApp"
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func generateCode(methods []WrapperMethod) error {
	// Skip methods we've already manually implemented
	skip := map[string]bool{
		"RunQuery":      true,
		"Upsert":        true,
		"DeleteRecords": true,
		"GetApp":        true,
		"GetFields":     true,
	}

	tmpl := template.Must(template.New("api").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(apiTemplate))

	// Filter methods
	var filteredMethods []WrapperMethod
	for _, m := range methods {
		if skip[m.Name] {
			continue
		}
		filteredMethods = append(filteredMethods, m)
	}

	f, err := os.Create("client/api_generated.go")
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, struct {
		Methods []WrapperMethod
	}{
		Methods: filteredMethods,
	})
}

const apiTemplate = `// Code generated by cmd/generate-wrappers. DO NOT EDIT.

package client

import (
	"context"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

// --- Auto-generated wrapper methods ---
// These provide a cleaner API over the generated oapi-codegen client.
// For full control, use client.API() to access the underlying client.

{{range .Methods}}
// {{.Name}} {{.Summary}}
func (c *Client) {{.Name}}(ctx context.Context{{range .PathParams}}, {{.Name}} {{.Type}}{{end}}{{if .HasParams}}, params *{{.ParamsType}}{{end}}{{if .HasBody}}, body {{.BodyType}}{{end}}) (*generated.{{.ResponseType}}, error) {
	resp, err := c.API().{{.APIMethod}}(ctx{{range .PathParams}}, {{.Name}}{{end}}{{if .HasParams}}, params{{end}}{{if .HasBody}}, body{{end}})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, &core.QuickbaseError{Message: "unexpected response", StatusCode: resp.StatusCode()}
	}
	return resp, nil
}
{{end}}
`
