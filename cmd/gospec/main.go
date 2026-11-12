// Command gospec adapts the shared OpenAPI spec for Go code generation.
//
// The spec (spec/output/quickbase-patched.json) is shared with other
// languages and stays language-neutral. Go wants three things it doesn't
// say, added here with oapi-codegen extensions:
//
//  1. Named result types. An operation's inline 200 or 207 response object
//     becomes a component, <OperationId>Result (GetAppResult), so methods
//     return a type callers can name.
//  2. Plain fields in results. An optional property of a schema used only
//     in responses is a value, not a pointer (x-go-type-skip-optional-pointer):
//     a missing field reads as its zero value, so results need no nil checks.
//     Schemas used in requests keep pointers, where "not set" must differ
//     from zero (a false that turns a setting off).
//  3. Records as core.Record. A record (QuickbaseRecord) is core.Record and
//     a field value (FieldValue) core.FieldValue (x-go-type): its value is a
//     plain Go value instead of a union type, and the record has methods
//     (Values, Decode) that a generated type can't.
//
// Usage:
//
//	go run ./cmd/gospec spec/output/quickbase-patched.json .build/openapi-go.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type obj = map[string]any

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gospec <shared spec> <output>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail(err)
	}
	var spec obj
	if err := json.Unmarshal(raw, &spec); err != nil {
		fail(err)
	}

	named := nameResults(spec)
	marked := plainResultFields(spec)
	plainRecordValues(spec)

	out, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(os.Args[2]), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(os.Args[2], out, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("gospec: %d result types named, %d optional result fields made plain\n", named, marked)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gospec:", err)
	os.Exit(1)
}

func schemas(spec obj) obj {
	return spec["components"].(obj)["schemas"].(obj)
}

// operations calls fn for each operation, in a stable order.
func operations(spec obj, fn func(path, method string, op obj)) {
	paths := spec["paths"].(obj)
	keys := make([]string, 0, len(paths))
	for p := range paths {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	for _, p := range keys {
		item, _ := paths[p].(obj)
		for _, m := range []string{"get", "put", "post", "delete", "patch"} {
			if op, ok := item[m].(obj); ok {
				fn(p, m, op)
			}
		}
	}
}

func jsonSchema(response any) (obj, obj) {
	r, _ := response.(obj)
	content, _ := r["content"].(obj)
	media, _ := content["application/json"].(obj)
	s, _ := media["schema"].(obj)
	return media, s
}

// nameResults moves each inline 200/207 response object to a component. A
// 207 whose schema matches its 200's shares the type.
func nameResults(spec obj) int {
	comps := schemas(spec)
	count := 0
	operations(spec, func(_, _ string, op obj) {
		id, _ := op["operationId"].(string)
		responses, _ := op["responses"].(obj)
		var name200 string
		var schema200 []byte
		for _, code := range []string{"200", "207"} {
			media, s := jsonSchema(responses[code])
			if s == nil || s["$ref"] != nil || s["type"] != "object" {
				continue
			}
			encoded, _ := json.Marshal(s)
			name := pascal(id) + "Result"
			if code == "207" {
				if name200 != "" && string(encoded) == string(schema200) {
					media["schema"] = obj{"$ref": "#/components/schemas/" + name200}
					continue
				}
				name = pascal(id) + "MultiStatusResult"
			}
			if _, taken := comps[name]; taken {
				fail(fmt.Errorf("%s: component %s already exists", id, name))
			}
			comps[name] = s
			media["schema"] = obj{"$ref": "#/components/schemas/" + name}
			count++
			if code == "200" {
				name200, schema200 = name, encoded
			}
		}
	})
	return count
}

// plainResultFields marks the optional properties of schemas only responses
// use (directly or nested) to be generated without pointers.
func plainResultFields(spec obj) int {
	comps := schemas(spec)
	requestSide := map[string]bool{}
	responseSide := map[string]bool{}
	operations(spec, func(_, _ string, op obj) {
		collectRefs(op["requestBody"], comps, requestSide)
		collectRefs(op["parameters"], comps, requestSide)
		collectRefs(op["responses"], comps, responseSide)
	})
	marked := 0
	for name := range responseSide {
		if !requestSide[name] {
			marked += markOptional(comps[name])
		}
	}
	operations(spec, func(_, _ string, op obj) {
		marked += markOptional(op["responses"])
	})
	return marked
}

func collectRefs(node any, comps obj, seen map[string]bool) {
	switch v := node.(type) {
	case obj:
		if ref, ok := v["$ref"].(string); ok && strings.HasPrefix(ref, "#/components/schemas/") {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if !seen[name] {
				seen[name] = true
				collectRefs(comps[name], comps, seen)
			}
		}
		for _, child := range v {
			collectRefs(child, comps, seen)
		}
	case []any:
		for _, child := range v {
			collectRefs(child, comps, seen)
		}
	}
}

// markOptional marks optional properties within a schema, not following
// $refs (each component is handled on its own).
func markOptional(node any) int {
	marked := 0
	switch v := node.(type) {
	case obj:
		if v["$ref"] != nil {
			return 0
		}
		if props, ok := v["properties"].(obj); ok {
			required := map[string]bool{}
			if list, ok := v["required"].([]any); ok {
				for _, r := range list {
					if s, ok := r.(string); ok {
						required[s] = true
					}
				}
			}
			for name, p := range props {
				if prop, ok := p.(obj); ok && !required[name] {
					prop["x-go-type-skip-optional-pointer"] = true
					marked++
				}
			}
		}
		for _, child := range v {
			marked += markOptional(child)
		}
	case []any:
		for _, child := range v {
			marked += markOptional(child)
		}
	}
	return marked
}

// plainRecordValues maps records and field values to core's types.
func plainRecordValues(spec obj) {
	core := obj{"path": "github.com/DrewBradfordXYZ/quickbase-go/v3/core"}
	for name, goType := range map[string]string{"QuickbaseRecord": "core.Record", "FieldValue": "core.FieldValue"} {
		schema, ok := schemas(spec)[name].(obj)
		if !ok {
			fail(fmt.Errorf("%s not found", name))
		}
		schema["x-go-type"] = goType
		schema["x-go-type-import"] = core
	}
}

// pascal turns an operationId (getApp) into a type name (GetApp).
func pascal(id string) string {
	r := []rune(id)
	if len(r) == 0 {
		return id
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
