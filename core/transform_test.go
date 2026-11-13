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

	t.Run("returns error for a misspelled table alias", func(t *testing.T) {
		body := map[string]any{
			"from": "projcts",
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
