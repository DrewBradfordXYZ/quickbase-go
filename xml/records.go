package xml

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
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
	inner := ""
	if query != "" {
		inner = "<query>" + query + "</query>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, tableId, "API_DoQueryCount", body)
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
// See: https://help.quickbase.com/docs/api-getrecordinfo
func (c *Client) GetRecordInfo(ctx context.Context, tableId string, recordId int) (*GetRecordInfoResult, error) {
	inner := "<rid>" + strconv.Itoa(recordId) + "</rid>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, tableId, "API_GetRecordInfo", body)
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
	}, nil
}

// GetRecordInfoByKey returns a single record using a custom key field value.
//
// This is similar to GetRecordInfo but uses a key field value instead of record ID.
// The keyValue should be the value from the table's designated key field.
//
// Example:
//
//	// If "Order Number" (field 6) is the key field
//	result, err := xmlClient.GetRecordInfoByKey(ctx, tableId, "ORD-12345")
//
// See: https://help.quickbase.com/docs/api-getrecordinfo
func (c *Client) GetRecordInfoByKey(ctx context.Context, tableId string, keyValue string) (*GetRecordInfoResult, error) {
	inner := "<key>" + keyValue + "</key>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, tableId, "API_GetRecordInfo", body)
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
	}, nil
}
