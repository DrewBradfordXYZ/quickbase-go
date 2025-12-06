package client

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
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
		_ = json.Unmarshal(body, &errBody) // ignore parse errors, use what we got
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

// --- Friendly wrapper methods ---

// RunQueryResult contains the result of a RunQuery call
type RunQueryResult struct {
	Data     []generated.QuickbaseRecord
	Fields   []FieldInfo
	Metadata QueryMetadata
}

// FieldInfo contains metadata about a field in query results
type FieldInfo struct {
	ID    int
	Label string
	Type  string
}

// QueryMetadata contains pagination metadata from a query
type QueryMetadata struct {
	TotalRecords int
	NumRecords   int
	NumFields    int
	Skip         int
}

// RunQuery executes a query and returns the first page of results.
// For all results, use RunQueryAll or RunQueryIterator.
//
// If a schema is configured, table and field aliases in the body are
// automatically resolved to IDs, and response field IDs are converted
// back to aliases with values unwrapped.
func (c *Client) RunQuery(ctx context.Context, body generated.RunQueryJSONRequestBody) (*RunQueryResult, error) {
	// Transform request body if schema is configured
	transformedBody, tableID, err := c.transformRunQueryBody(body)
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

	result := &RunQueryResult{}

	if resp.JSON200.Data != nil {
		// Transform response data if schema is configured
		result.Data = c.transformRecords(*resp.JSON200.Data, tableID)
	}

	if resp.JSON200.Fields != nil {
		for _, f := range *resp.JSON200.Fields {
			info := FieldInfo{}
			if f.Id != nil {
				info.ID = *f.Id
			}
			if f.Label != nil {
				info.Label = *f.Label
			}
			if f.Type != nil {
				info.Type = *f.Type
			}
			result.Fields = append(result.Fields, info)
		}
	}

	if resp.JSON200.Metadata != nil {
		m := resp.JSON200.Metadata
		result.Metadata = QueryMetadata{
			TotalRecords: m.TotalRecords,
			NumRecords:   m.NumRecords,
			NumFields:    m.NumFields,
			Skip:         derefInt(m.Skip),
		}
	}

	return result, nil
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
		// Convert body to map for where transformation
		bodyMap := map[string]any{
			"from":  body.From,
			"where": *body.Where,
		}
		transformed, _, _ := core.TransformRequest(bodyMap, c.schema)
		if where, ok := transformed["where"].(string); ok {
			result.Where = &where
		}
	}

	return result, tableID, nil
}

// transformRecords transforms response records, converting field IDs to aliases and unwrapping values.
func (c *Client) transformRecords(records []generated.QuickbaseRecord, tableID string) []generated.QuickbaseRecord {
	if c.schema == nil || tableID == "" {
		return records
	}

	result := make([]generated.QuickbaseRecord, len(records))
	for i, record := range records {
		// Convert record to map[string]any for transformation
		recordMap := make(map[string]any)
		for k, v := range record {
			recordMap[k] = v
		}

		// Transform
		response := map[string]any{"data": []any{recordMap}}
		transformed := core.TransformResponse(response, c.schema, tableID)

		// Convert back to QuickbaseRecord
		if data, ok := transformed["data"].([]any); ok && len(data) > 0 {
			if transformedRecord, ok := data[0].(map[string]any); ok {
				newRecord := make(generated.QuickbaseRecord)
				for k, v := range transformedRecord {
					// Wrap value back into FieldValue format for type compatibility
					newRecord[k] = wrapFieldValue(v)
				}
				result[i] = newRecord
			}
		} else {
			result[i] = record
		}
	}

	return result
}

// wrapFieldValue wraps an unwrapped value back into the FieldValue format.
// The generated FieldValue has a union type for Value, so we store the raw value.
func wrapFieldValue(v any) generated.FieldValue {
	// Marshal and unmarshal to create proper FieldValue
	wrapped := map[string]any{"value": v}
	data, _ := json.Marshal(wrapped)
	var fv generated.FieldValue
	_ = json.Unmarshal(data, &fv)
	return fv
}

// RunQueryAll fetches all records across all pages.
// This automatically handles pagination.
func (c *Client) RunQueryAll(ctx context.Context, body generated.RunQueryJSONRequestBody) ([]generated.QuickbaseRecord, error) {
	fetcher := c.runQueryFetcher(body)
	return CollectAll(ctx, fetcher)
}

// RunQueryN fetches up to n records across pages.
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

// helper functions used by RunQuery

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
