// Request and response transformation utilities for schema aliases.
//
// These functions transform requests (converting aliases to IDs) and responses
// (converting IDs to aliases and unwrapping values).
package core

import (
	"fmt"
	"regexp"
	"strconv"
)

// TransformRequest transforms a request body, resolving table and field aliases to IDs.
// Handles: from, to, select, sortBy, groupBy, where, data
func TransformRequest(body map[string]any, schema *ResolvedSchema) (map[string]any, string, error) {
	if schema == nil {
		// Extract tableID from body even without schema
		tableID := extractTableIDFromBody(body)
		return body, tableID, nil
	}

	result := make(map[string]any)
	for k, v := range body {
		result[k] = v
	}

	var tableID string
	var err error

	// Resolve table references (from, to)
	if from, ok := result["from"].(string); ok {
		tableID, err = ResolveTableAlias(schema, from)
		if err != nil {
			return nil, "", err
		}
		result["from"] = tableID
	}
	if to, ok := result["to"].(string); ok {
		tableID, err = ResolveTableAlias(schema, to)
		if err != nil {
			return nil, "", err
		}
		result["to"] = tableID
	}

	// Need tableID to resolve field aliases
	if tableID == "" {
		return result, "", nil
	}

	// Resolve select array
	if selectArr, ok := result["select"].([]any); ok {
		resolved := make([]int, 0, len(selectArr))
		for _, field := range selectArr {
			fieldID, err := ResolveFieldAlias(schema, tableID, field)
			if err != nil {
				return nil, "", err
			}
			resolved = append(resolved, fieldID)
		}
		result["select"] = resolved
	}

	// Resolve sortBy array
	if sortBy, ok := result["sortBy"].([]any); ok {
		resolved := make([]map[string]any, 0, len(sortBy))
		for _, item := range sortBy {
			if sortItem, ok := item.(map[string]any); ok {
				newItem := make(map[string]any)
				for k, v := range sortItem {
					newItem[k] = v
				}
				if fieldRef, ok := sortItem["fieldId"]; ok {
					fieldID, err := ResolveFieldAlias(schema, tableID, fieldRef)
					if err != nil {
						return nil, "", err
					}
					newItem["fieldId"] = fieldID
				}
				resolved = append(resolved, newItem)
			}
		}
		result["sortBy"] = resolved
	}

	// Resolve groupBy array
	if groupBy, ok := result["groupBy"].([]any); ok {
		resolved := make([]map[string]any, 0, len(groupBy))
		for _, item := range groupBy {
			if groupItem, ok := item.(map[string]any); ok {
				newItem := make(map[string]any)
				for k, v := range groupItem {
					newItem[k] = v
				}
				if fieldRef, ok := groupItem["fieldId"]; ok {
					fieldID, err := ResolveFieldAlias(schema, tableID, fieldRef)
					if err != nil {
						return nil, "", err
					}
					newItem["fieldId"] = fieldID
				}
				resolved = append(resolved, newItem)
			}
		}
		result["groupBy"] = resolved
	}

	// Resolve where clause (string replacement)
	if where, ok := result["where"].(string); ok {
		result["where"] = transformWhereClause(where, schema, tableID)
	}

	// Resolve data array (for upsert)
	if data, ok := result["data"].([]any); ok {
		resolved := make([]map[string]any, 0, len(data))
		for _, item := range data {
			if record, ok := item.(map[string]any); ok {
				transformed, err := transformRecordForRequest(record, schema, tableID)
				if err != nil {
					return nil, "", err
				}
				resolved = append(resolved, transformed)
			}
		}
		result["data"] = resolved
	}

	return result, tableID, nil
}

// wherePattern matches field references in where clauses: {'fieldAlias'. or {fieldAlias.
var wherePattern = regexp.MustCompile(`\{['"]?([^.'"}\]]+)['"]?\.`)

// transformWhereClause transforms field aliases in a where clause to field IDs.
func transformWhereClause(where string, schema *ResolvedSchema, tableID string) string {
	return wherePattern.ReplaceAllStringFunc(where, func(match string) string {
		// Extract the field reference from the match
		submatch := wherePattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		fieldRef := submatch[1]

		// Try to resolve as alias
		fieldID, err := ResolveFieldAlias(schema, tableID, fieldRef)
		if err != nil {
			// If not found, keep original (might be a raw ID)
			return match
		}
		return fmt.Sprintf("{%d.", fieldID)
	})
}

// transformRecordForRequest transforms a record's field alias keys to IDs.
func transformRecordForRequest(record map[string]any, schema *ResolvedSchema, tableID string) (map[string]any, error) {
	result := make(map[string]any)

	for key, value := range record {
		// Try to resolve the key as a field alias
		fieldID, err := ResolveFieldAlias(schema, tableID, key)
		if err != nil {
			// If not an alias, keep the original key (might already be an ID)
			result[key] = value
		} else {
			result[strconv.Itoa(fieldID)] = value
		}
	}

	return result, nil
}

// TransformResponse transforms a response, converting field IDs to aliases and unwrapping values.
func TransformResponse(response map[string]any, schema *ResolvedSchema, tableID string) map[string]any {
	if response == nil {
		return nil
	}

	result := make(map[string]any)
	for k, v := range response {
		result[k] = v
	}

	// Handle data array (runQuery, upsert responses)
	if data, ok := result["data"].([]any); ok && tableID != "" {
		transformed := make([]any, 0, len(data))
		for _, item := range data {
			if record, ok := item.(map[string]any); ok {
				transformed = append(transformed, transformRecordForResponse(record, schema, tableID))
			} else {
				transformed = append(transformed, item)
			}
		}
		result["data"] = transformed
	}

	return result
}

// transformRecordForResponse transforms a record from the response.
// - Converts field ID keys to aliases (if schema defined)
// - Unwraps { value: X } to just X
func transformRecordForResponse(record map[string]any, schema *ResolvedSchema, tableID string) map[string]any {
	result := make(map[string]any)

	for key, value := range record {
		// Determine the output key (alias or original)
		outputKey := key

		// Try to parse key as field ID and get alias
		if fieldID, err := strconv.Atoi(key); err == nil && schema != nil {
			if alias := GetFieldAlias(schema, tableID, fieldID); alias != "" {
				outputKey = alias
			}
		}

		// Unwrap { value: X } -> X
		unwrappedValue := unwrapFieldValue(value)
		result[outputKey] = unwrappedValue
	}

	return result
}

// unwrapFieldValue unwraps a field value from QuickBase's { value: X } format.
func unwrapFieldValue(value any) any {
	if value == nil {
		return nil
	}

	// Check if it's a map with a "value" key
	if obj, ok := value.(map[string]any); ok {
		if val, hasValue := obj["value"]; hasValue {
			// Recursively unwrap in case value is an array of wrapped values
			return unwrapFieldValue(val)
		}
	}

	// Handle array of { value: X } objects
	if arr, ok := value.([]any); ok {
		result := make([]any, 0, len(arr))
		for _, item := range arr {
			result = append(result, unwrapFieldValue(item))
		}
		return result
	}

	return value
}

// extractTableIDFromBody extracts the table ID from a request body.
func extractTableIDFromBody(body map[string]any) string {
	if body == nil {
		return ""
	}

	// Check 'from' (runQuery, deleteRecords)
	if from, ok := body["from"].(string); ok {
		return from
	}

	// Check 'to' (upsert)
	if to, ok := body["to"].(string); ok {
		return to
	}

	return ""
}
