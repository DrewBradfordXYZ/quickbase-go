package client

import (
	"encoding/json"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/generated"
)

// --- Union Type Helpers ---
// These functions help work with oapi-codegen union types that don't have
// generated From* methods. The union types have private fields, so we use
// JSON round-tripping through the parent type to set values.

// StringToWhereUnion converts a string to a RunQueryJSONBody_Where union type.
// The Where parameter accepts either a query string or an array of record IDs.
func StringToWhereUnion(s string) (*generated.RunQueryJSONBody_Where, error) {
	// Create a minimal body with just the where clause
	bodyMap := map[string]any{
		"from":  "placeholder", // Required field
		"where": s,
	}

	jsonBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	var body generated.RunQueryJSONRequestBody
	if err := json.Unmarshal(jsonBytes, &body); err != nil {
		return nil, err
	}

	return body.Where, nil
}

// SortFieldsToSortByUnion converts a slice of SortField to a SortByUnion type.
// The SortBy parameter accepts either an array of sort specifications or false (to disable sorting).
func SortFieldsToSortByUnion(fields []generated.SortField) (*generated.SortByUnion, error) {
	// Convert SortField slice to raw format for JSON
	sortByRaw := make([]map[string]any, len(fields))
	for i, f := range fields {
		sortByRaw[i] = map[string]any{
			"fieldId": f.FieldId,
			"order":   f.Order,
		}
	}

	// Create a minimal body with just the sortBy clause
	bodyMap := map[string]any{
		"from":   "placeholder", // Required field
		"sortBy": sortByRaw,
	}

	jsonBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	var body generated.RunQueryJSONRequestBody
	if err := json.Unmarshal(jsonBytes, &body); err != nil {
		return nil, err
	}

	return body.SortBy, nil
}

// StringToDeleteWhereUnion converts a string to a DeleteRecordsJSONBody_Where union type.
// The Where parameter accepts either a query string or an array of record IDs.
func StringToDeleteWhereUnion(s string) (generated.DeleteRecordsJSONBody_Where, error) {
	// Create a minimal body with just the where clause
	bodyMap := map[string]any{
		"from":  "placeholder", // Required field
		"where": s,
	}

	jsonBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return generated.DeleteRecordsJSONBody_Where{}, err
	}

	var body generated.DeleteRecordsJSONRequestBody
	if err := json.Unmarshal(jsonBytes, &body); err != nil {
		return generated.DeleteRecordsJSONBody_Where{}, err
	}

	return body.Where, nil
}

// extractWhereString extracts the where string from a RunQueryJSONBody_Where union.
// Returns the string value and true if successful, or empty string and false if not a string.
func extractWhereString(where *generated.RunQueryJSONBody_Where) (string, bool) {
	if where == nil {
		return "", false
	}

	// Marshal the parent body to JSON and extract where
	body := generated.RunQueryJSONRequestBody{
		From:  "placeholder",
		Where: where,
	}

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return "", false
	}

	var bodyMap map[string]any
	if err := json.Unmarshal(jsonBytes, &bodyMap); err != nil {
		return "", false
	}

	if whereVal, ok := bodyMap["where"]; ok {
		if whereStr, ok := whereVal.(string); ok {
			return whereStr, true
		}
	}

	return "", false
}

// ExtractSortFields extracts the sort fields from a SortByUnion.
// Returns the slice of SortField values and nil error if successful.
func ExtractSortFields(sortBy *generated.SortByUnion) ([]generated.SortField, error) {
	if sortBy == nil {
		return nil, nil
	}

	// Marshal the parent body to JSON and extract sortBy
	body := generated.RunQueryJSONRequestBody{
		From:   "placeholder",
		SortBy: sortBy,
	}

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	var bodyMap map[string]any
	if err := json.Unmarshal(jsonBytes, &bodyMap); err != nil {
		return nil, err
	}

	sortByVal, ok := bodyMap["sortBy"]
	if !ok {
		return nil, nil
	}

	// sortBy should be an array of {fieldId, order}
	sortByArr, ok := sortByVal.([]any)
	if !ok {
		return nil, nil
	}

	sortFields := make([]generated.SortField, len(sortByArr))
	for i, item := range sortByArr {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if fieldId, ok := itemMap["fieldId"].(float64); ok {
			sortFields[i].FieldId = int(fieldId)
		}
		if order, ok := itemMap["order"].(string); ok {
			sortFields[i].Order = generated.SortFieldOrder(order)
		}
	}

	return sortFields, nil
}

// --- Record Transformation ---
// These functions transform records from generated types to friendly user-facing types.

// Record is a record as a map: field ID keys ("6"), or aliases with a schema
// (WithSchemaOptions TransformResponses), to plain values (string, float64,
// bool, []any, map[string]any, nil).
type Record = map[string]any

// UnwrapRecord turns a generated record ({"6": {"value": "Alpha"}}) into a
// Record ({"6": "Alpha"}).
func UnwrapRecord(record generated.QuickbaseRecord) Record {
	result := make(Record, len(record))
	for k, v := range record {
		result[k] = v.Value
	}
	return result
}

// UnwrapRecords turns generated records into Records.
func UnwrapRecords(records []generated.QuickbaseRecord) []Record {
	result := make([]Record, len(records))
	for i, record := range records {
		result[i] = UnwrapRecord(record)
	}
	return result
}

// Deref returns the value of a pointer, or the zero value if it is nil: for
// the optional fields of request types, which stay pointers.
func Deref[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// DerefOr returns the value of a pointer, or def if it is nil.
func DerefOr[T any](ptr *T, def T) T {
	if ptr == nil {
		return def
	}
	return *ptr
}
