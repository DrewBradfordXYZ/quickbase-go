package client

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/DrewBradfordXYZ/quickbase-go/v2/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v2/generated"
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

// --- RunQuery with schema transformation ---

// RunQuery executes a query and returns the result with convenience methods.
// For all results, use RunQueryAll or RunQueryIterator.
//
// If a schema is configured, table and field aliases in the body are
// automatically resolved to IDs before the request is sent.
//
// Use result.Records() for unwrapped record data, or access result.Data directly.
func (c *Client) RunQuery(ctx context.Context, body generated.RunQueryJSONRequestBody) (*RunQueryResult, error) {
	// Transform request body if schema is configured
	transformedBody, _, err := c.transformRunQueryBody(body)
	if err != nil {
		return nil, err
	}

	resp, err := c.API().RunQueryWithResponse(ctx, transformedBody)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, parseAPIError(resp.StatusCode(), resp.Body, resp.HTTPResponse)
	}

	return &RunQueryResult{resp.JSON200}, nil
}

// transformRunQueryBody transforms a RunQuery body, resolving table alias to ID.
// Also transforms where clause field aliases.
func (c *Client) transformRunQueryBody(body generated.RunQueryJSONRequestBody) (generated.RunQueryJSONRequestBody, string, error) {
	tableID := body.From

	if c.schema == nil {
		return body, tableID, nil
	}

	result := body

	// Resolve table alias in 'from'
	resolvedTableID, err := core.ResolveTableAlias(c.schema, body.From)
	if err != nil {
		return body, "", err
	}
	result.From = resolvedTableID
	tableID = resolvedTableID

	// Transform where clause field aliases
	if body.Where != nil {
		// Extract the where string from the union type
		if whereStr, ok := extractWhereString(body.Where); ok {
			// Transform field aliases in where clause
			bodyMap := map[string]any{
				"from":  body.From,
				"where": whereStr,
			}
			transformed, _, _ := core.TransformRequest(bodyMap, c.schema)
			if transformedWhere, ok := transformed["where"].(string); ok {
				// Convert back to union type
				whereUnion, err := StringToWhereUnion(transformedWhere)
				if err == nil {
					result.Where = whereUnion
				}
			}
		}
	}

	return result, tableID, nil
}

// RunQueryAll fetches all records across all pages.
// This automatically handles pagination and returns raw QuickbaseRecord slices.
// Use UnwrapRecords() from the quickbase package to convert to map[string]any.
func (c *Client) RunQueryAll(ctx context.Context, body generated.RunQueryJSONRequestBody) ([]generated.QuickbaseRecord, error) {
	fetcher := c.runQueryFetcher(body)
	return CollectAll(ctx, fetcher)
}

// RunQueryN fetches up to n records across pages.
// This automatically handles pagination and returns raw QuickbaseRecord slices.
// Use UnwrapRecords() from the quickbase package to convert to map[string]any.
func (c *Client) RunQueryN(ctx context.Context, body generated.RunQueryJSONRequestBody, n int) ([]generated.QuickbaseRecord, error) {
	fetcher := c.runQueryFetcher(body)
	return CollectN(ctx, fetcher, n)
}

// runQueryFetcher creates a page fetcher for RunQuery
func (c *Client) runQueryFetcher(body generated.RunQueryJSONRequestBody) PageFetcher[generated.QuickbaseRecord, *runQueryPageResponse] {
	return func(ctx context.Context, skip int, nextToken string) (*runQueryPageResponse, error) {
		// Create a copy of body with updated skip
		bodyCopy := body
		if bodyCopy.Options == nil {
			bodyCopy.Options = &generated.RunQueryJSONBody_Options{}
		}
		bodyCopy.Options.Skip = &skip

		resp, err := c.API().RunQueryWithResponse(ctx, bodyCopy)
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, parseAPIError(resp.StatusCode(), resp.Body, resp.HTTPResponse)
		}

		var data []generated.QuickbaseRecord
		if resp.JSON200.Data != nil {
			data = *resp.JSON200.Data
		}

		var metadata PaginationMetadata
		if resp.JSON200.Metadata != nil {
			m := resp.JSON200.Metadata
			metadata = PaginationMetadata{
				TotalRecords: &m.TotalRecords,
				NumRecords:   &m.NumRecords,
				Skip:         m.Skip,
			}
		}

		return &runQueryPageResponse{data: data, metadata: metadata}, nil
	}
}

// runQueryPageResponse adapts the generated response to PaginatedResponse interface
type runQueryPageResponse struct {
	data     []generated.QuickbaseRecord
	metadata PaginationMetadata
}

func (r *runQueryPageResponse) GetData() []generated.QuickbaseRecord {
	return r.data
}

func (r *runQueryPageResponse) GetMetadata() PaginationMetadata {
	return r.metadata
}

// --- RunReport pagination helpers ---

// RunReportAll fetches all records from a report across all pages.
// This automatically handles pagination.
func (c *Client) RunReportAll(ctx context.Context, reportID string, tableID string) ([]generated.QuickbaseRecord, error) {
	fetcher := c.runReportFetcher(reportID, tableID)
	return CollectAll(ctx, fetcher)
}

// RunReportN fetches up to n records from a report across pages.
func (c *Client) RunReportN(ctx context.Context, reportID string, tableID string, n int) ([]generated.QuickbaseRecord, error) {
	fetcher := c.runReportFetcher(reportID, tableID)
	return CollectN(ctx, fetcher, n)
}

// runReportFetcher creates a page fetcher for RunReport
func (c *Client) runReportFetcher(reportID string, tableID string) PageFetcher[generated.QuickbaseRecord, *runReportPageResponse] {
	return func(ctx context.Context, skip int, nextToken string) (*runReportPageResponse, error) {
		params := &generated.RunReportParams{
			TableId: tableID,
			Skip:    &skip,
		}

		resp, err := c.API().RunReportWithResponse(ctx, reportID, params, nil)
		if err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, parseAPIError(resp.StatusCode(), resp.Body, resp.HTTPResponse)
		}

		var data []generated.QuickbaseRecord
		if resp.JSON200.Data != nil {
			data = *resp.JSON200.Data
		}

		var metadata PaginationMetadata
		if resp.JSON200.Metadata != nil {
			m := resp.JSON200.Metadata
			metadata = PaginationMetadata{
				TotalRecords: &m.TotalRecords,
				NumRecords:   &m.NumRecords,
				Skip:         m.Skip,
			}
		}

		return &runReportPageResponse{data: data, metadata: metadata}, nil
	}
}

// runReportPageResponse adapts the generated response to PaginatedResponse interface
type runReportPageResponse struct {
	data     []generated.QuickbaseRecord
	metadata PaginationMetadata
}

func (r *runReportPageResponse) GetData() []generated.QuickbaseRecord {
	return r.data
}

func (r *runReportPageResponse) GetMetadata() PaginationMetadata {
	return r.metadata
}

// --- Helper functions ---
// These are kept for internal use. For user-facing helpers, see quickbase.go

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
