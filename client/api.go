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

// --- RunReport pagination helpers ---

// RunReportResult contains the result of a RunReport call
type RunReportResult struct {
	Data     []generated.QuickbaseRecord
	Fields   []FieldInfo
	Metadata QueryMetadata
}

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

// helper functions

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

// --- GetUsers result types ---

// UserInfo contains simplified user information.
type UserInfo struct {
	ID        string // HashId - unique identifier
	Email     string
	FirstName string
	LastName  string
	UserName  string
}

// GetUsersResult wraps the getUsers response with helper methods.
type GetUsersResult struct {
	raw *generated.GetUsersResponse
}

// Users returns the list of users as simplified UserInfo structs.
func (r *GetUsersResult) Users() []UserInfo {
	if r.raw == nil || r.raw.JSON200 == nil {
		return nil
	}
	users := make([]UserInfo, len(r.raw.JSON200.Users))
	for i, u := range r.raw.JSON200.Users {
		users[i] = UserInfo{
			ID:        u.HashId,
			Email:     u.EmailAddress,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			UserName:  u.UserName,
		}
	}
	return users
}

// NextPageToken returns the pagination token for the next page.
// Returns empty string if there are no more pages.
func (r *GetUsersResult) NextPageToken() string {
	if r.raw == nil || r.raw.JSON200 == nil {
		return ""
	}
	return r.raw.JSON200.Metadata.NextPageToken
}

// Raw returns the underlying generated response for advanced use cases.
func (r *GetUsersResult) Raw() *generated.GetUsersResponse {
	return r.raw
}

// --- GetRelationships result types ---

// GetRelationshipsResult wraps the getRelationships response with helper methods.
type GetRelationshipsResult struct {
	raw *generated.GetRelationshipsResponse
}

// Relationships returns the list of relationships as simplified RelationshipInfo structs.
// Note: RelationshipInfo is defined in builders_generated.go
func (r *GetRelationshipsResult) Relationships() []RelationshipInfo {
	if r.raw == nil || r.raw.JSON200 == nil {
		return nil
	}
	rels := make([]RelationshipInfo, len(r.raw.JSON200.Relationships))
	for i, rel := range r.raw.JSON200.Relationships {
		rels[i] = RelationshipInfo{
			ID:            rel.Id,
			ParentTableID: rel.ParentTableId,
			ChildTableID:  rel.ChildTableId,
			IsCrossApp:    rel.IsCrossApp,
		}
	}
	return rels
}

// Raw returns the underlying generated response for advanced use cases.
func (r *GetRelationshipsResult) Raw() *generated.GetRelationshipsResponse {
	return r.raw
}

// --- GetFieldUsage / GetFieldsUsage result types ---

// FieldUsageInfo contains field information with usage summary.
type FieldUsageInfo struct {
	ID         int
	Name       string
	Type       string
	TotalUsage int                          // Sum of all usage counts
	Usage      *generated.GetFieldsUsage_200_Usage // Full usage details
}

// GetFieldUsageResult wraps the getFieldUsage response with helper methods.
type GetFieldUsageResult struct {
	raw *generated.GetFieldUsageResponse
}

// Fields returns the field usage information.
// Note: getFieldUsage returns an array but typically contains one item.
func (r *GetFieldUsageResult) Fields() []FieldUsageInfo {
	if r.raw == nil || r.raw.JSON200 == nil {
		return nil
	}
	return convertFieldUsageItems(*r.raw.JSON200)
}

// Raw returns the underlying generated response for advanced use cases.
func (r *GetFieldUsageResult) Raw() *generated.GetFieldUsageResponse {
	return r.raw
}

// GetFieldsUsageResult wraps the getFieldsUsage response with helper methods.
type GetFieldsUsageResult struct {
	raw *generated.GetFieldsUsageResponse
}

// Fields returns the list of field usage information.
func (r *GetFieldsUsageResult) Fields() []FieldUsageInfo {
	if r.raw == nil || r.raw.JSON200 == nil {
		return nil
	}
	items := *r.raw.JSON200
	results := make([]FieldUsageInfo, len(items))
	for i, item := range items {
		results[i] = FieldUsageInfo{
			ID:         item.Field.Id,
			Name:       item.Field.Name,
			Type:       item.Field.Type,
			TotalUsage: sumFieldUsage(&item.Usage),
			Usage:      &item.Usage,
		}
	}
	return results
}

// Raw returns the underlying generated response for advanced use cases.
func (r *GetFieldsUsageResult) Raw() *generated.GetFieldsUsageResponse {
	return r.raw
}

// convertFieldUsageItems converts GetFieldUsage items to FieldUsageInfo.
// The types are slightly different between GetFieldUsage and GetFieldsUsage.
func convertFieldUsageItems(items []generated.GetFieldUsage_200_Item) []FieldUsageInfo {
	results := make([]FieldUsageInfo, len(items))
	for i, item := range items {
		// Convert the usage type
		usage := generated.GetFieldsUsage_200_Usage{
			Actions:         generated.GetFieldsUsage_200_Usage_Actions{Count: item.Usage.Actions.Count},
			AppHomePages:    generated.GetFieldsUsage_200_Usage_AppHomePages{Count: item.Usage.AppHomePages.Count},
			Dashboards:      struct{ Count int `json:"count"` }{Count: item.Usage.Dashboards.Count},
			DefaultReports:  generated.GetFieldsUsage_200_Usage_DefaultReports{Count: item.Usage.DefaultReports.Count},
			ExactForms:      generated.GetFieldsUsage_200_Usage_ExactForms{Count: item.Usage.ExactForms.Count},
			Fields:          generated.GetFieldsUsage_200_Usage_Fields{Count: item.Usage.Fields.Count},
			Forms:           generated.GetFieldsUsage_200_Usage_Forms{Count: item.Usage.Forms.Count},
			Notifications:   generated.GetFieldsUsage_200_Usage_Notifications{Count: item.Usage.Notifications.Count},
			PersonalReports: generated.GetFieldsUsage_200_Usage_PersonalReports{Count: item.Usage.PersonalReports.Count},
			Pipelines:       generated.GetFieldsUsage_200_Usage_Pipelines{Count: item.Usage.Pipelines.Count},
			Relationships:   generated.GetFieldsUsage_200_Usage_Relationships{Count: item.Usage.Relationships.Count},
			Reminders:       generated.GetFieldsUsage_200_Usage_Reminders{Count: item.Usage.Reminders.Count},
			Reports:         generated.GetFieldsUsage_200_Usage_Reports{Count: item.Usage.Reports.Count},
			Roles:           generated.GetFieldsUsage_200_Usage_Roles{Count: item.Usage.Roles.Count},
			TableImports:    struct{ Count int `json:"count"` }{Count: item.Usage.TableImports.Count},
			TableRules:      struct{ Count int `json:"count"` }{Count: item.Usage.TableRules.Count},
			Webhooks:        generated.GetFieldsUsage_200_Usage_Webhooks{Count: item.Usage.Webhooks.Count},
		}
		results[i] = FieldUsageInfo{
			ID:         item.Field.Id,
			Name:       item.Field.Name,
			Type:       item.Field.Type,
			TotalUsage: sumFieldUsage(&usage),
			Usage:      &usage,
		}
	}
	return results
}

// sumFieldUsage calculates the total usage count across all usage types.
func sumFieldUsage(u *generated.GetFieldsUsage_200_Usage) int {
	if u == nil {
		return 0
	}
	return u.Actions.Count +
		u.AppHomePages.Count +
		u.Dashboards.Count +
		u.DefaultReports.Count +
		u.ExactForms.Count +
		u.Fields.Count +
		u.Forms.Count +
		u.Notifications.Count +
		u.PersonalReports.Count +
		u.Pipelines.Count +
		u.Relationships.Count +
		u.Reminders.Count +
		u.Reports.Count +
		u.Roles.Count +
		u.TableImports.Count +
		u.TableRules.Count +
		u.Webhooks.Count
}

// --- GetFields result types ---

// FieldPermission represents a role's permission on a field.
type FieldPermission struct {
	RoleID         int
	RoleName       string
	PermissionType string // "None", "View", "Modify"
}

// SchemaFieldInfo contains comprehensive field information for schema discovery.
type SchemaFieldInfo struct {
	ID               int
	Label            string
	FieldType        string
	Mode             string // "lookup", "summary", "formula", or "" for regular fields
	Required         bool
	Unique           bool
	Permissions      []FieldPermission
	Properties       *generated.GetFields_200_Properties // Full properties for advanced use
	AppearsByDefault bool
	FindEnabled      bool
	NoWrap           bool
	Bold             bool
	Audited          bool
	DoesDataCopy     bool
	FieldHelp        string
}

// GetFieldsResult wraps the getFields response with helper methods.
type GetFieldsResult struct {
	raw *generated.GetFieldsResponse
}

// Fields returns the list of fields with comprehensive schema information.
func (r *GetFieldsResult) Fields() []SchemaFieldInfo {
	if r.raw == nil || r.raw.JSON200 == nil {
		return nil
	}
	items := *r.raw.JSON200
	results := make([]SchemaFieldInfo, len(items))
	for i, f := range items {
		info := SchemaFieldInfo{
			ID:               int(f.Id),
			Label:            derefString(f.Label),
			FieldType:        derefString(f.FieldType),
			Mode:             derefString(f.Mode),
			Required:         derefBool(f.Required),
			Unique:           derefBool(f.Unique),
			Properties:       f.Properties,
			AppearsByDefault: derefBool(f.AppearsByDefault),
			FindEnabled:      derefBool(f.FindEnabled),
			NoWrap:           derefBool(f.NoWrap),
			Bold:             derefBool(f.Bold),
			Audited:          derefBool(f.Audited),
			DoesDataCopy:     derefBool(f.DoesDataCopy),
			FieldHelp:        derefString(f.FieldHelp),
		}
		if f.Permissions != nil {
			for _, p := range *f.Permissions {
				info.Permissions = append(info.Permissions, FieldPermission{
					RoleID:         derefInt(p.RoleId),
					RoleName:       derefString(p.Role),
					PermissionType: derefString(p.PermissionType),
				})
			}
		}
		results[i] = info
	}
	return results
}

// Roles returns a deduplicated list of all roles found across all fields.
func (r *GetFieldsResult) Roles() []RoleInfo {
	if r.raw == nil || r.raw.JSON200 == nil {
		return nil
	}
	roleMap := make(map[int]RoleInfo)
	for _, f := range *r.raw.JSON200 {
		if f.Permissions == nil {
			continue
		}
		for _, p := range *f.Permissions {
			if p.RoleId != nil {
				roleMap[*p.RoleId] = RoleInfo{
					ID:   *p.RoleId,
					Name: derefString(p.Role),
				}
			}
		}
	}
	roles := make([]RoleInfo, 0, len(roleMap))
	for _, r := range roleMap {
		roles = append(roles, r)
	}
	return roles
}

// Raw returns the underlying generated response for advanced use cases.
func (r *GetFieldsResult) Raw() *generated.GetFieldsResponse {
	return r.raw
}

// RoleInfo contains basic role information extracted from field permissions.
type RoleInfo struct {
	ID   int
	Name string
}

// helper
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
