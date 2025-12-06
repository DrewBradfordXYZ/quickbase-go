package main

// ResponseTransform defines how to transform a response for better UX.
// This config-driven approach replaces manual implementations in api.go.
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

// responseTransforms defines transformations for operations that benefit from
// simplified result types. Operations not listed here return generated types.
//
// Note: runQuery is handled specially due to its complex schema transformation.
// It remains a manual implementation in api.go.
var responseTransforms = map[string]ResponseTransform{
	// getApp: Dereferences optional pointer fields to direct values
	"getApp": {
		ResultType: "GetAppResult",
		ResultFields: []FieldTransform{
			{Source: "id", Target: "ID", Type: "string", Dereference: true},
			{Source: "name", Target: "Name", Type: "string"},
			{Source: "description", Target: "Description", Type: "string", Dereference: true},
			{Source: "created", Target: "Created", Type: "string", Dereference: true},
			{Source: "updated", Target: "Updated", Type: "string", Dereference: true},
			{Source: "dateFormat", Target: "DateFormat", Type: "string", Dereference: true},
			{Source: "timeZone", Target: "TimeZone", Type: "string", Dereference: true},
		},
	},

	// upsert: Flattens nested metadata and dereferences pointer fields
	"upsert": {
		ResultType: "UpsertResult",
		ResultFields: []FieldTransform{
			{Source: "metadata.createdRecordIds", Target: "CreatedRecordIDs", Type: "[]int", Dereference: true},
			{Source: "metadata.unchangedRecordIds", Target: "UnchangedRecordIDs", Type: "[]int", Dereference: true},
			{Source: "metadata.updatedRecordIds", Target: "UpdatedRecordIDs", Type: "[]int", Dereference: true},
			{Source: "metadata.totalNumberOfRecordsProcessed", Target: "TotalNumberOfRecordsProcessed", Type: "int", Dereference: true},
		},
	},

	// deleteRecords: Extracts single field with dereference
	"deleteRecords": {
		ResultType: "DeleteRecordsResult",
		ResultFields: []FieldTransform{
			{Source: "numberDeleted", Target: "NumberDeleted", Type: "int", Dereference: true},
		},
	},

	// getFields: Array response with selective field extraction
	"getFields": {
		ResultType:      "FieldDetails",
		IsArrayResponse: true,
		ResultFields: []FieldTransform{
			{Source: "id", Target: "ID", Type: "int", TypeCast: "int"},
			{Source: "label", Target: "Label", Type: "string", Dereference: true},
			{Source: "fieldType", Target: "FieldType", Type: "string", Dereference: true},
		},
	},

	// runQuery: Complex transformation - kept as manual implementation
	// See api.go for the full implementation including:
	// - Bidirectional schema transformation (field aliases)
	// - Where clause parsing and transformation
	// - Record value unwrapping
	// - Pagination helpers (RunQueryAll, RunQueryN)
}

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
