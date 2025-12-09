package main

// AliasConfig defines which parameters should support alias resolution
type AliasConfig struct {
	// TableParams are parameter names that reference table IDs and should support table alias resolution
	TableParams []string

	// FieldParams are parameter names that reference field IDs and should support field alias resolution
	FieldParams []string

	// FieldArrayParams are parameter names that are arrays of field IDs
	FieldArrayParams []string

	// FieldStructParams are struct field paths that contain field IDs (e.g., "sortBy[].fieldId")
	FieldStructParams []string
}

// aliasRules defines the alias resolution rules for the builder generator.
// This is the only manual configuration needed - everything else is auto-generated from the spec.
var aliasRules = AliasConfig{
	// Parameters that reference tables (resolved via schema.Tables)
	TableParams: []string{
		"from",          // runQuery
		"to",            // upsert
		"tableId",       // various operations
		"childTableId",  // relationships
		"parentTableId", // relationships
	},

	// Parameters that reference a single field ID
	FieldParams: []string{
		"mergeFieldId", // upsert
	},

	// Parameters that are arrays of field IDs
	FieldArrayParams: []string{
		"select",         // runQuery
		"fieldsToReturn", // upsert
	},

	// Struct fields that contain field IDs
	FieldStructParams: []string{
		"sortBy[].fieldId",  // runQuery
		"groupBy[].fieldId", // runQuery
	},
}

// ConstructorConfig defines which parameters go in the constructor vs chainable methods
type ConstructorConfig struct {
	// OperationID to constructor params mapping
	// If not specified, defaults are inferred from path params
	Overrides map[string][]string
}

// constructorRules defines constructor parameter overrides.
// By default, path params become constructor params.
// This config allows customization for operations that need different behavior.
var constructorRules = ConstructorConfig{
	Overrides: map[string][]string{
		// createApp has no path params, but we don't need a constructor param
		// since name is a required body field (validated at runtime)
		"createApp": {},

		// runQuery, upsert, deleteRecords use "from"/"to" as the table identifier
		// These are body params but act as the primary identifier
		"runQuery":      {"from"},
		"upsert":        {"to"},
		"deleteRecords": {"from"},

		// Operations with tableId as query param need it in constructor
		// No path params - just tableId
		"getTableReports":          {"tableId"},
		"getFields":                {"tableId"},
		"createField":              {"tableId"},
		"deleteFields":             {"tableId"},
		"getFieldsUsage":           {"tableId"},
		"createSolutionFromRecord": {"tableId"},

		// Has path param + tableId query param
		"getReport":                   {"reportId", "tableId"},
		"runReport":                   {"reportId", "tableId"},
		"getField":                    {"fieldId", "tableId"},
		"updateField":                 {"fieldId", "tableId"},
		"getFieldUsage":               {"fieldId", "tableId"},
		"exportSolutionToRecord":      {"solutionId", "tableId"},
		"updateSolutionToRecord":      {"solutionId", "tableId"},
		"changesetSolutionFromRecord": {"solutionId", "tableId"},
		"generateDocument":            {"templateId", "tableId"},

		// createTable has appId as query param but not primary identifier
		"createTable": {},
	},
}

// paginatedOperations lists operations that support pagination
var paginatedOperations = map[string]bool{
	"runQuery":  true,
	"runReport": true,
}

// manualImplementations lists operations that have fully manual implementations in api.go
// These operations are completely skipped in code generation.
//
// Most operations are now auto-generated with response transformations defined in
// transforms.go. Only runQuery remains fully manual due to its complex bidirectional
// schema transformation requirements.
var manualImplementations = map[string]bool{
	"runQuery": true, // Has schema transformation, record transformation, pagination
}

// manualResultTypes maps operation IDs to their manual result type names in api.go.
// These operations have hand-written result wrappers that provide helper methods
// beyond what the generator can produce. The builder will return the wrapper type
// instead of the raw generated response.
//
// Note: runReport is in manualImplementations (fully hand-written builder),
// so it doesn't need to be here.
var manualResultTypes = map[string]string{
	"getFields":        "GetFieldsResult",
	"getFieldUsage":    "GetFieldUsageResult",
	"getFieldsUsage":   "GetFieldsUsageResult",
	"getUsers":         "GetUsersResult",
	"getRelationships": "GetRelationshipsResult",
}

// getManualResultType returns the manual result type name for an operation, if any.
func getManualResultType(opID string) (string, bool) {
	t, ok := manualResultTypes[opID]
	return t, ok
}

// shouldSkipResultType returns true if the operation should not generate a result type
// (because it has a manual result type instead)
func shouldSkipResultType(opID string) bool {
	_, ok := manualResultTypes[opID]
	return ok
}

// hasManualImplementation returns true if the operation has a manual implementation
func hasManualImplementation(opID string) bool {
	return manualImplementations[opID]
}

// isTableParam checks if a parameter name should support table alias resolution
func isTableParam(name string) bool {
	for _, p := range aliasRules.TableParams {
		if p == name {
			return true
		}
	}
	return false
}

// isFieldParam checks if a parameter name should support single field alias resolution
func isFieldParam(name string) bool {
	for _, p := range aliasRules.FieldParams {
		if p == name {
			return true
		}
	}
	return false
}

// isFieldArrayParam checks if a parameter name should support field array alias resolution
func isFieldArrayParam(name string) bool {
	for _, p := range aliasRules.FieldArrayParams {
		if p == name {
			return true
		}
	}
	return false
}

// isFieldStructParam checks if a struct field path should support field alias resolution
func isFieldStructParam(path string) bool {
	for _, p := range aliasRules.FieldStructParams {
		if p == path {
			return true
		}
	}
	return false
}

// getConstructorParams returns the constructor parameters for an operation
func getConstructorParams(operationID string, pathParams []string) []string {
	if override, ok := constructorRules.Overrides[operationID]; ok {
		return override
	}
	// Default: use path params as constructor params
	return pathParams
}
