package client

import (
	"context"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/generated"
)

// QueryBuilder provides a fluent API for building and executing RunQuery requests.
// It simplifies query construction by eliminating repetitive table parameter passing
// and providing schema-aware field resolution.
//
// Example:
//
//	result, err := client.Query("projects").
//	    Select("name", "status", "dueDate").
//	    Where("{'status'.EX.'Active'}").
//	    SortBy(quickbase.Asc("name"), quickbase.Desc("dueDate")).
//	    Options(100, 0).
//	    Run(ctx)
type QueryBuilder struct {
	client  *Client
	table   string   // Original table (alias or ID)
	tableID string   // Resolved table ID
	fields  []any    // Field IDs or aliases
	where   string
	sortBy  []SortSpec
	groupBy []any // Field IDs or aliases
	top     *int
	skip    *int
	err     error
}

// Query starts building a query for the specified table.
// The table parameter can be a table ID or an alias (if schema is configured).
//
// Example:
//
//	client.Query("projects")        // Table alias (requires schema)
//	client.Query("bqxyz123")        // Table ID (works without schema)
func (c *Client) Query(table string) *QueryBuilder {
	b := &QueryBuilder{
		client: c,
		table:  table,
	}

	if c.schema != nil {
		tableID, err := core.ResolveTableAlias(c.schema, table)
		if err != nil {
			b.err = err
			return b
		}
		b.tableID = tableID
	} else {
		b.tableID = table
	}

	return b
}

// Select specifies the fields to return. Accepts field IDs (int) or aliases (string).
// When schema is configured, string aliases are resolved to field IDs.
//
// Example:
//
//	Select("name", "status")        // Field aliases (requires schema)
//	Select(6, 7)                    // Field IDs (works without schema)
//	Select("name", 6, "status")     // Mixed (aliases require schema)
func (b *QueryBuilder) Select(fields ...any) *QueryBuilder {
	if b.err != nil {
		return b
	}
	b.fields = fields
	return b
}

// Where sets the filter query string. Field aliases in the where clause are
// automatically resolved if schema is configured.
//
// Example:
//
//	Where("{'status'.EX.'Active'}")           // Using field alias
//	Where("{7.EX.'Active'}")                  // Using field ID
//	Where("{status.EX.'Active'} AND {dueDate.LT.'2024-01-01'}")
func (b *QueryBuilder) Where(query string) *QueryBuilder {
	if b.err != nil {
		return b
	}
	b.where = query
	return b
}

// SortBy sets the sort order for results. Use Asc() and Desc() helpers.
//
// Example:
//
//	SortBy(quickbase.Asc("name"))
//	SortBy(quickbase.Asc("name"), quickbase.Desc("dueDate"))
//	SortBy(quickbase.Asc(6), quickbase.Desc(7))
func (b *QueryBuilder) SortBy(sorts ...SortSpec) *QueryBuilder {
	if b.err != nil {
		return b
	}
	b.sortBy = sorts
	return b
}

// GroupBy sets the fields to group by. Accepts field IDs (int) or aliases (string).
//
// Example:
//
//	GroupBy("status")               // Field alias
//	GroupBy(7)                      // Field ID
//	GroupBy("status", "assignee")   // Multiple fields
func (b *QueryBuilder) GroupBy(fields ...any) *QueryBuilder {
	if b.err != nil {
		return b
	}
	b.groupBy = fields
	return b
}

// Options sets pagination options: top (max records) and skip (offset).
//
// Example:
//
//	Options(100, 0)     // First 100 records
//	Options(50, 100)    // Records 101-150
func (b *QueryBuilder) Options(top, skip int) *QueryBuilder {
	if b.err != nil {
		return b
	}
	b.top = &top
	b.skip = &skip
	return b
}

// Top sets the maximum number of records to return.
func (b *QueryBuilder) Top(n int) *QueryBuilder {
	if b.err != nil {
		return b
	}
	b.top = &n
	return b
}

// Skip sets the number of records to skip (offset).
func (b *QueryBuilder) Skip(n int) *QueryBuilder {
	if b.err != nil {
		return b
	}
	b.skip = &n
	return b
}

// buildBody constructs the RunQueryJSONRequestBody from builder state.
func (b *QueryBuilder) buildBody() (generated.RunQueryJSONRequestBody, error) {
	body := generated.RunQueryJSONRequestBody{
		From: b.tableID,
	}

	// Resolve select fields
	if len(b.fields) > 0 {
		fieldIDs, err := b.resolveFieldIDs(b.fields)
		if err != nil {
			return body, err
		}
		body.Select = &fieldIDs
	}

	// Set where clause (transformation happens in RunQuery)
	if b.where != "" {
		whereUnion, err := StringToWhereUnion(b.where)
		if err != nil {
			return body, err
		}
		body.Where = whereUnion
	}

	// Resolve and set sortBy
	if len(b.sortBy) > 0 {
		sortFields, err := b.resolveSortFields()
		if err != nil {
			return body, err
		}
		// Create SortBy union from sort fields
		sortByUnion, err := SortFieldsToSortByUnion(sortFields)
		if err != nil {
			return body, err
		}
		body.SortBy = sortByUnion
	}

	// Resolve groupBy
	if len(b.groupBy) > 0 {
		groupByIDs, err := b.resolveFieldIDs(b.groupBy)
		if err != nil {
			return body, err
		}
		// GroupBy expects []generated.RunQueryJSONBody_GroupBy_Item
		groupByFields := make([]generated.RunQueryJSONBody_GroupBy_Item, len(groupByIDs))
		for i, id := range groupByIDs {
			fieldID := id
			groupByFields[i] = generated.RunQueryJSONBody_GroupBy_Item{FieldId: &fieldID}
		}
		body.GroupBy = &groupByFields
	}

	// Set pagination options
	if b.top != nil || b.skip != nil {
		opts := &generated.RunQueryJSONBody_Options{}
		if b.top != nil {
			opts.Top = b.top
		}
		if b.skip != nil {
			opts.Skip = b.skip
		}
		body.Options = opts
	}

	return body, nil
}

// resolveFieldIDs converts a slice of field references (int IDs or string aliases)
// to a slice of int field IDs.
func (b *QueryBuilder) resolveFieldIDs(fields []any) ([]int, error) {
	ids := make([]int, 0, len(fields))

	for _, field := range fields {
		switch f := field.(type) {
		case int:
			ids = append(ids, f)
		case string:
			if b.client.schema != nil {
				// Use tableID (resolved) for field lookup, not table (alias)
				fieldID, err := core.ResolveFieldAlias(b.client.schema, b.tableID, f)
				if err != nil {
					return nil, err
				}
				ids = append(ids, fieldID)
			} else {
				return nil, &core.SchemaError{
					Message: "schema required to resolve field alias: " + f,
				}
			}
		default:
			return nil, &core.ValidationError{
				QuickbaseError: core.QuickbaseError{
					Message: "field must be int (ID) or string (alias)",
				},
			}
		}
	}

	return ids, nil
}

// resolveSortFields converts SortSpec slice to generated.SortField slice.
func (b *QueryBuilder) resolveSortFields() ([]generated.SortField, error) {
	sortFields := make([]generated.SortField, len(b.sortBy))

	for i, spec := range b.sortBy {
		var fieldID int

		switch f := spec.Field.(type) {
		case int:
			fieldID = f
		case string:
			if b.client.schema != nil {
				// Use tableID (resolved) for field lookup, not table (alias)
				resolved, err := core.ResolveFieldAlias(b.client.schema, b.tableID, f)
				if err != nil {
					return nil, err
				}
				fieldID = resolved
			} else {
				return nil, &core.SchemaError{
					Message: "schema required to resolve field alias: " + f,
				}
			}
		default:
			return nil, &core.ValidationError{
				QuickbaseError: core.QuickbaseError{
					Message: "sort field must be int (ID) or string (alias)",
				},
			}
		}

		sortFields[i] = generated.SortField{
			FieldId: fieldID,
			Order:   spec.Order,
		}
	}

	return sortFields, nil
}

// Run executes the query and returns all records as unwrapped maps.
// This is a convenience method that auto-unwraps records since you're opting
// into the Query builder's fluent API. For raw generated types, use RunRaw().
func (b *QueryBuilder) Run(ctx context.Context) ([]Record, error) {
	if b.err != nil {
		return nil, b.err
	}

	body, err := b.buildBody()
	if err != nil {
		return nil, err
	}

	records, err := b.client.RunQueryAll(ctx, body)
	if err != nil {
		return nil, err
	}

	return unwrapRecords(records), nil
}

// RunRaw executes the query and returns the result with convenience methods.
// Use this when you need access to the full response including metadata.
func (b *QueryBuilder) RunRaw(ctx context.Context) (*RunQueryResult, error) {
	if b.err != nil {
		return nil, b.err
	}

	body, err := b.buildBody()
	if err != nil {
		return nil, err
	}

	return b.client.RunQuery(ctx, body)
}

// RunN executes the query and returns up to n records as unwrapped maps.
// This is a convenience method that auto-unwraps records.
func (b *QueryBuilder) RunN(ctx context.Context, n int) ([]Record, error) {
	if b.err != nil {
		return nil, b.err
	}

	body, err := b.buildBody()
	if err != nil {
		return nil, err
	}

	records, err := b.client.RunQueryN(ctx, body, n)
	if err != nil {
		return nil, err
	}

	return unwrapRecords(records), nil
}
