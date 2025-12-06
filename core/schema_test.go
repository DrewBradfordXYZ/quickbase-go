package core

import (
	"testing"
)

func TestResolveSchema(t *testing.T) {
	t.Run("returns nil for nil schema", func(t *testing.T) {
		result := ResolveSchema(nil)
		if result != nil {
			t.Errorf("ResolveSchema(nil) = %v, want nil", result)
		}
	})

	t.Run("builds lookup maps correctly", func(t *testing.T) {
		schema := &Schema{
			Tables: map[string]TableSchema{
				"projects": {
					ID: "bqw3ryzab",
					Fields: map[string]int{
						"id":     3,
						"name":   6,
						"status": 7,
					},
				},
				"tasks": {
					ID: "bqw4xyzcd",
					Fields: map[string]int{
						"id":        3,
						"title":     6,
						"projectId": 8,
					},
				},
			},
		}

		resolved := ResolveSchema(schema)

		// Check table alias to ID mapping
		if resolved.TableAliasToID["projects"] != "bqw3ryzab" {
			t.Errorf("TableAliasToID[projects] = %q, want %q", resolved.TableAliasToID["projects"], "bqw3ryzab")
		}
		if resolved.TableAliasToID["tasks"] != "bqw4xyzcd" {
			t.Errorf("TableAliasToID[tasks] = %q, want %q", resolved.TableAliasToID["tasks"], "bqw4xyzcd")
		}

		// Check table ID to alias mapping
		if resolved.TableIDToAlias["bqw3ryzab"] != "projects" {
			t.Errorf("TableIDToAlias[bqw3ryzab] = %q, want %q", resolved.TableIDToAlias["bqw3ryzab"], "projects")
		}

		// Check field alias to ID mapping
		if resolved.FieldAliasToID["bqw3ryzab"]["name"] != 6 {
			t.Errorf("FieldAliasToID[bqw3ryzab][name] = %d, want 6", resolved.FieldAliasToID["bqw3ryzab"]["name"])
		}

		// Check field ID to alias mapping
		if resolved.FieldIDToAlias["bqw3ryzab"][6] != "name" {
			t.Errorf("FieldIDToAlias[bqw3ryzab][6] = %q, want %q", resolved.FieldIDToAlias["bqw3ryzab"][6], "name")
		}
	})
}

func TestResolveTableAlias(t *testing.T) {
	schema := ResolveSchema(&Schema{
		Tables: map[string]TableSchema{
			"projects": {ID: "bqw3ryzab", Fields: map[string]int{}},
			"tasks":    {ID: "bqw4xyzcd", Fields: map[string]int{}},
		},
	})

	t.Run("returns table ID unchanged if no schema", func(t *testing.T) {
		result, err := ResolveTableAlias(nil, "bqw3ryzab")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "bqw3ryzab" {
			t.Errorf("ResolveTableAlias = %q, want %q", result, "bqw3ryzab")
		}
	})

	t.Run("resolves alias to ID", func(t *testing.T) {
		result, err := ResolveTableAlias(schema, "projects")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "bqw3ryzab" {
			t.Errorf("ResolveTableAlias = %q, want %q", result, "bqw3ryzab")
		}
	})

	t.Run("returns table ID unchanged if already an ID", func(t *testing.T) {
		result, err := ResolveTableAlias(schema, "bqw3ryzab")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "bqw3ryzab" {
			t.Errorf("ResolveTableAlias = %q, want %q", result, "bqw3ryzab")
		}
	})

	t.Run("returns error for unknown alias", func(t *testing.T) {
		_, err := ResolveTableAlias(schema, "unknown")
		if err == nil {
			t.Fatal("expected error for unknown alias")
		}
		schemaErr, ok := err.(*SchemaError)
		if !ok {
			t.Fatalf("expected *SchemaError, got %T", err)
		}
		if schemaErr.Message == "" {
			t.Error("SchemaError.Message should not be empty")
		}
	})

	t.Run("suggests similar alias on typo", func(t *testing.T) {
		_, err := ResolveTableAlias(schema, "projcts") // typo
		if err == nil {
			t.Fatal("expected error")
		}
		schemaErr := err.(*SchemaError)
		// Should contain "Did you mean"
		if schemaErr.Message == "" {
			t.Error("expected error message with suggestion")
		}
	})
}

func TestResolveFieldAlias(t *testing.T) {
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

	t.Run("returns int field ID unchanged", func(t *testing.T) {
		result, err := ResolveFieldAlias(schema, "bqw3ryzab", 6)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 6 {
			t.Errorf("ResolveFieldAlias = %d, want 6", result)
		}
	})

	t.Run("resolves string alias to ID", func(t *testing.T) {
		result, err := ResolveFieldAlias(schema, "bqw3ryzab", "name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 6 {
			t.Errorf("ResolveFieldAlias = %d, want 6", result)
		}
	})

	t.Run("returns error for unknown alias", func(t *testing.T) {
		_, err := ResolveFieldAlias(schema, "bqw3ryzab", "unknown")
		if err == nil {
			t.Fatal("expected error for unknown alias")
		}
		_, ok := err.(*SchemaError)
		if !ok {
			t.Fatalf("expected *SchemaError, got %T", err)
		}
	})

	t.Run("returns error without schema", func(t *testing.T) {
		_, err := ResolveFieldAlias(nil, "bqw3ryzab", "name")
		if err == nil {
			t.Fatal("expected error without schema")
		}
	})

	t.Run("returns error for non-string non-int type", func(t *testing.T) {
		_, err := ResolveFieldAlias(schema, "bqw3ryzab", 3.14)
		if err == nil {
			t.Fatal("expected error for float type")
		}
	})

	t.Run("suggests similar field on typo", func(t *testing.T) {
		_, err := ResolveFieldAlias(schema, "bqw3ryzab", "stauts") // typo
		if err == nil {
			t.Fatal("expected error")
		}
		schemaErr := err.(*SchemaError)
		// Should contain suggestion
		if schemaErr.Message == "" {
			t.Error("expected error message with suggestion")
		}
	})
}

func TestGetFieldAlias(t *testing.T) {
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

	t.Run("returns empty string for nil schema", func(t *testing.T) {
		result := GetFieldAlias(nil, "bqw3ryzab", 6)
		if result != "" {
			t.Errorf("GetFieldAlias = %q, want empty string", result)
		}
	})

	t.Run("returns alias for known field ID", func(t *testing.T) {
		result := GetFieldAlias(schema, "bqw3ryzab", 6)
		if result != "name" {
			t.Errorf("GetFieldAlias = %q, want %q", result, "name")
		}
	})

	t.Run("returns empty string for unknown field ID", func(t *testing.T) {
		result := GetFieldAlias(schema, "bqw3ryzab", 999)
		if result != "" {
			t.Errorf("GetFieldAlias = %q, want empty string", result)
		}
	})

	t.Run("returns empty string for unknown table ID", func(t *testing.T) {
		result := GetFieldAlias(schema, "unknown", 6)
		if result != "" {
			t.Errorf("GetFieldAlias = %q, want empty string", result)
		}
	})
}

func TestGetTableAlias(t *testing.T) {
	schema := ResolveSchema(&Schema{
		Tables: map[string]TableSchema{
			"projects": {ID: "bqw3ryzab", Fields: map[string]int{}},
		},
	})

	t.Run("returns empty string for nil schema", func(t *testing.T) {
		result := GetTableAlias(nil, "bqw3ryzab")
		if result != "" {
			t.Errorf("GetTableAlias = %q, want empty string", result)
		}
	})

	t.Run("returns alias for known table ID", func(t *testing.T) {
		result := GetTableAlias(schema, "bqw3ryzab")
		if result != "projects" {
			t.Errorf("GetTableAlias = %q, want %q", result, "projects")
		}
	})

	t.Run("returns empty string for unknown table ID", func(t *testing.T) {
		result := GetTableAlias(schema, "unknown")
		if result != "" {
			t.Errorf("GetTableAlias = %q, want empty string", result)
		}
	})
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"projects", "projcts", 1},
		{"status", "stauts", 2},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			result := levenshteinDistance(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestFindSimilar(t *testing.T) {
	candidates := []string{"projects", "tasks", "users", "status"}

	t.Run("finds close match", func(t *testing.T) {
		result := findSimilar("projcts", candidates)
		if result != "projects" {
			t.Errorf("findSimilar = %q, want %q", result, "projects")
		}
	})

	t.Run("returns empty for no close match", func(t *testing.T) {
		result := findSimilar("zzzzzzzzz", candidates)
		if result != "" {
			t.Errorf("findSimilar = %q, want empty string", result)
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		result := findSimilar("PROJECTS", candidates)
		if result != "projects" {
			t.Errorf("findSimilar = %q, want %q", result, "projects")
		}
	})
}

func TestSchemaError(t *testing.T) {
	t.Run("Error() returns message", func(t *testing.T) {
		err := &SchemaError{Message: "unknown table alias 'foo'"}
		if err.Error() != "unknown table alias 'foo'" {
			t.Errorf("Error() = %q, want %q", err.Error(), "unknown table alias 'foo'")
		}
	})
}

func TestSchemaBuilder(t *testing.T) {
	t.Run("builds schema with single table", func(t *testing.T) {
		schema := NewSchema().
			Table("projects", "bqxyz123").
			Field("id", 3).
			Field("name", 6).
			Build()

		if len(schema.Tables) != 1 {
			t.Fatalf("expected 1 table, got %d", len(schema.Tables))
		}

		table, ok := schema.Tables["projects"]
		if !ok {
			t.Fatal("expected 'projects' table")
		}
		if table.ID != "bqxyz123" {
			t.Errorf("table ID = %q, want %q", table.ID, "bqxyz123")
		}
		if table.Fields["id"] != 3 {
			t.Errorf("field 'id' = %d, want 3", table.Fields["id"])
		}
		if table.Fields["name"] != 6 {
			t.Errorf("field 'name' = %d, want 6", table.Fields["name"])
		}
	})

	t.Run("builds schema with multiple tables", func(t *testing.T) {
		schema := NewSchema().
			Table("projects", "bqxyz123").
			Field("id", 3).
			Field("name", 6).
			Table("tasks", "bqabc456").
			Field("id", 3).
			Field("title", 7).
			Build()

		if len(schema.Tables) != 2 {
			t.Fatalf("expected 2 tables, got %d", len(schema.Tables))
		}

		projects := schema.Tables["projects"]
		if projects.Fields["name"] != 6 {
			t.Errorf("projects.name = %d, want 6", projects.Fields["name"])
		}

		tasks := schema.Tables["tasks"]
		if tasks.Fields["title"] != 7 {
			t.Errorf("tasks.title = %d, want 7", tasks.Fields["title"])
		}
	})

	t.Run("Field without Table is ignored", func(t *testing.T) {
		schema := NewSchema().
			Field("orphan", 99). // No table set yet
			Table("projects", "bqxyz123").
			Build()

		// Should only have the projects table, orphan field ignored
		if len(schema.Tables) != 1 {
			t.Fatalf("expected 1 table, got %d", len(schema.Tables))
		}
	})
}

func TestSchemaOptions(t *testing.T) {
	t.Run("default options enable response transformation", func(t *testing.T) {
		opts := DefaultSchemaOptions()
		if !opts.TransformResponses {
			t.Error("TransformResponses should be true by default")
		}
	})

	t.Run("ResolveSchemaWithOptions applies options", func(t *testing.T) {
		schema := &Schema{
			Tables: map[string]TableSchema{
				"test": {ID: "bq123", Fields: map[string]int{}},
			},
		}

		opts := SchemaOptions{TransformResponses: false}
		resolved := ResolveSchemaWithOptions(schema, opts)

		if resolved.Options.TransformResponses {
			t.Error("Options.TransformResponses should be false")
		}
	})

	t.Run("ResolveSchema uses default options", func(t *testing.T) {
		schema := &Schema{
			Tables: map[string]TableSchema{
				"test": {ID: "bq123", Fields: map[string]int{}},
			},
		}

		resolved := ResolveSchema(schema)

		if !resolved.Options.TransformResponses {
			t.Error("Default Options.TransformResponses should be true")
		}
	})
}
