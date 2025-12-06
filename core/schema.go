// Schema resolution and lookup utilities for table and field aliases.
//
// Schema aliases allow developers to use readable names for tables and fields
// instead of cryptic QuickBase IDs. The SDK transforms aliases to IDs in requests
// and IDs back to aliases in responses.
//
// Example usage:
//
//	schema := &core.Schema{
//	    Tables: map[string]core.TableSchema{
//	        "projects": {
//	            ID: "bqw3ryzab",
//	            Fields: map[string]int{
//	                "id":     3,
//	                "name":   6,
//	                "status": 7,
//	            },
//	        },
//	    },
//	}
package core

import (
	"fmt"
	"strings"
)

// Schema defines table and field aliases for a QuickBase application.
// Use NewSchema() builder or define directly as a struct.
//
// Example with builder:
//
//	schema := core.NewSchema().
//	    Table("projects", "bqxyz123").
//	        Field("recordId", 3).
//	        Field("name", 6).
//	        Field("status", 7).
//	    Build()
type Schema struct {
	Tables map[string]TableSchema `json:"tables"`
}

// TableSchema defines a table's ID and field mappings.
type TableSchema struct {
	ID     string         `json:"id"`
	Fields map[string]int `json:"fields"`
}

// SchemaOptions configures schema behavior.
type SchemaOptions struct {
	// TransformResponses controls whether response data keys are converted
	// from field IDs to aliases. Default is true when schema is provided.
	TransformResponses bool
}

// DefaultSchemaOptions returns the default schema options.
func DefaultSchemaOptions() SchemaOptions {
	return SchemaOptions{
		TransformResponses: true,
	}
}

// ResolvedSchema contains precomputed lookup maps for efficient alias resolution.
type ResolvedSchema struct {
	Original *Schema
	Options  SchemaOptions

	// Forward lookups (alias → ID)
	TableAliasToID map[string]string            // table alias → table ID
	FieldAliasToID map[string]map[string]int    // table ID → (field alias → field ID)

	// Reverse lookups (ID → alias)
	TableIDToAlias map[string]string            // table ID → table alias
	FieldIDToAlias map[string]map[int]string    // table ID → (field ID → field alias)
}

// SchemaError is returned when an unknown table or field alias is used.
type SchemaError struct {
	Message string
}

func (e *SchemaError) Error() string {
	return e.Message
}

// ResolveSchema builds lookup maps from a schema definition.
// Returns nil if schema is nil.
func ResolveSchema(schema *Schema) *ResolvedSchema {
	return ResolveSchemaWithOptions(schema, DefaultSchemaOptions())
}

// ResolveSchemaWithOptions builds lookup maps from a schema definition with custom options.
// Returns nil if schema is nil.
func ResolveSchemaWithOptions(schema *Schema, opts SchemaOptions) *ResolvedSchema {
	if schema == nil {
		return nil
	}

	resolved := &ResolvedSchema{
		Original:       schema,
		Options:        opts,
		TableAliasToID: make(map[string]string),
		TableIDToAlias: make(map[string]string),
		FieldAliasToID: make(map[string]map[string]int),
		FieldIDToAlias: make(map[string]map[int]string),
	}

	for tableAlias, tableSchema := range schema.Tables {
		tableID := tableSchema.ID

		// Table mappings
		resolved.TableAliasToID[tableAlias] = tableID
		resolved.TableIDToAlias[tableID] = tableAlias

		// Field mappings for this table
		aliasToID := make(map[string]int)
		idToAlias := make(map[int]string)

		for fieldAlias, fieldID := range tableSchema.Fields {
			aliasToID[fieldAlias] = fieldID
			idToAlias[fieldID] = fieldAlias
		}

		resolved.FieldAliasToID[tableID] = aliasToID
		resolved.FieldIDToAlias[tableID] = idToAlias
	}

	return resolved
}

// ResolveTableAlias resolves a table alias to its ID.
// If the input is already a table ID, returns it unchanged.
// Returns an error if the alias is not found.
func ResolveTableAlias(schema *ResolvedSchema, tableRef string) (string, error) {
	if schema == nil {
		return tableRef, nil
	}

	// Check if it's an alias
	if tableID, ok := schema.TableAliasToID[tableRef]; ok {
		return tableID, nil
	}

	// Check if it's already a table ID
	if _, ok := schema.TableIDToAlias[tableRef]; ok {
		return tableRef, nil
	}

	// Unknown alias - throw helpful error
	available := make([]string, 0, len(schema.TableAliasToID))
	for alias := range schema.TableAliasToID {
		available = append(available, alias)
	}

	suggestion := findSimilar(tableRef, available)
	suggestionText := ""
	if suggestion != "" {
		suggestionText = fmt.Sprintf(" Did you mean '%s'?", suggestion)
	}

	availableText := ""
	if len(available) > 0 {
		availableText = fmt.Sprintf(" Available: %s", strings.Join(available, ", "))
	}

	return "", &SchemaError{
		Message: fmt.Sprintf("unknown table alias '%s'.%s%s", tableRef, suggestionText, availableText),
	}
}

// ResolveFieldAlias resolves a field alias to its ID for a given table.
// If the input is already a field ID (int), returns it unchanged.
// Returns an error if the alias is not found.
func ResolveFieldAlias(schema *ResolvedSchema, tableID string, fieldRef any) (int, error) {
	// If it's already an int, return as-is
	if id, ok := fieldRef.(int); ok {
		return id, nil
	}

	// Must be a string alias
	alias, ok := fieldRef.(string)
	if !ok {
		return 0, &SchemaError{Message: fmt.Sprintf("field reference must be int or string, got %T", fieldRef)}
	}

	// If no schema, try to parse as number or return error
	if schema == nil {
		return 0, &SchemaError{Message: fmt.Sprintf("cannot resolve field alias '%s' without schema", alias)}
	}

	// Look up the alias
	if fieldMap, ok := schema.FieldAliasToID[tableID]; ok {
		if fieldID, ok := fieldMap[alias]; ok {
			return fieldID, nil
		}
	}

	// Unknown alias - throw helpful error
	var available []string
	if fieldMap, ok := schema.FieldAliasToID[tableID]; ok {
		available = make([]string, 0, len(fieldMap))
		for a := range fieldMap {
			available = append(available, a)
		}
	}

	suggestion := findSimilar(alias, available)
	suggestionText := ""
	if suggestion != "" {
		suggestionText = fmt.Sprintf(" Did you mean '%s'?", suggestion)
	}

	// Get table alias for better error message
	tableAlias := tableID
	if ta, ok := schema.TableIDToAlias[tableID]; ok {
		tableAlias = ta
	}

	return 0, &SchemaError{
		Message: fmt.Sprintf("unknown field alias '%s' in table '%s'.%s", alias, tableAlias, suggestionText),
	}
}

// GetFieldAlias returns the alias for a field ID, if defined in schema.
// Returns empty string if no alias is defined.
func GetFieldAlias(schema *ResolvedSchema, tableID string, fieldID int) string {
	if schema == nil {
		return ""
	}
	if fieldMap, ok := schema.FieldIDToAlias[tableID]; ok {
		return fieldMap[fieldID]
	}
	return ""
}

// GetTableAlias returns the alias for a table ID, if defined in schema.
// Returns empty string if no alias is defined.
func GetTableAlias(schema *ResolvedSchema, tableID string) string {
	if schema == nil {
		return ""
	}
	return schema.TableIDToAlias[tableID]
}

// findSimilar finds a similar string from a list (for "did you mean" suggestions).
// Uses Levenshtein distance.
func findSimilar(input string, candidates []string) string {
	const maxDistance = 3
	inputLower := strings.ToLower(input)

	var bestMatch string
	bestDistance := maxDistance + 1

	for _, candidate := range candidates {
		distance := levenshteinDistance(inputLower, strings.ToLower(candidate))
		if distance < bestDistance {
			bestDistance = distance
			bestMatch = candidate
		}
	}

	if bestDistance <= maxDistance {
		return bestMatch
	}
	return ""
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	matrix := make([][]int, len(b)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(a)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill in the rest
	for i := 1; i <= len(b); i++ {
		for j := 1; j <= len(a); j++ {
			if b[i-1] == a[j-1] {
				matrix[i][j] = matrix[i-1][j-1]
			} else {
				matrix[i][j] = min(
					matrix[i-1][j-1]+1, // substitution
					matrix[i][j-1]+1,   // insertion
					matrix[i-1][j]+1,   // deletion
				)
			}
		}
	}

	return matrix[len(b)][len(a)]
}

func min(values ...int) int {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// SchemaBuilder provides a fluent API for building Schema definitions.
type SchemaBuilder struct {
	schema       *Schema
	currentTable string
}

// NewSchema creates a new SchemaBuilder for fluent schema definition.
//
// Example:
//
//	schema := core.NewSchema().
//	    Table("projects", "bqxyz123").
//	        Field("recordId", 3).
//	        Field("name", 6).
//	        Field("status", 7).
//	    Table("tasks", "bqabc456").
//	        Field("recordId", 3).
//	        Field("title", 6).
//	    Build()
func NewSchema() *SchemaBuilder {
	return &SchemaBuilder{
		schema: &Schema{
			Tables: make(map[string]TableSchema),
		},
	}
}

// Table adds a new table to the schema and sets it as the current table
// for subsequent Field() calls.
func (b *SchemaBuilder) Table(alias, tableID string) *SchemaBuilder {
	b.schema.Tables[alias] = TableSchema{
		ID:     tableID,
		Fields: make(map[string]int),
	}
	b.currentTable = alias
	return b
}

// Field adds a field mapping to the current table.
// Must be called after Table().
func (b *SchemaBuilder) Field(alias string, fieldID int) *SchemaBuilder {
	if b.currentTable == "" {
		return b
	}
	table := b.schema.Tables[b.currentTable]
	table.Fields[alias] = fieldID
	b.schema.Tables[b.currentTable] = table
	return b
}

// Build returns the constructed Schema.
func (b *SchemaBuilder) Build() *Schema {
	return b.schema
}
