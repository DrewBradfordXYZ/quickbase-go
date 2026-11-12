package client

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"strconv"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/generated"
)

// parseAPIError extracts a typed error from a non-200 API response.
// It parses the response body to get message, description, and any field errors,
// then returns the appropriate error type based on status code.
func parseAPIError(statusCode int, body []byte, httpResp *http.Response) error {
	// Extract ray ID from headers
	var rayID string
	if httpResp != nil {
		rayID = httpResp.Header.Get("qb-api-ray")
		if rayID == "" {
			rayID = httpResp.Header.Get("cf-ray")
		}
	}

	// Parse error body
	var errBody struct {
		Message     string            `json:"message"`
		Description string            `json:"description"`
		Errors      []core.FieldError `json:"errors"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &errBody); err != nil {
			// If JSON parsing fails, use raw body as message
			errBody.Message = string(body)
		}
	}

	message := errBody.Message
	if message == "" {
		message = http.StatusText(statusCode)
	}

	switch statusCode {
	case 400:
		return &core.ValidationError{
			QuickbaseError: core.QuickbaseError{
				Message:     message,
				Description: errBody.Description,
				StatusCode:  statusCode,
				RayID:       rayID,
			},
			Errors: errBody.Errors,
		}
	case 401:
		return &core.AuthenticationError{
			QuickbaseError: core.QuickbaseError{
				Message:     message,
				Description: errBody.Description,
				StatusCode:  statusCode,
				RayID:       rayID,
			},
		}
	case 403:
		return &core.AuthorizationError{
			QuickbaseError: core.QuickbaseError{
				Message:     message,
				Description: errBody.Description,
				StatusCode:  statusCode,
				RayID:       rayID,
			},
		}
	case 404:
		return &core.NotFoundError{
			QuickbaseError: core.QuickbaseError{
				Message:     message,
				Description: errBody.Description,
				StatusCode:  statusCode,
				RayID:       rayID,
			},
		}
	default:
		if statusCode >= 500 {
			return &core.ServerError{
				QuickbaseError: core.QuickbaseError{
					Message:     message,
					Description: errBody.Description,
					StatusCode:  statusCode,
					RayID:       rayID,
				},
			}
		}
		return &core.QuickbaseError{
			Message:     message,
			Description: errBody.Description,
			StatusCode:  statusCode,
			RayID:       rayID,
		}
	}
}

// --- Paging: RunQuery and RunReport across pages ---
//
// RunQuery and RunReport (generated) return one page. These fetch every
// page, or n records, or yield records a page at a time. Schema aliases in
// the body are resolved in the HTTP layer, as for every request.

// RunQueryAll fetches every record across all pages.
func (c *Client) RunQueryAll(ctx context.Context, body generated.RunQueryJSONRequestBody) ([]generated.QuickbaseRecord, error) {
	return CollectAll(ctx, c.runQueryFetcher(body))
}

// RunQueryN fetches up to n records across pages.
func (c *Client) RunQueryN(ctx context.Context, body generated.RunQueryJSONRequestBody, n int) ([]generated.QuickbaseRecord, error) {
	return CollectN(ctx, c.runQueryFetcher(body), n)
}

// RunQueryIterator yields every record across all pages, fetching a page at
// a time, so a large table never has to fit in memory. Stop early by
// breaking out of the loop.
//
//	for record, err := range client.RunQueryIterator(ctx, body) {
//	    if err != nil {
//	        return err
//	    }
//	    // process record
//	}
func (c *Client) RunQueryIterator(ctx context.Context, body generated.RunQueryJSONRequestBody) iter.Seq2[generated.QuickbaseRecord, error] {
	return Paginate(ctx, c.runQueryFetcher(body))
}

// runQueryFetcher fetches one page of a query at the given skip.
func (c *Client) runQueryFetcher(body generated.RunQueryJSONRequestBody) PageFetcher[generated.QuickbaseRecord, *recordPage] {
	return func(ctx context.Context, skip int, _ string) (*recordPage, error) {
		// Copy the body and its options: setting Skip on the caller's
		// Options would change their body.
		page := body
		var opts generated.RunQueryJSONBody_Options
		if body.Options != nil {
			opts = *body.Options
		}
		opts.Skip = &skip
		page.Options = &opts

		result, err := c.RunQuery(ctx, page)
		if err != nil {
			return nil, err
		}
		m := result.Metadata
		return &recordPage{data: c.keyByAlias(body.From, result.Data), metadata: PaginationMetadata{TotalRecords: &m.TotalRecords, NumRecords: &m.NumRecords, Skip: &m.Skip}}, nil
	}
}

// RunReportAll fetches every record of a report across all pages.
func (c *Client) RunReportAll(ctx context.Context, reportID string, tableID string) ([]generated.QuickbaseRecord, error) {
	return CollectAll(ctx, c.runReportFetcher(reportID, tableID))
}

// RunReportN fetches up to n records of a report across pages.
func (c *Client) RunReportN(ctx context.Context, reportID string, tableID string, n int) ([]generated.QuickbaseRecord, error) {
	return CollectN(ctx, c.runReportFetcher(reportID, tableID), n)
}

// RunReportIterator yields every record of a report, a page at a time.
func (c *Client) RunReportIterator(ctx context.Context, reportID string, tableID string) iter.Seq2[generated.QuickbaseRecord, error] {
	return Paginate(ctx, c.runReportFetcher(reportID, tableID))
}

// runReportFetcher fetches one page of a report at the given skip.
func (c *Client) runReportFetcher(reportID string, tableID string) PageFetcher[generated.QuickbaseRecord, *recordPage] {
	return func(ctx context.Context, skip int, _ string) (*recordPage, error) {
		result, err := c.RunReport(ctx, reportID, generated.RunReportParams{TableId: tableID, Skip: &skip})
		if err != nil {
			return nil, err
		}
		m := result.Metadata
		return &recordPage{data: c.keyByAlias(tableID, result.Data), metadata: PaginationMetadata{TotalRecords: &m.TotalRecords, NumRecords: &m.NumRecords, Skip: &m.Skip}}, nil
	}
}

// keyByAlias keys records by field alias instead of field ID when the
// client's schema has TransformResponses. Fields the schema doesn't name
// keep their ID.
func (c *Client) keyByAlias(table string, records []generated.QuickbaseRecord) []generated.QuickbaseRecord {
	if c.schema == nil || !c.schema.Options.TransformResponses {
		return records
	}
	tableID, err := core.ResolveTableAlias(c.schema, table)
	if err != nil {
		return records
	}
	for i, record := range records {
		keyed := make(generated.QuickbaseRecord, len(record))
		for key, value := range record {
			if id, err := strconv.Atoi(key); err == nil {
				if alias := core.GetFieldAlias(c.schema, tableID, id); alias != "" {
					key = alias
				}
			}
			keyed[key] = value
		}
		records[i] = keyed
	}
	return records
}

// recordPage is a page of records for the paging helpers.
type recordPage struct {
	data     []generated.QuickbaseRecord
	metadata PaginationMetadata
}

func (r *recordPage) GetData() []generated.QuickbaseRecord { return r.data }

func (r *recordPage) GetMetadata() PaginationMetadata { return r.metadata }
