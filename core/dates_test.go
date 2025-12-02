package core

import (
	"testing"
	"time"
)

func TestIsISODateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Date-only strings
		{"date only", "2024-01-15", true},
		{"date only end of year", "2024-12-31", true},

		// Date-time strings
		{"datetime without timezone", "2024-01-15T10:30:00", true},
		{"datetime with Z", "2024-01-15T10:30:00Z", true},
		{"datetime with milliseconds", "2024-01-15T10:30:00.000Z", true},
		{"datetime with 3 digit ms", "2024-01-15T10:30:00.123Z", true},

		// Date-time with timezone offset
		{"datetime with +00:00", "2024-01-15T10:30:00+00:00", true},
		{"datetime with -05:00", "2024-01-15T10:30:00-05:00", true},
		{"datetime with +0530 no colon", "2024-01-15T10:30:00+0530", true},

		// Non-date strings
		{"plain text", "hello world", false},
		{"year only", "2024", false},
		{"year-month only", "2024-01", false},
		{"US date format", "01-15-2024", false},
		{"empty string", "", false},
		{"number string", "12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsISODateString(tt.input)
			if result != tt.expected {
				t.Errorf("IsISODateString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseISODate(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		checkYear   int
		checkMonth  time.Month
		checkDay    int
	}{
		{"date only", "2024-01-15", false, 2024, time.January, 15},
		{"datetime with Z", "2024-01-15T10:30:00Z", false, 2024, time.January, 15},
		{"datetime with milliseconds", "2024-01-15T10:30:00.000Z", false, 2024, time.January, 15},
		{"datetime RFC3339", "2024-03-20T14:45:00+00:00", false, 2024, time.March, 20},
		{"invalid date", "not-a-date", true, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseISODate(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseISODate(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseISODate(%q) unexpected error: %v", tt.input, err)
				return
			}
			if result.Year() != tt.checkYear {
				t.Errorf("ParseISODate(%q) year = %d, want %d", tt.input, result.Year(), tt.checkYear)
			}
			if result.Month() != tt.checkMonth {
				t.Errorf("ParseISODate(%q) month = %v, want %v", tt.input, result.Month(), tt.checkMonth)
			}
			if result.Day() != tt.checkDay {
				t.Errorf("ParseISODate(%q) day = %d, want %d", tt.input, result.Day(), tt.checkDay)
			}
		})
	}
}

func TestTransformDates(t *testing.T) {
	t.Run("converts ISO date strings to time.Time", func(t *testing.T) {
		input := map[string]any{
			"created": "2024-01-15T10:30:00.000Z",
			"updated": "2024-03-20T14:45:00.000Z",
		}

		result := TransformDates(input, true)

		created, ok := result["created"].(time.Time)
		if !ok {
			t.Errorf("expected created to be time.Time, got %T", result["created"])
		}
		if created.Year() != 2024 || created.Month() != time.January || created.Day() != 15 {
			t.Errorf("created date incorrect: %v", created)
		}

		updated, ok := result["updated"].(time.Time)
		if !ok {
			t.Errorf("expected updated to be time.Time, got %T", result["updated"])
		}
		if updated.Year() != 2024 || updated.Month() != time.March || updated.Day() != 20 {
			t.Errorf("updated date incorrect: %v", updated)
		}
	})

	t.Run("preserves non-date strings", func(t *testing.T) {
		input := map[string]any{
			"name": "Test Application",
			"id":   "bpqe82s1",
		}

		result := TransformDates(input, true)

		if result["name"] != "Test Application" {
			t.Errorf("name = %v, want 'Test Application'", result["name"])
		}
		if result["id"] != "bpqe82s1" {
			t.Errorf("id = %v, want 'bpqe82s1'", result["id"])
		}
	})

	t.Run("handles nested objects", func(t *testing.T) {
		input := map[string]any{
			"app": map[string]any{
				"name":    "Test App",
				"created": "2024-01-15T10:30:00.000Z",
				"metadata": map[string]any{
					"updated": "2024-03-20T14:45:00.000Z",
				},
			},
		}

		result := TransformDates(input, true)

		app := result["app"].(map[string]any)
		if app["name"] != "Test App" {
			t.Errorf("app.name = %v, want 'Test App'", app["name"])
		}
		if _, ok := app["created"].(time.Time); !ok {
			t.Errorf("app.created should be time.Time, got %T", app["created"])
		}

		metadata := app["metadata"].(map[string]any)
		if _, ok := metadata["updated"].(time.Time); !ok {
			t.Errorf("app.metadata.updated should be time.Time, got %T", metadata["updated"])
		}
	})

	t.Run("handles arrays", func(t *testing.T) {
		input := map[string]any{
			"items": []any{
				map[string]any{"id": 1, "created": "2024-01-01T00:00:00Z"},
				map[string]any{"id": 2, "created": "2024-02-01T00:00:00Z"},
			},
		}

		result := TransformDates(input, true)

		items := result["items"].([]any)
		if len(items) != 2 {
			t.Errorf("items length = %d, want 2", len(items))
		}

		item0 := items[0].(map[string]any)
		if _, ok := item0["created"].(time.Time); !ok {
			t.Errorf("items[0].created should be time.Time, got %T", item0["created"])
		}

		item1 := items[1].(map[string]any)
		if _, ok := item1["created"].(time.Time); !ok {
			t.Errorf("items[1].created should be time.Time, got %T", item1["created"])
		}
	})

	t.Run("skips transformation when disabled", func(t *testing.T) {
		input := map[string]any{
			"created": "2024-01-15T10:30:00.000Z",
			"name":    "Test",
		}

		result := TransformDates(input, false)

		if result["created"] != "2024-01-15T10:30:00.000Z" {
			t.Errorf("created should remain string when disabled")
		}
		if _, ok := result["created"].(string); !ok {
			t.Errorf("created should be string, got %T", result["created"])
		}
	})

	t.Run("handles nil input", func(t *testing.T) {
		result := TransformDates(nil, true)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("converts date-only strings", func(t *testing.T) {
		input := map[string]any{
			"date": "2024-01-15",
		}

		result := TransformDates(input, true)

		if _, ok := result["date"].(time.Time); !ok {
			t.Errorf("date should be time.Time, got %T", result["date"])
		}
	})
}

func TestTransformDatesQuickBaseResponse(t *testing.T) {
	t.Run("transforms typical getApp response", func(t *testing.T) {
		response := map[string]any{
			"id":          "bpqe82s1",
			"name":        "Test Application",
			"description": "A test application",
			"created":     "2024-01-15T10:30:00.000Z",
			"updated":     "2024-03-20T14:45:00.000Z",
			"dateFormat":  "MM-DD-YYYY",
			"variables": []any{
				map[string]any{"name": "AppVersion", "value": "1.0.0"},
			},
		}

		result := TransformDates(response, true)

		if result["id"] != "bpqe82s1" {
			t.Errorf("id = %v, want 'bpqe82s1'", result["id"])
		}
		if result["name"] != "Test Application" {
			t.Errorf("name = %v, want 'Test Application'", result["name"])
		}
		if _, ok := result["created"].(time.Time); !ok {
			t.Errorf("created should be time.Time, got %T", result["created"])
		}
		if _, ok := result["updated"].(time.Time); !ok {
			t.Errorf("updated should be time.Time, got %T", result["updated"])
		}
		// dateFormat should NOT be converted (it's a format string, not a date)
		if result["dateFormat"] != "MM-DD-YYYY" {
			t.Errorf("dateFormat = %v, want 'MM-DD-YYYY'", result["dateFormat"])
		}

		variables := result["variables"].([]any)
		variable := variables[0].(map[string]any)
		if variable["name"] != "AppVersion" {
			t.Errorf("variable name = %v, want 'AppVersion'", variable["name"])
		}
	})

	t.Run("transforms runQuery response with record data", func(t *testing.T) {
		response := map[string]any{
			"data": []any{
				map[string]any{
					"3": map[string]any{"value": 1},
					"6": map[string]any{"value": "Test Record"},
					"7": map[string]any{"value": "2024-01-15T10:30:00.000Z"},
				},
			},
			"fields": []any{
				map[string]any{"id": 3, "label": "Record ID#", "type": "recordid"},
				map[string]any{"id": 6, "label": "Name", "type": "text"},
				map[string]any{"id": 7, "label": "Created", "type": "timestamp"},
			},
			"metadata": map[string]any{
				"totalRecords": 1,
				"numRecords":   1,
				"skip":         0,
			},
		}

		result := TransformDates(response, true)

		data := result["data"].([]any)
		record := data[0].(map[string]any)
		field7 := record["7"].(map[string]any)

		// The date value inside the record should be converted
		if _, ok := field7["value"].(time.Time); !ok {
			t.Errorf("field 7 value should be time.Time, got %T", field7["value"])
		}

		field6 := record["6"].(map[string]any)
		if field6["value"] != "Test Record" {
			t.Errorf("field 6 value = %v, want 'Test Record'", field6["value"])
		}
	})
}
