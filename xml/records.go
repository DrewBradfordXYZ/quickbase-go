package xml

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
)

// DoQueryCountResult contains the response from API_DoQueryCount.
type DoQueryCountResult struct {
	// NumMatches is the number of records matching the query
	NumMatches int
}

// doQueryCountResponse is the XML response structure for API_DoQueryCount.
type doQueryCountResponse struct {
	BaseResponse
	NumMatches int `xml:"numMatches"`
}

// DoQueryCount returns the count of records matching a query without fetching data.
//
// This is more efficient than running a full query when you only need to know
// how many records match. The query parameter uses the same syntax as API_DoQuery.
//
// If a schema was configured with [WithSchema], table aliases can be used.
//
// Example:
//
//	// Count all records
//	result, err := xmlClient.DoQueryCount(ctx, tableId, "")
//	fmt.Printf("Total records: %d\n", result.NumMatches)
//
//	// Count records matching a filter
//	result, err := xmlClient.DoQueryCount(ctx, tableId, "{'7'.EX.'Active'}")
//	fmt.Printf("Active records: %d\n", result.NumMatches)
//
// See: https://help.quickbase.com/docs/api-doquerycount
func (c *Client) DoQueryCount(ctx context.Context, tableId, query string) (*DoQueryCountResult, error) {
	resolvedID := c.resolveTable(tableId)
	inner := ""
	if query != "" {
		inner = "<query>" + query + "</query>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, resolvedID, "API_DoQueryCount", body)
	if err != nil {
		return nil, fmt.Errorf("API_DoQueryCount: %w", err)
	}

	var resp doQueryCountResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_DoQueryCount response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &DoQueryCountResult{
		NumMatches: resp.NumMatches,
	}, nil
}

// RecordField represents a field value with its metadata from API_GetRecordInfo.
type RecordField struct {
	// ID is the field ID
	ID int

	// Name is the field label/name
	Name string

	// Type is the field type (e.g., "Text", "Date", "File Attachment")
	Type string

	// Value is the raw field value
	Value string

	// Printable is the human-readable formatted value (e.g., formatted dates)
	// This may be empty for some field types.
	Printable string
}

// GetRecordInfoResult contains the response from API_GetRecordInfo.
type GetRecordInfoResult struct {
	// RecordID is the record's unique ID
	RecordID int

	// NumFields is the total number of fields in the record
	NumFields int

	// UpdateID is used for optimistic concurrency control
	UpdateID string

	// Fields contains all field values with their metadata
	Fields []RecordField

	// schema and tableID are used for Field() lookups (set internally)
	schema  *core.ResolvedSchema
	tableID string
}

// Field returns a field by alias or ID string.
// If a schema was provided to the XML client, aliases are resolved first.
// Returns nil if not found.
//
// Example:
//
//	// With schema
//	result.Field("name").Value   // Access by alias
//
//	// Without schema or for unknown fields
//	result.Field("6").Value      // Access by ID string
func (r *GetRecordInfoResult) Field(key string) *RecordField {
	// Try to resolve alias to ID if schema exists
	if r.schema != nil {
		if fieldID, err := core.ResolveFieldAlias(r.schema, r.tableID, key); err == nil {
			for i := range r.Fields {
				if r.Fields[i].ID == fieldID {
					return &r.Fields[i]
				}
			}
		}
	}

	// Fallback: try parsing as int
	if id, err := strconv.Atoi(key); err == nil {
		for i := range r.Fields {
			if r.Fields[i].ID == id {
				return &r.Fields[i]
			}
		}
	}

	return nil
}

// FieldByID returns a field by its numeric ID.
// Returns nil if not found.
func (r *GetRecordInfoResult) FieldByID(id int) *RecordField {
	for i := range r.Fields {
		if r.Fields[i].ID == id {
			return &r.Fields[i]
		}
	}
	return nil
}

// recordFieldXML is the XML structure for a field in API_GetRecordInfo response.
type recordFieldXML struct {
	FID       int    `xml:"fid"`
	Name      string `xml:"name"`
	Type      string `xml:"type"`
	Value     string `xml:"value"`
	Printable string `xml:"printable"`
}

// getRecordInfoResponse is the XML response structure for API_GetRecordInfo.
type getRecordInfoResponse struct {
	BaseResponse
	RID       int              `xml:"rid"`
	NumFields int              `xml:"num_fields"`
	UpdateID  string           `xml:"update_id"`
	Fields    []recordFieldXML `xml:"field"`
}

// GetRecordInfo returns a single record with full field metadata.
//
// Unlike the JSON API which only returns field values by ID, this returns
// each field's name, type, value, and human-readable printable format.
// This is useful for building generic record viewers or debugging.
//
// The tableId can be a table alias (if schema is configured) or a raw table ID.
// The recordId can be either the record ID (rid) or a value from a custom key field.
//
// Example:
//
//	result, err := xmlClient.GetRecordInfo(ctx, tableId, 123)
//	fmt.Printf("Record %d has %d fields\n", result.RecordID, result.NumFields)
//	for _, field := range result.Fields {
//	    fmt.Printf("  %s (%s): %s\n", field.Name, field.Type, field.Value)
//	    if field.Printable != "" {
//	        fmt.Printf("    Formatted: %s\n", field.Printable)
//	    }
//	}
//
//	// With schema - access fields by alias:
//	result.Field("name").Value
//
// See: https://help.quickbase.com/docs/api-getrecordinfo
func (c *Client) GetRecordInfo(ctx context.Context, tableId string, recordId int) (*GetRecordInfoResult, error) {
	resolvedTableId := c.resolveTable(tableId)

	inner := "<rid>" + strconv.Itoa(recordId) + "</rid>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, resolvedTableId, "API_GetRecordInfo", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetRecordInfo: %w", err)
	}

	var resp getRecordInfoResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetRecordInfo response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	// Convert XML fields to result fields
	fields := make([]RecordField, len(resp.Fields))
	for i, f := range resp.Fields {
		fields[i] = RecordField{
			ID:        f.FID,
			Name:      f.Name,
			Type:      f.Type,
			Value:     f.Value,
			Printable: f.Printable,
		}
	}

	return &GetRecordInfoResult{
		RecordID:  resp.RID,
		NumFields: resp.NumFields,
		UpdateID:  resp.UpdateID,
		Fields:    fields,
		schema:    c.schema,
		tableID:   resolvedTableId,
	}, nil
}

// GetRecordInfoByKey returns a single record using a custom key field value.
//
// This is similar to GetRecordInfo but uses a key field value instead of record ID.
// The tableId can be a table alias (if schema is configured) or a raw table ID.
// The keyValue should be the value from the table's designated key field.
//
// Example:
//
//	// If "Order Number" (field 6) is the key field
//	result, err := xmlClient.GetRecordInfoByKey(ctx, tableId, "ORD-12345")
//
//	// With schema - access fields by alias:
//	result.Field("name").Value
//
// See: https://help.quickbase.com/docs/api-getrecordinfo
func (c *Client) GetRecordInfoByKey(ctx context.Context, tableId string, keyValue string) (*GetRecordInfoResult, error) {
	resolvedTableId := c.resolveTable(tableId)

	inner := "<key>" + keyValue + "</key>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, resolvedTableId, "API_GetRecordInfo", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetRecordInfo: %w", err)
	}

	var resp getRecordInfoResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetRecordInfo response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	// Convert XML fields to result fields
	fields := make([]RecordField, len(resp.Fields))
	for i, f := range resp.Fields {
		fields[i] = RecordField{
			ID:        f.FID,
			Name:      f.Name,
			Type:      f.Type,
			Value:     f.Value,
			Printable: f.Printable,
		}
	}

	return &GetRecordInfoResult{
		RecordID:  resp.RID,
		NumFields: resp.NumFields,
		UpdateID:  resp.UpdateID,
		Fields:    fields,
		schema:    c.schema,
		tableID:   resolvedTableId,
	}, nil
}

// ImportFromCSVOptions configures the CSV import operation.
type ImportFromCSVOptions struct {
	// RecordsCSV is the CSV data to import (required).
	// Use CDATA wrapper for complex data.
	RecordsCSV string

	// CList is a period-delimited list of field IDs mapping CSV columns to fields.
	// Use 0 to skip a column. Required when updating records or mapping specific fields.
	// Example: "6.7.8" maps columns 1-3 to fields 6, 7, 8.
	CList string

	// CListOutput specifies which fields to return in the response.
	// Period-delimited field IDs.
	CListOutput string

	// SkipFirst skips the first row (header row) if true.
	SkipFirst bool

	// DecimalPercent when true interprets 0.50 as 50% instead of 0.50%.
	DecimalPercent bool

	// MsInUTC when true interprets date/times as UTC milliseconds.
	MsInUTC bool

	// MergeFieldId uses a different field as the merge key instead of the table key.
	// The field must be unique.
	MergeFieldId int
}

// ImportFromCSVRecord represents a record that was added or updated.
type ImportFromCSVRecord struct {
	// RecordID is the record's unique ID
	RecordID int

	// UpdateID is used for optimistic concurrency control in subsequent edits
	UpdateID string
}

// ImportFromCSVResult contains the response from API_ImportFromCSV.
type ImportFromCSVResult struct {
	// NumRecsInput is the total number of records in the CSV
	NumRecsInput int

	// NumRecsAdded is the number of new records created
	NumRecsAdded int

	// NumRecsUpdated is the number of existing records updated
	NumRecsUpdated int

	// Records contains the record IDs and update IDs for all affected records
	Records []ImportFromCSVRecord
}

// ridXML is the XML structure for a record ID in import responses.
type ridXML struct {
	RID      int    `xml:",chardata"`
	UpdateID string `xml:"update_id,attr"`
}

// importFromCSVResponse is the XML response structure for API_ImportFromCSV.
type importFromCSVResponse struct {
	BaseResponse
	NumRecsInput   int      `xml:"num_recs_input"`
	NumRecsAdded   int      `xml:"num_recs_added"`
	NumRecsUpdated int      `xml:"num_recs_updated"`
	RIDs           []ridXML `xml:"rids>rid"`
}

// ImportFromCSV adds or updates multiple records from CSV data.
//
// You can add new records and update existing records in the same request.
// For adds, leave the record ID column empty. For updates, include the
// key field (usually field 3, Record ID#) in the clist and CSV data.
//
// Example - Add new records:
//
//	result, err := xmlClient.ImportFromCSV(ctx, tableId, xml.ImportFromCSVOptions{
//	    RecordsCSV: "Name,Status\nJohn,Active\nJane,Pending",
//	    CList:      "6.7",
//	    SkipFirst:  true,
//	})
//	fmt.Printf("Added %d records\n", result.NumRecsAdded)
//
// Example - Update existing records:
//
//	result, err := xmlClient.ImportFromCSV(ctx, tableId, xml.ImportFromCSVOptions{
//	    RecordsCSV: "Record ID,Status\n1,Complete\n2,Active",
//	    CList:      "3.7",
//	    SkipFirst:  true,
//	})
//	fmt.Printf("Updated %d records\n", result.NumRecsUpdated)
//
// See: https://help.quickbase.com/docs/api-importfromcsv
func (c *Client) ImportFromCSV(ctx context.Context, tableId string, opts ImportFromCSVOptions) (*ImportFromCSVResult, error) {
	inner := "<records_csv><![CDATA[" + opts.RecordsCSV + "]]></records_csv>"

	if opts.CList != "" {
		inner += "<clist>" + opts.CList + "</clist>"
	}
	if opts.CListOutput != "" {
		inner += "<clist_output>" + opts.CListOutput + "</clist_output>"
	}
	if opts.SkipFirst {
		inner += "<skipfirst>1</skipfirst>"
	}
	if opts.DecimalPercent {
		inner += "<decimalPercent>1</decimalPercent>"
	}
	if opts.MsInUTC {
		inner += "<msInUTC>1</msInUTC>"
	}
	if opts.MergeFieldId > 0 {
		inner += "<mergeFieldId>" + strconv.Itoa(opts.MergeFieldId) + "</mergeFieldId>"
	}

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_ImportFromCSV", body)
	if err != nil {
		return nil, fmt.Errorf("API_ImportFromCSV: %w", err)
	}

	var resp importFromCSVResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_ImportFromCSV response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	records := make([]ImportFromCSVRecord, len(resp.RIDs))
	for i, rid := range resp.RIDs {
		records[i] = ImportFromCSVRecord{
			RecordID: rid.RID,
			UpdateID: rid.UpdateID,
		}
	}

	return &ImportFromCSVResult{
		NumRecsInput:   resp.NumRecsInput,
		NumRecsAdded:   resp.NumRecsAdded,
		NumRecsUpdated: resp.NumRecsUpdated,
		Records:        records,
	}, nil
}

// RunImportResult contains the response from API_RunImport.
type RunImportResult struct {
	// ImportStatus describes the result, e.g., "3 new records were created"
	ImportStatus string
}

// runImportResponse is the XML response structure for API_RunImport.
type runImportResponse struct {
	BaseResponse
	ImportStatus string `xml:"import_status"`
}

// RunImport executes a saved import definition.
//
// Saved imports are configured in the QuickBase UI under Import/Export.
// This allows you to run predefined imports from table to table.
//
// To find the import ID:
//  1. Open the application and click Import/Export
//  2. Select "Import into a table from another table"
//  3. Click the saved import name
//  4. Look for &id=X in the URL - X is the import ID
//
// Example:
//
//	result, err := xmlClient.RunImport(ctx, tableId, 10)
//	fmt.Println(result.ImportStatus) // "3 new records were created"
//
// See: https://help.quickbase.com/docs/api-runimport
func (c *Client) RunImport(ctx context.Context, tableId string, importId int) (*RunImportResult, error) {
	inner := "<id>" + strconv.Itoa(importId) + "</id>"

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_RunImport", body)
	if err != nil {
		return nil, fmt.Errorf("API_RunImport: %w", err)
	}

	var resp runImportResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_RunImport response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &RunImportResult{
		ImportStatus: resp.ImportStatus,
	}, nil
}

// CopyMasterDetailOptions configures the master-detail copy operation.
type CopyMasterDetailOptions struct {
	// DestRecordID is the destination master record ID.
	// Set to 0 to copy the source master record and create a new one.
	// Set to an existing record ID to import detail records into that master.
	DestRecordID int

	// SourceRecordID is the source master record ID to copy from (required).
	SourceRecordID int

	// CopyFieldID is the field ID to use for naming the copied record.
	// Only required when DestRecordID is 0 (creating a new master record).
	// The new record name will be "Copy of [field value]".
	// Must be a text field, not a lookup/formula/unique field.
	CopyFieldID int

	// Recurse copies detail records of detail records recursively.
	// Supports up to 10 levels. Default is true.
	Recurse *bool

	// RelFieldIDs limits copying to specific relationships by report link field IDs.
	// Leave empty or set to "all" to copy all relationships.
	RelFieldIDs []int
}

// CopyMasterDetailResult contains the response from API_CopyMasterDetail.
type CopyMasterDetailResult struct {
	// ParentRecordID is the record ID of the destination master record.
	// Either the existing destrid or the newly created master record.
	ParentRecordID int

	// NumCreated is the total number of new records created.
	NumCreated int
}

// copyMasterDetailResponse is the XML response structure for API_CopyMasterDetail.
type copyMasterDetailResponse struct {
	BaseResponse
	ParentRID  int `xml:"parentrid"`
	NumCreated int `xml:"numcreated"`
}

// CopyMasterDetail copies a master record with its detail records, or imports
// detail records from one master into another.
//
// This is useful for:
// - Cloning a project with all its tasks
// - Creating templates from existing records
// - Copying detail records between master records
//
// Example - Copy master and all details:
//
//	result, err := xmlClient.CopyMasterDetail(ctx, tableId, xml.CopyMasterDetailOptions{
//	    DestRecordID:   0,        // Create new master
//	    SourceRecordID: 1,        // Copy from record 1
//	    CopyFieldID:    6,        // Use field 6 for "Copy of [name]"
//	})
//	fmt.Printf("Created master record %d with %d total records\n",
//	    result.ParentRecordID, result.NumCreated)
//
// Example - Import details into existing master:
//
//	result, err := xmlClient.CopyMasterDetail(ctx, tableId, xml.CopyMasterDetailOptions{
//	    DestRecordID:   3,        // Import into existing record 3
//	    SourceRecordID: 1,        // Copy details from record 1
//	})
//
// See: https://help.quickbase.com/docs/api-copymasterdetail
func (c *Client) CopyMasterDetail(ctx context.Context, tableId string, opts CopyMasterDetailOptions) (*CopyMasterDetailResult, error) {
	inner := "<destrid>" + strconv.Itoa(opts.DestRecordID) + "</destrid>"
	inner += "<sourcerid>" + strconv.Itoa(opts.SourceRecordID) + "</sourcerid>"

	if opts.DestRecordID == 0 && opts.CopyFieldID > 0 {
		inner += "<copyfid>" + strconv.Itoa(opts.CopyFieldID) + "</copyfid>"
	}

	if opts.Recurse != nil {
		if *opts.Recurse {
			inner += "<recurse>true</recurse>"
		} else {
			inner += "<recurse>false</recurse>"
		}
	}

	if len(opts.RelFieldIDs) > 0 {
		relIds := make([]string, len(opts.RelFieldIDs))
		for i, id := range opts.RelFieldIDs {
			relIds[i] = strconv.Itoa(id)
		}
		inner += "<relfids>" + joinStrings(relIds, ",") + "</relfids>"
	}

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_CopyMasterDetail", body)
	if err != nil {
		return nil, fmt.Errorf("API_CopyMasterDetail: %w", err)
	}

	var resp copyMasterDetailResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_CopyMasterDetail response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &CopyMasterDetailResult{
		ParentRecordID: resp.ParentRID,
		NumCreated:     resp.NumCreated,
	}, nil
}

// joinStrings joins strings with a separator (simple helper to avoid importing strings)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
