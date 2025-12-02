package client

import (
	"context"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

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
func (c *Client) RunQuery(ctx context.Context, body generated.RunQueryJSONRequestBody) (*RunQueryResult, error) {
	resp, err := c.API().RunQueryWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, &core.QuickbaseError{Message: "unexpected response", StatusCode: resp.StatusCode()}
	}

	result := &RunQueryResult{}

	if resp.JSON200.Data != nil {
		result.Data = *resp.JSON200.Data
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
			return nil, &core.QuickbaseError{Message: "unexpected response", StatusCode: resp.StatusCode()}
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

// UpsertResult contains the result of an Upsert call
type UpsertResult struct {
	CreatedRecordIDs              []int
	UnchangedRecordIDs            []int
	UpdatedRecordIDs              []int
	TotalNumberOfRecordsProcessed int
}

// Upsert inserts or updates records in a table.
func (c *Client) Upsert(ctx context.Context, body generated.UpsertJSONRequestBody) (*UpsertResult, error) {
	resp, err := c.API().UpsertWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, &core.QuickbaseError{Message: "unexpected response", StatusCode: resp.StatusCode()}
	}

	result := &UpsertResult{}
	m := resp.JSON200.Metadata

	if m.CreatedRecordIds != nil {
		result.CreatedRecordIDs = *m.CreatedRecordIds
	}
	if m.UnchangedRecordIds != nil {
		result.UnchangedRecordIDs = *m.UnchangedRecordIds
	}
	if m.UpdatedRecordIds != nil {
		result.UpdatedRecordIDs = *m.UpdatedRecordIds
	}
	if m.TotalNumberOfRecordsProcessed != nil {
		result.TotalNumberOfRecordsProcessed = *m.TotalNumberOfRecordsProcessed
	}

	return result, nil
}

// DeleteRecordsResult contains the result of a DeleteRecords call
type DeleteRecordsResult struct {
	NumberDeleted int
}

// DeleteRecords deletes records matching a query.
func (c *Client) DeleteRecords(ctx context.Context, body generated.DeleteRecordsJSONRequestBody) (*DeleteRecordsResult, error) {
	resp, err := c.API().DeleteRecordsWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, &core.QuickbaseError{Message: "unexpected response", StatusCode: resp.StatusCode()}
	}

	return &DeleteRecordsResult{
		NumberDeleted: derefInt(resp.JSON200.NumberDeleted),
	}, nil
}

// GetAppResult contains the result of a GetApp call
type GetAppResult struct {
	ID          string
	Name        string
	Description string
	Created     string
	Updated     string
	DateFormat  string
	TimeZone    string
}

// GetApp retrieves app details.
func (c *Client) GetApp(ctx context.Context, appID string) (*GetAppResult, error) {
	resp, err := c.API().GetAppWithResponse(ctx, appID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, &core.QuickbaseError{Message: "unexpected response", StatusCode: resp.StatusCode()}
	}

	return &GetAppResult{
		ID:          derefStr(resp.JSON200.Id),
		Name:        resp.JSON200.Name,
		Description: derefStr(resp.JSON200.Description),
		Created:     derefStr(resp.JSON200.Created),
		Updated:     derefStr(resp.JSON200.Updated),
		DateFormat:  derefStr(resp.JSON200.DateFormat),
		TimeZone:    derefStr(resp.JSON200.TimeZone),
	}, nil
}

// FieldDetails contains information about a field
type FieldDetails struct {
	ID        int
	Label     string
	FieldType string
}

// GetFields retrieves all fields for a table.
func (c *Client) GetFields(ctx context.Context, tableID string) ([]FieldDetails, error) {
	resp, err := c.API().GetFieldsWithResponse(ctx, &generated.GetFieldsParams{TableId: tableID})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, &core.QuickbaseError{Message: "unexpected response", StatusCode: resp.StatusCode()}
	}

	var fields []FieldDetails
	for _, f := range *resp.JSON200 {
		fields = append(fields, FieldDetails{
			ID:        int(f.Id),
			Label:     derefStr(f.Label),
			FieldType: derefStr(f.FieldType),
		})
	}
	return fields, nil
}

// helper functions

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
