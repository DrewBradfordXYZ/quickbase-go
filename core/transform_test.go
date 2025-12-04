package core

import (
	"reflect"
	"testing"
)

func TestTransformRequest(t *testing.T) {
	schema := ResolveSchema(&Schema{
		Tables: map[string]TableSchema{
			"projects": {
				ID: "bqw3ryzab",
				Fields: map[string]int{
					"id":     3,
					"name":   6,
					"status": 7,
				},
			},
		},
	})

	t.Run("passes through unchanged without schema", func(t *testing.T) {
		body := map[string]any{
			"from":  "bqw3ryzab",
			"where": "{3.EX.'123'}",
		}

		result, tableID, err := TransformRequest(body, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tableID != "bqw3ryzab" {
			t.Errorf("tableID = %q, want %q", tableID, "bqw3ryzab")
		}
		if result["from"] != "bqw3ryzab" {
			t.Errorf("from = %q, want %q", result["from"], "bqw3ryzab")
		}
	})

	t.Run("resolves table alias in 'from'", func(t *testing.T) {
		body := map[string]any{
			"from": "projects",
		}

		result, tableID, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["from"] != "bqw3ryzab" {
			t.Errorf("from = %q, want %q", result["from"], "bqw3ryzab")
		}
		if tableID != "bqw3ryzab" {
			t.Errorf("tableID = %q, want %q", tableID, "bqw3ryzab")
		}
	})

	t.Run("resolves table alias in 'to'", func(t *testing.T) {
		body := map[string]any{
			"to": "projects",
		}

		result, tableID, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["to"] != "bqw3ryzab" {
			t.Errorf("to = %q, want %q", result["to"], "bqw3ryzab")
		}
		if tableID != "bqw3ryzab" {
			t.Errorf("tableID = %q, want %q", tableID, "bqw3ryzab")
		}
	})

	t.Run("resolves select array", func(t *testing.T) {
		body := map[string]any{
			"from":   "projects",
			"select": []any{"name", "status", 3},
		}

		result, _, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		selectArr := result["select"].([]int)
		expected := []int{6, 7, 3}
		if !reflect.DeepEqual(selectArr, expected) {
			t.Errorf("select = %v, want %v", selectArr, expected)
		}
	})

	t.Run("resolves sortBy array", func(t *testing.T) {
		body := map[string]any{
			"from": "projects",
			"sortBy": []any{
				map[string]any{"fieldId": "name", "order": "ASC"},
				map[string]any{"fieldId": 3, "order": "DESC"},
			},
		}

		result, _, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sortBy := result["sortBy"].([]map[string]any)
		if sortBy[0]["fieldId"] != 6 {
			t.Errorf("sortBy[0].fieldId = %v, want 6", sortBy[0]["fieldId"])
		}
		if sortBy[1]["fieldId"] != 3 {
			t.Errorf("sortBy[1].fieldId = %v, want 3", sortBy[1]["fieldId"])
		}
	})

	t.Run("resolves groupBy array", func(t *testing.T) {
		body := map[string]any{
			"from": "projects",
			"groupBy": []any{
				map[string]any{"fieldId": "status", "grouping": "equal-values"},
			},
		}

		result, _, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		groupBy := result["groupBy"].([]map[string]any)
		if groupBy[0]["fieldId"] != 7 {
			t.Errorf("groupBy[0].fieldId = %v, want 7", groupBy[0]["fieldId"])
		}
	})

	t.Run("transforms where clause", func(t *testing.T) {
		body := map[string]any{
			"from":  "projects",
			"where": "{'status'.EX.'Active'}",
		}

		result, _, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		where := result["where"].(string)
		if where != "{7.EX.'Active'}" {
			t.Errorf("where = %q, want %q", where, "{7.EX.'Active'}")
		}
	})

	t.Run("transforms where clause with quoted alias", func(t *testing.T) {
		body := map[string]any{
			"from":  "projects",
			"where": "{\"name\".CT.'test'}",
		}

		result, _, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		where := result["where"].(string)
		if where != "{6.CT.'test'}" {
			t.Errorf("where = %q, want %q", where, "{6.CT.'test'}")
		}
	})

	t.Run("transforms data array for upsert", func(t *testing.T) {
		body := map[string]any{
			"to": "projects",
			"data": []any{
				map[string]any{
					"name":   map[string]any{"value": "Project A"},
					"status": map[string]any{"value": "Active"},
				},
			},
		}

		result, _, err := TransformRequest(body, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data := result["data"].([]map[string]any)
		if _, ok := data[0]["6"]; !ok {
			t.Error("expected field '6' (name) in transformed data")
		}
		if _, ok := data[0]["7"]; !ok {
			t.Error("expected field '7' (status) in transformed data")
		}
	})

	t.Run("returns error for unknown table alias", func(t *testing.T) {
		body := map[string]any{
			"from": "unknown",
		}

		_, _, err := TransformRequest(body, schema)
		if err == nil {
			t.Fatal("expected error for unknown table alias")
		}
	})

	t.Run("returns error for unknown field alias", func(t *testing.T) {
		body := map[string]any{
			"from":   "projects",
			"select": []any{"unknown_field"},
		}

		_, _, err := TransformRequest(body, schema)
		if err == nil {
			t.Fatal("expected error for unknown field alias")
		}
	})
}

func TestTransformWhereClause(t *testing.T) {
	schema := ResolveSchema(&Schema{
		Tables: map[string]TableSchema{
			"projects": {
				ID: "bqw3ryzab",
				Fields: map[string]int{
					"name":   6,
					"status": 7,
				},
			},
		},
	})

	tests := []struct {
		name     string
		where    string
		expected string
	}{
		{
			name:     "simple alias",
			where:    "{'status'.EX.'Active'}",
			expected: "{7.EX.'Active'}",
		},
		{
			name:     "double-quoted alias",
			where:    "{\"name\".CT.'test'}",
			expected: "{6.CT.'test'}",
		},
		{
			name:     "numeric field ID passthrough",
			where:    "{3.EX.'123'}",
			expected: "{3.EX.'123'}",
		},
		{
			name:     "complex AND query",
			where:    "{'status'.EX.'Active'}AND{'name'.CT.'Project'}",
			expected: "{7.EX.'Active'}AND{6.CT.'Project'}",
		},
		{
			name:     "unknown alias left unchanged",
			where:    "{'unknown'.EX.'test'}",
			expected: "{'unknown'.EX.'test'}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformWhereClause(tt.where, schema, "bqw3ryzab")
			if result != tt.expected {
				t.Errorf("transformWhereClause = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTransformResponse(t *testing.T) {
	schema := ResolveSchema(&Schema{
		Tables: map[string]TableSchema{
			"projects": {
				ID: "bqw3ryzab",
				Fields: map[string]int{
					"id":     3,
					"name":   6,
					"status": 7,
				},
			},
		},
	})

	t.Run("returns nil for nil response", func(t *testing.T) {
		result := TransformResponse(nil, schema, "bqw3ryzab")
		if result != nil {
			t.Errorf("TransformResponse(nil) = %v, want nil", result)
		}
	})

	t.Run("transforms field IDs to aliases", func(t *testing.T) {
		response := map[string]any{
			"data": []any{
				map[string]any{
					"3": map[string]any{"value": 123},
					"6": map[string]any{"value": "Project A"},
					"7": map[string]any{"value": "Active"},
				},
			},
		}

		result := TransformResponse(response, schema, "bqw3ryzab")

		data := result["data"].([]any)
		record := data[0].(map[string]any)

		if record["id"] != 123 {
			t.Errorf("record['id'] = %v, want 123", record["id"])
		}
		if record["name"] != "Project A" {
			t.Errorf("record['name'] = %v, want 'Project A'", record["name"])
		}
		if record["status"] != "Active" {
			t.Errorf("record['status'] = %v, want 'Active'", record["status"])
		}
	})

	t.Run("keeps unknown field IDs as-is", func(t *testing.T) {
		response := map[string]any{
			"data": []any{
				map[string]any{
					"6":   map[string]any{"value": "Project A"},
					"999": map[string]any{"value": "Unknown field"},
				},
			},
		}

		result := TransformResponse(response, schema, "bqw3ryzab")

		data := result["data"].([]any)
		record := data[0].(map[string]any)

		if record["name"] != "Project A" {
			t.Errorf("record['name'] = %v, want 'Project A'", record["name"])
		}
		// Unknown field should keep numeric key but still unwrap value
		if record["999"] != "Unknown field" {
			t.Errorf("record['999'] = %v, want 'Unknown field'", record["999"])
		}
	})

	t.Run("passes through without schema", func(t *testing.T) {
		response := map[string]any{
			"data": []any{
				map[string]any{
					"6": map[string]any{"value": "Project A"},
				},
			},
		}

		result := TransformResponse(response, nil, "bqw3ryzab")

		data := result["data"].([]any)
		record := data[0].(map[string]any)

		// Without schema, should keep field ID but unwrap value
		if record["6"] != "Project A" {
			t.Errorf("record['6'] = %v, want 'Project A'", record["6"])
		}
	})

	t.Run("handles empty tableID", func(t *testing.T) {
		response := map[string]any{
			"data": []any{
				map[string]any{
					"6": map[string]any{"value": "Project A"},
				},
			},
		}

		result := TransformResponse(response, schema, "")

		// Should pass through data unchanged when no tableID
		data := result["data"].([]any)
		record := data[0].(map[string]any)
		wrapped := record["6"].(map[string]any)
		if wrapped["value"] != "Project A" {
			t.Errorf("expected unchanged data when tableID is empty")
		}
	})
}

func TestUnwrapFieldValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: nil,
		},
		{
			name:     "simple wrapped value",
			input:    map[string]any{"value": "test"},
			expected: "test",
		},
		{
			name:     "wrapped number",
			input:    map[string]any{"value": 123},
			expected: 123,
		},
		{
			name:     "wrapped boolean",
			input:    map[string]any{"value": true},
			expected: true,
		},
		{
			name:     "unwrapped string",
			input:    "test",
			expected: "test",
		},
		{
			name:     "array of wrapped values",
			input:    []any{map[string]any{"value": 1}, map[string]any{"value": 2}},
			expected: []any{1, 2},
		},
		{
			name:     "nested wrapped value",
			input:    map[string]any{"value": map[string]any{"value": "nested"}},
			expected: "nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unwrapFieldValue(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("unwrapFieldValue = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractTableIDFromBody(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]any
		expected string
	}{
		{
			name:     "nil body",
			body:     nil,
			expected: "",
		},
		{
			name:     "from field",
			body:     map[string]any{"from": "bqw3ryzab"},
			expected: "bqw3ryzab",
		},
		{
			name:     "to field",
			body:     map[string]any{"to": "bqw3ryzab"},
			expected: "bqw3ryzab",
		},
		{
			name:     "from takes precedence over to",
			body:     map[string]any{"from": "fromTable", "to": "toTable"},
			expected: "fromTable",
		},
		{
			name:     "no table reference",
			body:     map[string]any{"data": []any{}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTableIDFromBody(tt.body)
			if result != tt.expected {
				t.Errorf("extractTableIDFromBody = %q, want %q", result, tt.expected)
			}
		})
	}
}
