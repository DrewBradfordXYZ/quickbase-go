package core

import (
	"regexp"
	"time"
)

// ISO date pattern matches: 2024-01-15, 2024-01-15T10:30:00, 2024-01-15T10:30:00.000Z, etc.
var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}:\d{2}(\.\d{1,3})?(Z|[+-]\d{2}:?\d{2})?)?$`)

// IsISODateString checks if a string looks like an ISO 8601 date.
func IsISODateString(value string) bool {
	return isoDatePattern.MatchString(value)
}

// ParseISODate parses an ISO 8601 date string to time.Time.
func ParseISODate(value string) (time.Time, error) {
	// Try various ISO 8601 formats
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, &time.ParseError{Value: value, Message: "not a valid ISO 8601 date"}
}

// TransformDates recursively transforms ISO date strings to time.Time in a map.
// This modifies the map in place and returns it.
func TransformDates(data map[string]any, enabled bool) map[string]any {
	if !enabled || data == nil {
		return data
	}

	for key, value := range data {
		data[key] = transformValue(value, enabled)
	}

	return data
}

func transformValue(value any, enabled bool) any {
	if !enabled {
		return value
	}

	switch v := value.(type) {
	case string:
		if IsISODateString(v) {
			if t, err := ParseISODate(v); err == nil {
				return t
			}
		}
		return v

	case map[string]any:
		return TransformDates(v, enabled)

	case []any:
		for i, item := range v {
			v[i] = transformValue(item, enabled)
		}
		return v

	default:
		return v
	}
}
