package xml

import (
	"context"
	"fmt"
	"strconv"
)

// GenAddRecordFormField represents a field to pre-fill in the add record form.
type GenAddRecordFormField struct {
	// ID is the field ID (use this or Name, not both)
	ID int

	// Name is the field name (use this or ID, not both)
	Name string

	// Value is the value to pre-fill
	Value string
}

// GenAddRecordForm returns an HTML form for adding a new record.
//
// This returns the standard QuickBase "Add Record" page with optional
// pre-filled field values. The form includes edit fields and a Save button.
//
// Note: When using this form, at least one field must be modified by the user
// for the record to save (pre-filled values alone aren't sufficient).
//
// Example:
//
//	html, err := xmlClient.GenAddRecordForm(ctx, tableId, []xml.GenAddRecordFormField{
//	    {Name: "Email", Value: "user@example.com"},
//	    {ID: 7, Value: "Default Status"},
//	})
//	// Embed html in your page
//
// See: https://help.quickbase.com/docs/api-genaddrecordform
func (c *Client) GenAddRecordForm(ctx context.Context, tableId string, fields []GenAddRecordFormField) (string, error) {
	inner := ""
	for _, f := range fields {
		if f.Name != "" {
			inner += fmt.Sprintf("<field name=\"%s\">%s</field>", xmlEscape(f.Name), xmlEscape(f.Value))
		} else if f.ID > 0 {
			inner += fmt.Sprintf("<_fid_%d>%s</_fid_%d>", f.ID, xmlEscape(f.Value), f.ID)
		}
	}

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_GenAddRecordForm", body)
	if err != nil {
		return "", fmt.Errorf("API_GenAddRecordForm: %w", err)
	}

	// This API returns HTML directly, not XML
	return string(respBody), nil
}

// GenResultsTableOptions configures the results table generation.
type GenResultsTableOptions struct {
	// Query is the query string (e.g., "{'7'.CT.'Active'}")
	// Use this OR QueryID OR QueryName, not multiple.
	Query string

	// QueryID is the ID of a saved query/report
	QueryID int

	// QueryName is the name of a saved query/report
	QueryName string

	// CList is a period-delimited list of field IDs to return.
	// Use "a" to return all fields.
	CList string

	// SList is a period-delimited list of field IDs for sorting.
	SList string

	// Format specifies the output format.
	// Use JHT for JavaScript function, JSA for JavaScript array,
	// CSV for comma-separated values, or TSV for tab-separated values.
	Format GenResultsFormat

	// Options is a period-delimited list of options:
	// - num-n: return max n records
	// - skp-n: skip first n records
	// - sortorder-A: ascending sort
	// - sortorder-D: descending sort
	// - ned: omit edit icons
	// - nvw: omit view icons
	// - nfg: omit new/updated icons
	// - phd: plain (non-hyperlinked) headers
	// - abs: absolute URLs
	// - onlynew: only records marked new/updated
	Options string
}

// GenResultsFormat specifies the output format for GenResultsTable.
type GenResultsFormat string

const (
	// GenResultsFormatJHT returns HTML as a JavaScript function qdbWrite()
	GenResultsFormatJHT GenResultsFormat = "jht"

	// GenResultsFormatJHTNew returns HTML with newer CSS styles
	GenResultsFormatJHTNew GenResultsFormat = "jht_new"

	// GenResultsFormatJSA returns a JavaScript array
	GenResultsFormatJSA GenResultsFormat = "jsa"

	// GenResultsFormatCSV returns comma-separated values
	// Cannot be combined with num-n or skp-n options.
	GenResultsFormatCSV GenResultsFormat = "csv"

	// GenResultsFormatTSV returns tab-separated values
	// Cannot be combined with num-n or skp-n options.
	GenResultsFormatTSV GenResultsFormat = "tsv"
)

// GenResultsTable returns query results formatted for embedding in HTML pages.
//
// This is typically used to embed QuickBase data in external web pages.
// The output format depends on the Format option:
// - JHT: JavaScript function qdbWrite() containing HTML table
// - JSA: JavaScript array of data
// - CSV: Comma-separated values
// - TSV: Tab-separated values
//
// Example:
//
//	html, err := xmlClient.GenResultsTable(ctx, tableId, xml.GenResultsTableOptions{
//	    QueryID: 5,
//	    CList:   "6.7.8",
//	    Format:  xml.GenResultsFormatJHT,
//	    Options: "num-10.sortorder-D",
//	})
//
// See: https://help.quickbase.com/docs/api-genresultstable
func (c *Client) GenResultsTable(ctx context.Context, tableId string, opts GenResultsTableOptions) (string, error) {
	inner := ""

	if opts.Query != "" {
		inner += "<query>" + xmlEscape(opts.Query) + "</query>"
	} else if opts.QueryID > 0 {
		inner += "<qid>" + strconv.Itoa(opts.QueryID) + "</qid>"
	} else if opts.QueryName != "" {
		inner += "<qname>" + xmlEscape(opts.QueryName) + "</qname>"
	}

	if opts.CList != "" {
		inner += "<clist>" + opts.CList + "</clist>"
	}
	if opts.SList != "" {
		inner += "<slist>" + opts.SList + "</slist>"
	}

	// Set format parameters
	switch opts.Format {
	case GenResultsFormatJHT:
		inner += "<jht>1</jht>"
	case GenResultsFormatJHTNew:
		inner += "<jht>n</jht>"
	case GenResultsFormatJSA:
		inner += "<jsa>1</jsa>"
	case GenResultsFormatCSV:
		if opts.Options != "" {
			opts.Options += ".csv"
		} else {
			opts.Options = "csv"
		}
	case GenResultsFormatTSV:
		if opts.Options != "" {
			opts.Options += ".tsv"
		} else {
			opts.Options = "tsv"
		}
	}

	if opts.Options != "" {
		inner += "<options>" + opts.Options + "</options>"
	}

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_GenResultsTable", body)
	if err != nil {
		return "", fmt.Errorf("API_GenResultsTable: %w", err)
	}

	// This API returns HTML/JS/CSV content directly
	return string(respBody), nil
}

// GetRecordAsHTMLOptions configures the HTML record display.
type GetRecordAsHTMLOptions struct {
	// RecordID is the record ID to display (required)
	RecordID int

	// FormID is the optional form ID (dfid) to use for rendering.
	// If not specified, uses the default form layout.
	FormID int
}

// GetRecordAsHTML returns a record rendered as an HTML fragment.
//
// This is useful for embedding a QuickBase record view in external pages.
// The HTML matches the layout of the specified form (or default form).
//
// Example:
//
//	html, err := xmlClient.GetRecordAsHTML(ctx, tableId, xml.GetRecordAsHTMLOptions{
//	    RecordID: 123,
//	    FormID:   10, // optional: use specific form layout
//	})
//	// Embed html in your page
//
// See: https://help.quickbase.com/docs/api-getrecordashtml
func (c *Client) GetRecordAsHTML(ctx context.Context, tableId string, opts GetRecordAsHTMLOptions) (string, error) {
	inner := "<rid>" + strconv.Itoa(opts.RecordID) + "</rid>"

	if opts.FormID > 0 {
		inner += "<dfid>" + strconv.Itoa(opts.FormID) + "</dfid>"
	}

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_GetRecordAsHTML", body)
	if err != nil {
		return "", fmt.Errorf("API_GetRecordAsHTML: %w", err)
	}

	// This API returns HTML content directly
	return string(respBody), nil
}
