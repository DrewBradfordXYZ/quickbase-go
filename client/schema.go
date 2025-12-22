package client

import (
	"github.com/DrewBradfordXYZ/quickbase-go/core"
)

// ErrNoSchema is returned when a schema operation is attempted without a configured schema.
var ErrNoSchema = &core.SchemaError{Message: "no schema configured; use WithSchema() option"}

// Table resolves a table alias to its ID.
// Returns the alias as-is if it's already an ID (not found in schema).
//
// Example:
//
//	tableID, err := client.Table("projects")  // Returns "bqxyz123"
//	tableID, err := client.Table("bqxyz123") // Returns "bqxyz123" (passthrough)
func (c *Client) Table(alias string) (string, error) {
	if c.schema == nil {
		return alias, nil // No schema, passthrough
	}
	return core.ResolveTableAlias(c.schema, alias)
}

// Fields resolves field names to IDs using the configured schema.
// The table can be an alias or ID. Returns an error if schema is not configured.
//
// Example:
//
//	ids, err := client.Fields("projects", "name", "status", "dueDate")
//	// Returns []int{6, 7, 12}
//
// Use this to build the Select parameter:
//
//	fieldIDs, _ := client.Fields("projects", "name", "status")
//	result, err := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:   "bqxyz123",
//	    Select: &fieldIDs,
//	})
func (c *Client) Fields(table string, names ...string) ([]int, error) {
	if c.schema == nil {
		return nil, ErrNoSchema
	}

	// First resolve the table alias to get the actual table ID
	tableID, err := core.ResolveTableAlias(c.schema, table)
	if err != nil {
		return nil, err
	}

	ids := make([]int, len(names))
	for i, name := range names {
		fieldID, err := core.ResolveFieldAlias(c.schema, tableID, name)
		if err != nil {
			return nil, err
		}
		ids[i] = fieldID
	}

	return ids, nil
}

// Field resolves a single field name to its ID.
// The table can be an alias or ID. Returns an error if schema is not configured.
//
// Example:
//
//	id, err := client.Field("projects", "name")  // Returns 6
func (c *Client) Field(table string, name string) (int, error) {
	if c.schema == nil {
		return 0, ErrNoSchema
	}

	// First resolve the table alias to get the actual table ID
	tableID, err := core.ResolveTableAlias(c.schema, table)
	if err != nil {
		return 0, err
	}

	return core.ResolveFieldAlias(c.schema, tableID, name)
}

// HasSchema returns true if a schema is configured for this client.
func (c *Client) HasSchema() bool {
	return c.schema != nil
}
