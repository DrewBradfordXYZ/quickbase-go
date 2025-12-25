package client

import (
	"encoding/json"

	"github.com/DrewBradfordXYZ/quickbase-go/v2/generated"
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

// SortFieldsToSortByUnion converts a slice of SortField to a RunQueryJSONBody_SortBy union type.
// The SortBy parameter accepts either an array of sort specifications or false (to disable sorting).
func SortFieldsToSortByUnion(fields []generated.SortField) (*generated.RunQueryJSONBody_SortBy, error) {
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

// ExtractSortFields extracts the sort fields from a RunQueryJSONBody_SortBy union.
// Returns the slice of SortField values and nil error if successful.
func ExtractSortFields(sortBy *generated.RunQueryJSONBody_SortBy) ([]generated.SortField, error) {
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

// Record is a friendly type alias for query result records.
// Field keys are either field IDs (as strings like "6") or aliases (if schema is configured).
// Values are unwrapped from FieldValue wrappers to their actual Go types.
type Record = map[string]any

// unwrapRecord converts a generated QuickbaseRecord to a friendly map[string]any.
// It unwraps each FieldValue to expose the actual value directly.
func unwrapRecord(record generated.QuickbaseRecord) Record {
	result := make(Record)
	for k, fv := range record {
		result[k] = extractValue(fv.Value)
	}
	return result
}

// unwrapRecords converts a slice of generated QuickbaseRecords to friendly Records.
func unwrapRecords(records []generated.QuickbaseRecord) []Record {
	result := make([]Record, len(records))
	for i, record := range records {
		result[i] = unwrapRecord(record)
	}
	return result
}

// UnwrapRecords converts QuickbaseRecord slices to []map[string]any.
// This is an opt-in helper for when you want friendlier access to record data.
// The raw QuickbaseRecord wraps each field value in a FieldValue struct;
// this function extracts the actual values.
//
// Example:
//
//	resp, _ := client.RunQuery(ctx, body)
//	if resp.JSON200.Data != nil {
//	    records := quickbase.UnwrapRecords(*resp.JSON200.Data)
//	    for _, rec := range records {
//	        name := rec["6"] // access by field ID
//	    }
//	}
func UnwrapRecords(records []generated.QuickbaseRecord) []Record {
	return unwrapRecords(records)
}

// UnwrapRecord converts a single QuickbaseRecord to map[string]any.
func UnwrapRecord(record generated.QuickbaseRecord) Record {
	return unwrapRecord(record)
}

// Deref returns the value of a pointer, or zero value if nil.
// This is an opt-in helper for working with the many optional (pointer) fields
// in generated response types.
//
// Example:
//
//	resp, _ := client.GetApp(appId).Run(ctx)
//	name := quickbase.Deref(resp.JSON200.Name) // string, not *string
func Deref[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// DerefOr returns the value of a pointer, or the default if nil.
// This is an opt-in helper for working with optional pointer fields.
//
// Example:
//
//	resp, _ := client.GetApp(appId).Run(ctx)
//	name := quickbase.DerefOr(resp.JSON200.Name, "Unknown")
func DerefOr[T any](ptr *T, defaultVal T) T {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

// extractValue unwraps a FieldValue_Value union to its actual Go value.
// The FieldValue_Value is a union type that can contain:
// - string (text, rich text, phone, email, URL, etc.)
// - float64 (numeric, currency, percent, rating, duration)
// - bool (checkbox)
// - []any (multi-select, address, etc.)
// - map[string]any (user, file attachment, etc.)
func extractValue(v generated.FieldValue_Value) any {
	// The union type stores data as json.RawMessage internally.
	// We marshal it to JSON and then unmarshal to any to get the actual value.
	data, err := v.MarshalJSON()
	if err != nil {
		return nil
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

// --- Value Wrapping (for creating records) ---

// wrapValue converts a Go value to a FieldValue suitable for record creation.
// This is the inverse of extractValue.
func wrapValue(v any) generated.FieldValue {
	wrapped := map[string]any{"value": v}
	data, _ := json.Marshal(wrapped)
	var fv generated.FieldValue
	_ = json.Unmarshal(data, &fv)
	return fv
}

// wrapRecord converts a friendly Record to a generated QuickbaseRecord.
// This is the inverse of unwrapRecord.
func wrapRecord(record Record) generated.QuickbaseRecord {
	result := make(generated.QuickbaseRecord)
	for k, v := range record {
		result[k] = wrapValue(v)
	}
	return result
}
