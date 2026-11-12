// Command generate-methods writes one method on client.Client per API
// operation (client/methods_generated.go), from the generated client's
// signatures and the spec's summaries. See docs/architecture.md.
//
//   - Path parameters are arguments, query parameters one params struct, a
//     JSON body the generated body type, a body that isn't JSON []byte.
//   - It returns the success result (<Operation>Result, a list of items), or
//     []byte for a response that isn't JSON, or QuickBase's error.
//   - A 207 returns the result; its body has the success shape.
//
// Usage (from the repository root, after oapi-codegen):
//
//	go run ./cmd/generate-methods .build/openapi-go.json
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"unicode"
)

const (
	generatedFile = "generated/quickbase.gen.go"
	outputFile    = "client/methods_generated.go"
	aliasesFile   = "types_generated.go" // package quickbase
)

type param struct {
	name, typ string
}

type method struct {
	name        string // GetApp
	operationID string // getApp
	summary     string
	pathParams  []param
	paramsType  string // GetAppParams, or ""
	jsonBody    string // AddTrusteesJSONRequestBody, or ""
	rawBody     string // content type of a body that isn't JSON, or ""
	result      string // *generated.GetAppResult, []generated.GetFieldsItem, or "" for []byte
	sliceResult bool   // the result is the dereferenced JSON200 (a list, or any)
	multiStatus bool   // the response has a 207 with a body
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: generate-methods <Go-adapted spec>")
		os.Exit(2)
	}
	ops := readSpec(os.Args[1])

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, generatedFile, nil, 0)
	if err != nil {
		fail(err)
	}
	signatures, responses := collect(file)

	var methods []method
	for name, op := range ops {
		m := method{name: name, operationID: op.id, summary: op.summary, rawBody: op.rawBody}

		sig := signatures[name+"WithResponse"]
		if op.rawBody != "" || sig == nil {
			sig = signatures[name+"WithBodyWithResponse"]
		}
		if sig == nil {
			fail(fmt.Errorf("%s: no generated method", name))
		}
		for _, f := range sig.Params.List[1:] { // after ctx
			typ := exprString(f.Type)
			for _, n := range f.Names {
				switch {
				case strings.HasPrefix(typ, "..."): // reqEditors
				case n.Name == "params":
					m.paramsType = strings.TrimPrefix(typ, "*")
				case n.Name == "contentType":
				case n.Name == "body" && typ == "io.Reader":
				case n.Name == "body":
					m.jsonBody = typ
				default:
					m.pathParams = append(m.pathParams, param{n.Name, typ})
				}
			}
		}

		fields := responses[name+"Response"]
		if t, ok := fields["JSON200"]; ok {
			if t == "*interface{}" {
				// An untyped response: the decoded JSON
				m.result = "any"
				m.sliceResult = true
			} else if strings.HasPrefix(t, "*[]") {
				m.result = "[]generated." + strings.TrimPrefix(t, "*[]")
				m.sliceResult = true
			} else {
				m.result = "*generated." + strings.TrimPrefix(t, "*")
			}
		}
		_, m.multiStatus = fields["JSON207"]
		methods = append(methods, m)
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i].name < methods[j].name })

	src := render(methods)
	formatted, err := format.Source(src)
	if err != nil {
		os.WriteFile(outputFile+".broken", src, 0o644)
		fail(fmt.Errorf("formatting: %w (unformatted output in %s.broken)", err, outputFile))
	}
	if err := os.WriteFile(outputFile, formatted, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("generate-methods: %d methods in %s\n", len(methods), outputFile)

	n := writeAliases(file)
	fmt.Printf("generate-methods: %d type aliases in %s\n", n, aliasesFile)
}

// writeAliases re-exports every exported generated type from the root
// package under its own name (quickbase.GetAppResult), so callers need one
// import. A name the root package already declares is skipped.
func writeAliases(generatedAST *ast.File) int {
	declared := map[string]bool{}
	entries, err := os.ReadDir(".")
	if err != nil {
		fail(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == aliasesFile {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			fail(err)
		}
		for ident := range f.Scope.Objects {
			declared[ident] = true
		}
		for _, decl := range f.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gd.Specs {
					switch sp := spec.(type) {
					case *ast.TypeSpec:
						declared[sp.Name.Name] = true
					case *ast.ValueSpec:
						for _, n := range sp.Names {
							declared[n.Name] = true
						}
					}
				}
			}
		}
	}

	var names []string
	for _, decl := range generatedAST.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts := spec.(*ast.TypeSpec)
			if ts.Name.IsExported() && !declared[ts.Name.Name] && ts.TypeParams == nil {
				names = append(names, ts.Name.Name)
			}
		}
	}
	sort.Strings(names)

	var b bytes.Buffer
	b.WriteString("// Code generated by cmd/generate-methods. DO NOT EDIT.\n\npackage quickbase\n\n")
	b.WriteString("import \"github.com/DrewBradfordXYZ/quickbase-go/v3/generated\"\n\n")
	b.WriteString("// The generated package's types, under their own names.\ntype (\n")
	for _, n := range names {
		fmt.Fprintf(&b, "\t%s = generated.%s\n", n, n)
	}
	b.WriteString(")\n")
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(aliasesFile, formatted, 0o644); err != nil {
		fail(err)
	}
	return len(names)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "generate-methods:", err)
	os.Exit(1)
}

type operation struct {
	id, summary, rawBody string
}

// readSpec returns each operation by its Go method name.
func readSpec(path string) map[string]operation {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Summary     string `json:"summary"`
			RequestBody *struct {
				Content map[string]any `json:"content"`
			} `json:"requestBody"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		fail(err)
	}
	ops := map[string]operation{}
	for _, item := range spec.Paths {
		for _, op := range item {
			if op.OperationID == "" {
				continue
			}
			o := operation{id: op.OperationID, summary: op.Summary}
			if op.RequestBody != nil {
				if _, isJSON := op.RequestBody.Content["application/json"]; !isJSON {
					for contentType := range op.RequestBody.Content {
						o.rawBody = contentType
					}
				}
			}
			ops[pascal(op.OperationID)] = o
		}
	}
	return ops
}

// collect returns ClientWithResponses' methods and the *Response structs'
// field types.
func collect(file *ast.File) (map[string]*ast.FuncType, map[string]map[string]string) {
	signatures := map[string]*ast.FuncType{}
	responses := map[string]map[string]string{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) != 1 || exprString(d.Recv.List[0].Type) != "*ClientWithResponses" {
				continue
			}
			signatures[d.Name.Name] = d.Type
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(ts.Name.Name, "Response") {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := map[string]string{}
				for _, f := range st.Fields.List {
					for _, n := range f.Names {
						fields[n.Name] = exprString(f.Type)
					}
				}
				responses[ts.Name.Name] = fields
			}
		}
	}
	return signatures, responses
}

func exprString(e ast.Expr) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), e); err != nil {
		fail(err)
	}
	return b.String()
}

// qualify prefixes generated type names: int64 stays, GetAppParams becomes
// generated.GetAppParams.
func qualify(typ string) string {
	switch typ {
	case "string", "int", "int32", "int64", "float32", "float64", "bool":
		return typ
	}
	return "generated." + typ
}

func render(methods []method) []byte {
	var b bytes.Buffer
	b.WriteString(`// Code generated by cmd/generate-methods. DO NOT EDIT.

package client

import (
	"bytes"
	"context"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/generated"
)

var _ = bytes.NewReader

`)
	for _, m := range methods {
		renderMethod(&b, m)
	}
	return b.Bytes()
}

func renderMethod(b *bytes.Buffer, m method) {
	summary := strings.TrimSpace(m.summary)
	if summary == "" {
		summary = m.operationID
	}
	fmt.Fprintf(b, "// %s: %s.\n//\n// See https://developer.quickbase.com/operation/%s\n", m.name, strings.TrimSuffix(summary, "."), m.operationID)

	// Signature
	args := []string{"ctx context.Context"}
	call := []string{"ctx"}
	for _, p := range m.pathParams {
		args = append(args, p.name+" "+qualify(p.typ))
		call = append(call, p.name)
	}
	if m.paramsType != "" {
		args = append(args, "params "+qualify(m.paramsType))
		call = append(call, "&params")
	}
	switch {
	case m.rawBody != "":
		args = append(args, "body []byte")
		call = append(call, fmt.Sprintf("%q", m.rawBody), "bytes.NewReader(body)")
	case m.jsonBody != "":
		args = append(args, "body "+qualify(m.jsonBody))
		call = append(call, "body")
	}
	generatedName := m.name + "WithResponse"
	if m.rawBody != "" {
		generatedName = m.name + "WithBodyWithResponse"
	}

	result := m.result
	if result == "" {
		result = "[]byte"
	}
	fmt.Fprintf(b, "func (c *Client) %s(%s) (%s, error) {\n", m.name, strings.Join(args, ", "), result)
	if m.operationID == "upsert" {
		// A time.Time could be a Date or a Date/Time value, which QuickBase
		// accepts in different forms; refuse it before anything is sent
		b.WriteString("\tif err := core.CheckRecordData(body.Data); err != nil {\n\t\treturn nil, err\n\t}\n")
	}
	fmt.Fprintf(b, "\tresp, err := c.generated.%s(%s)\n", generatedName, strings.Join(call, ", "))
	b.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n")

	switch {
	case m.result == "":
		b.WriteString("\treturn rawResult(resp.StatusCode(), resp.Body, resp.HTTPResponse)\n")
	case m.sliceResult:
		b.WriteString("\tlist, err := jsonResult(resp.StatusCode(), resp.Body, resp.HTTPResponse, resp.JSON200, false)\n")
		b.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn *list, nil\n")
	case m.operationID == "upsert":
		b.WriteString("\tresult, err := jsonResult(resp.StatusCode(), resp.Body, resp.HTTPResponse, resp.JSON200, true)\n")
		b.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn result, c.lineErrors(result)\n")
	default:
		fmt.Fprintf(b, "\treturn jsonResult(resp.StatusCode(), resp.Body, resp.HTTPResponse, resp.JSON200, %t)\n", m.multiStatus)
	}
	b.WriteString("}\n\n")
}

// pascal turns an operationId (getApp) into a method name (GetApp).
func pascal(id string) string {
	r := []rune(id)
	if len(r) == 0 {
		return id
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
