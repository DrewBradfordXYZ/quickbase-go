package main

// ResponseTransform defines how to transform a response for better UX.
// In v2.0, we return raw generated types by default and provide opt-in helpers.
// This file is kept for future use but all transforms are disabled.
type ResponseTransform struct {
	// ResultType is the name of the result struct to generate (e.g., "UpsertResult").
	// If empty, the builder's Run() returns the generated response type directly.
	ResultType string

	// ResultFields defines the fields in the result struct.
	// Each entry maps a source path to a target field definition.
	ResultFields []FieldTransform

	// IsArrayResponse indicates the response is a root-level array (e.g., getFields returns []Field).
	// When true, ExtractFields is applied to each array element.
	IsArrayResponse bool

	// TransformResponse enables schema-based ID→alias transformation in response.
	// Only applicable for operations like runQuery that return field data.
	TransformResponse bool
}

// FieldTransform describes a single field transformation.
type FieldTransform struct {
	// Source is the path in the generated response (e.g., "metadata.skip", "id").
	// Supports dot notation for nested fields.
	Source string

	// Target is the field name in the result struct (e.g., "Skip", "ID").
	Target string

	// Type is the Go type for the result field (e.g., "int", "string", "[]int").
	Type string

	// Dereference indicates the source is a pointer that should be dereferenced.
	// When true, nil pointers are replaced with zero values.
	Dereference bool

	// TypeCast specifies a type conversion (e.g., "int" to convert int64 to int).
	TypeCast string
}

// responseTransforms is empty in v2.0 - all builders return raw generated types.
// Users can use opt-in helpers like quickbase.UnwrapRecords() and quickbase.Deref()
// when they want friendlier access patterns.
var responseTransforms = map[string]ResponseTransform{}

// hasResponseTransform returns true if the operation has a response transformation config.
func hasResponseTransform(opID string) bool {
	_, ok := responseTransforms[opID]
	return ok
}

// getResponseTransform returns the response transformation config for an operation.
func getResponseTransform(opID string) (ResponseTransform, bool) {
	t, ok := responseTransforms[opID]
	return t, ok
}
