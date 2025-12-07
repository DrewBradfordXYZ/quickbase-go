package xml

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
)

// SchemaResult contains the response from API_GetSchema.
type SchemaResult struct {
	// TimeZone is the application's time zone (e.g., "(UTC-08:00) Pacific Time (US & Canada)")
	TimeZone string

	// DateFormat is the date format setting (e.g., "MM-DD-YYYY", "YYYY-MM-DD")
	DateFormat string

	// Table contains the schema information
	Table TableSchema

	// schema is used for Field/ChildTable lookups (set internally)
	schema *core.ResolvedSchema

	// tableID is the table ID used for field alias resolution
	tableID string
}

// Field returns a field by alias or ID.
// If a schema was provided to the XML client, aliases are resolved first.
// Returns nil if not found.
//
// Example:
//
//	// With schema
//	result.Field("name").Label
//
//	// By field ID (as string)
//	result.Field("6").Label
func (r *SchemaResult) Field(key string) *SchemaField {
	// Try to resolve alias to ID if schema exists
	if r.schema != nil {
		if fieldID, err := core.ResolveFieldAlias(r.schema, r.tableID, key); err == nil {
			for i := range r.Table.Fields {
				if r.Table.Fields[i].ID == fieldID {
					return &r.Table.Fields[i]
				}
			}
		}
	}

	// Fallback: try parsing as numeric ID
	if id, err := strconv.Atoi(key); err == nil {
		for i := range r.Table.Fields {
			if r.Table.Fields[i].ID == id {
				return &r.Table.Fields[i]
			}
		}
	}

	return nil
}

// FieldByID returns a field by its numeric ID.
// Returns nil if not found.
//
// Example:
//
//	result.FieldByID(6).Label
func (r *SchemaResult) FieldByID(id int) *SchemaField {
	for i := range r.Table.Fields {
		if r.Table.Fields[i].ID == id {
			return &r.Table.Fields[i]
		}
	}
	return nil
}

// ChildTable returns a child table by alias or DBID.
// If a schema was provided to the XML client, aliases are resolved first.
// Returns nil if not found.
//
// Example:
//
//	// With schema
//	result.ChildTable("tasks").DBID
//
//	// By DBID
//	result.ChildTable("bqxyz123").DBID
func (r *SchemaResult) ChildTable(key string) *ChildTable {
	// Try to resolve alias to ID if schema exists
	if r.schema != nil {
		if tableID, err := core.ResolveTableAlias(r.schema, key); err == nil {
			for i := range r.Table.ChildTables {
				if r.Table.ChildTables[i].DBID == tableID {
					return &r.Table.ChildTables[i]
				}
			}
		}
	}

	// Fallback: try direct DBID match
	for i := range r.Table.ChildTables {
		if r.Table.ChildTables[i].DBID == key {
			return &r.Table.ChildTables[i]
		}
	}

	return nil
}

// TableSchema contains schema information for an app or table.
type TableSchema struct {
	// Name is the table/app name
	Name string `xml:"name"`

	// Description is the table/app description
	Description string `xml:"desc"`

	// Original contains metadata about the table
	Original TableOriginal `xml:"original"`

	// Variables contains app-level variables (DBVars)
	Variables []Variable `xml:"variables>var"`

	// ChildTables contains child table dbids (only for app-level schema)
	ChildTables []ChildTable `xml:"chdbids>chdbid"`

	// Queries contains saved reports/queries
	Queries []Query `xml:"queries>query"`

	// Fields contains field definitions (only for table-level schema)
	Fields []SchemaField `xml:"fields>field"`
}

// TableOriginal contains metadata about a table's creation and state.
type TableOriginal struct {
	// AppID is the parent application's dbid
	AppID string `xml:"app_id"`

	// TableID is this table's dbid
	TableID string `xml:"table_id"`

	// CreatedDate is when the table was created (milliseconds since epoch)
	CreatedDate string `xml:"cre_date"`

	// ModifiedDate is when the table was last modified (milliseconds since epoch)
	ModifiedDate string `xml:"mod_date"`

	// NextRecordID is the ID that will be assigned to the next record
	NextRecordID int `xml:"next_record_id"`

	// NextFieldID is the ID that will be assigned to the next field
	NextFieldID int `xml:"next_field_id"`

	// NextQueryID is the ID that will be assigned to the next query
	NextQueryID int `xml:"next_query_id"`

	// DefaultSortFieldID is the default sort field
	DefaultSortFieldID int `xml:"def_sort_fid"`

	// DefaultSortOrder is the default sort order (1=ascending, -1=descending)
	DefaultSortOrder int `xml:"def_sort_order"`
}

// Variable represents a DBVar (database variable).
type Variable struct {
	// Name is the variable name
	Name string `xml:"name,attr"`

	// Value is the variable value
	Value string `xml:",chardata"`
}

// ChildTable represents a child table in an application.
type ChildTable struct {
	// Name is the internal name (e.g., "_dbid_my_table")
	Name string `xml:"name,attr"`

	// DBID is the table's database ID
	DBID string `xml:",chardata"`
}

// Query represents a saved report/query.
type Query struct {
	// ID is the query ID
	ID int `xml:"id,attr"`

	// Name is the query name
	Name string `xml:"qyname"`

	// Type is the query type (e.g., "table")
	Type string `xml:"qytype"`

	// Description is the query description
	Description string `xml:"qydesc"`

	// Criteria is the query criteria/filter
	Criteria string `xml:"qycrit"`

	// ColumnList is the list of columns to display
	ColumnList string `xml:"qyclst"`

	// Options contains query options
	Options string `xml:"qyopts"`

	// SortList is the sort specification
	SortList string `xml:"qyslst"`

	// CalendarStartFieldList is for calendar views
	CalendarStartFieldList string `xml:"qycalst"`
}

// SchemaField represents a field definition from the schema.
type SchemaField struct {
	// ID is the field ID
	ID int `xml:"id,attr"`

	// FieldType is the field type (e.g., "text", "numeric", "checkbox")
	FieldType string `xml:"field_type,attr"`

	// BaseType is the underlying storage type (e.g., "text", "int64", "bool")
	BaseType string `xml:"base_type,attr"`

	// Label is the field label/name
	Label string `xml:"label"`

	// FieldHelp is the help text shown to users
	FieldHelp string `xml:"fieldhelp"`

	// NoWrap indicates if text wrapping is disabled
	NoWrap int `xml:"nowrap"`

	// Bold indicates if the field is displayed in bold
	Bold int `xml:"bold"`

	// Required indicates if the field is required
	Required int `xml:"required"`

	// Unique indicates if values must be unique
	Unique int `xml:"unique"`

	// DoesDataCopy indicates if data copies to related records
	DoesDataCopy int `xml:"does_data_copy"`

	// Mode indicates the field mode
	Mode string `xml:"mode"`

	// DefaultValue is the default value for new records
	DefaultValue string `xml:"default_value"`

	// Formula is the formula (for formula fields)
	Formula string `xml:"formula"`

	// Choices contains the choices for multiple-choice fields
	Choices []string `xml:"choices>choice"`

	// SummaryFunction is for summary fields (e.g., "Total", "Average")
	SummaryFunction string `xml:"summaryFunction"`

	// SummaryTargetFieldID is the field being summarized
	SummaryTargetFieldID int `xml:"summaryTargetFid"`

	// SummaryReferenceFieldID is the relationship field
	SummaryReferenceFieldID int `xml:"summaryReferenceFid"`
}

// getSchemaResponse is the XML response structure for API_GetSchema.
type getSchemaResponse struct {
	BaseResponse
	TimeZone   string      `xml:"time_zone"`
	DateFormat string      `xml:"date_format"`
	Table      TableSchema `xml:"table"`
}

// GetSchema returns comprehensive schema information for an app or table.
//
// When called with an application dbid, returns app-level information including
// child table dbids and app variables.
//
// When called with a table dbid, returns table-level information including
// field definitions, saved queries/reports, and table variables.
//
// If a schema was configured with [WithSchema], table aliases can be used
// as input and result helper methods become available:
//
//	// Use table alias instead of DBID
//	result, _ := xmlClient.GetSchema(ctx, "projects")
//
//	// Access fields by alias
//	result.Field("name").Label
//
// Example (app-level):
//
//	schema, err := xmlClient.GetSchema(ctx, appId)
//	fmt.Printf("App: %s\n", schema.Table.Name)
//	for _, child := range schema.Table.ChildTables {
//	    fmt.Printf("  Table %s: %s\n", child.Name, child.DBID)
//	}
//
// Example (table-level):
//
//	schema, err := xmlClient.GetSchema(ctx, tableId)
//	fmt.Printf("Table: %s\n", schema.Table.Name)
//	for _, field := range schema.Table.Fields {
//	    fmt.Printf("  Field %d: %s (%s)\n", field.ID, field.Label, field.FieldType)
//	}
//
// See: https://help.quickbase.com/docs/api-getschema
func (c *Client) GetSchema(ctx context.Context, dbid string) (*SchemaResult, error) {
	// Resolve table alias if schema is configured
	resolvedID := c.resolveTable(dbid)

	body := buildRequest("")

	respBody, err := c.caller.DoXML(ctx, resolvedID, "API_GetSchema", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetSchema: %w", err)
	}

	var resp getSchemaResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetSchema response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &SchemaResult{
		TimeZone:   resp.TimeZone,
		DateFormat: resp.DateFormat,
		Table:      resp.Table,
		schema:     c.schema,
		tableID:    resolvedID,
	}, nil
}
