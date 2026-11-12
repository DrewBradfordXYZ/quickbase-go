// Package quickbase provides a Go SDK for the QuickBase API.
//
// This SDK provides:
//   - Multiple authentication strategies (user token, temp token, SSO, ticket)
//   - Automatic retry with exponential backoff and jitter
//   - Proactive rate limiting with sliding window throttle
//   - Custom error types for different HTTP status codes
//   - Debug logging
//   - Date transformation (ISO strings to time.Time)
//
// # Authentication
//
// User token (recommended for server-side apps):
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithUserToken("b9f3pk_xxxx_xxxxxxxxxxxxxxx"),
//	)
//
// Ticket auth (username/password with proper createdBy/modifiedBy attribution):
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithTicketAuth("user@example.com", "password"),
//	)
//
// SSO/SAML (make API calls as a specific user):
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithSSOTokenAuth(samlAssertion),
//	)
//
// Temp token (for browser-initiated requests with tokens):
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithTempTokens(map[string]string{"bqr1111": token}),
//	)
//
// See the [auth] package for detailed documentation on each method.
//
// # Basic Usage
//
//	client, err := quickbase.New("myrealm",
//	    quickbase.WithUserToken("your-token"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use wrapper methods
//	app, _ := client.GetApp(ctx, "bqxyz123")
//	records, _ := client.RunQueryAll(ctx, quickbase.RunQueryBody{From: tableId})
//
//	// Or access the generated API directly for full control
//	resp, _ := client.API().GetAppWithResponse(ctx, appId)
//
// # Rate Limiting
//
// Proactive throttling (100 req/10s is QuickBase's limit):
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithUserToken("token"),
//	    quickbase.WithProactiveThrottle(100),
//	)
//
// Rate limit callback:
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithUserToken("token"),
//	    quickbase.WithOnRateLimit(func(info quickbase.RateLimitInfo) {
//	        log.Printf("Rate limited! Retry after %ds", info.RetryAfter)
//	    }),
//	)
package quickbase

import (
	"strconv"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/client"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/generated"
)

// Client is the main QuickBase API client.
type Client = client.Client

// Re-export types for convenience
type (

	// Error types
	QuickbaseError      = core.QuickbaseError
	RateLimitError      = core.RateLimitError
	AuthenticationError = core.AuthenticationError
	AuthorizationError  = core.AuthorizationError
	NotFoundError       = core.NotFoundError
	ValidationError     = core.ValidationError
	TimeoutError        = core.TimeoutError
	ServerError         = core.ServerError
	MissingTokenError   = core.MissingTokenError
	ReadOnlyError       = core.ReadOnlyError
	PlainTimeError      = core.PlainTimeError
	LineErrorsError     = core.LineErrorsError
	RateLimitInfo       = core.RateLimitInfo

	// Date and Date/Time field values
	Date     = core.Date
	DateTime = core.DateTime

	// Schema types
	Schema         = core.Schema
	TableSchema    = core.TableSchema
	SchemaOptions  = core.SchemaOptions
	ResolvedSchema = core.ResolvedSchema
	SchemaError    = core.SchemaError
	SchemaBuilder  = core.SchemaBuilder

	// Throttle types
	SlidingWindowThrottle = client.SlidingWindowThrottle
	NoOpThrottle          = client.NoOpThrottle
	Throttle              = client.Throttle

	// Pagination types
	PaginationMetadata = client.PaginationMetadata
	PaginationOptions  = client.PaginationOptions
	PaginationType     = client.PaginationType

	// Monitoring types
	RequestInfo = client.RequestInfo
	RetryInfo   = client.RetryInfo
)

// Pagination type constants
const (
	PaginationTypeSkip  = client.PaginationTypeSkip
	PaginationTypeToken = client.PaginationTypeToken
	PaginationTypeNone  = client.PaginationTypeNone
)

// Option configures a Client. It is the client package's Option: each With
// function here returns that package's, which documents it in full.
type Option = client.Option

// WithUserToken configures user token authentication. See [client.WithUserToken].
func WithUserToken(token string) Option { return client.WithUserToken(token) }

// WithTempTokenAuth configures temporary token authentication. See [client.WithTempTokenAuth].
//
// Deprecated: see [client.WithTempTokenAuth].
func WithTempTokenAuth(opts ...auth.TempTokenOption) Option { return client.WithTempTokenAuth(opts...) }

// WithTempTokens configures temporary token authentication with a map of tokens. See [client.WithTempTokens].
func WithTempTokens(tokens map[string]string) Option { return client.WithTempTokens(tokens) }

// WithSSOTokenAuth configures SSO (SAML) token authentication. See [client.WithSSOTokenAuth].
func WithSSOTokenAuth(samlToken string, opts ...auth.SSOTokenOption) Option {
	return client.WithSSOTokenAuth(samlToken, opts...)
}

// WithTicketAuth configures ticket authentication using username/password. See [client.WithTicketAuth].
func WithTicketAuth(username, password string, opts ...auth.TicketOption) Option {
	return client.WithTicketAuth(username, password, opts...)
}

// WithTicket configures authentication using a pre-existing ticket. See [client.WithTicket].
func WithTicket(ticket string) Option { return client.WithTicket(ticket) }

// WithMaxRetries sets how many times a request is attempted in all, counting the first (default 3). See [client.WithMaxRetries].
func WithMaxRetries(n int) Option { return client.WithMaxRetries(n) }

// WithRetryDelay sets the initial delay between retries. See [client.WithRetryDelay].
func WithRetryDelay(d time.Duration) Option { return client.WithRetryDelay(d) }

// WithMaxRetryDelay sets the maximum delay between retries. See [client.WithMaxRetryDelay].
func WithMaxRetryDelay(d time.Duration) Option { return client.WithMaxRetryDelay(d) }

// WithBackoffMultiplier sets the exponential backoff multiplier. See [client.WithBackoffMultiplier].
func WithBackoffMultiplier(m float64) Option { return client.WithBackoffMultiplier(m) }

// WithTimeout sets the request timeout. See [client.WithTimeout].
func WithTimeout(d time.Duration) Option { return client.WithTimeout(d) }

// WithMaxIdleConns sets the maximum number of idle connections across all hosts. See [client.WithMaxIdleConns].
func WithMaxIdleConns(n int) Option { return client.WithMaxIdleConns(n) }

// WithMaxIdleConnsPerHost sets maximum idle connections to QuickBase (default 6). See [client.WithMaxIdleConnsPerHost].
func WithMaxIdleConnsPerHost(n int) Option { return client.WithMaxIdleConnsPerHost(n) }

// WithIdleConnTimeout sets how long idle connections stay in the pool. See [client.WithIdleConnTimeout].
func WithIdleConnTimeout(d time.Duration) Option { return client.WithIdleConnTimeout(d) }

// WithProactiveThrottle enables sliding window throttling. See [client.WithProactiveThrottle].
func WithProactiveThrottle(requestsPer10Seconds int) Option {
	return client.WithProactiveThrottle(requestsPer10Seconds)
}

// WithThrottle sets a custom throttle implementation. See [client.WithThrottle].
func WithThrottle(t Throttle) Option { return client.WithThrottle(t) }

// WithDebug enables debug logging. See [client.WithDebug].
func WithDebug(enabled bool) Option { return client.WithDebug(enabled) }

// WithConvertDates has no effect: the client never converted dates. See [client.WithConvertDates].
//
// Deprecated: see [client.WithConvertDates].
func WithConvertDates(enabled bool) Option { return client.WithConvertDates(enabled) }

// WithOnRateLimit sets a callback for rate limit events. See [client.WithOnRateLimit].
func WithOnRateLimit(callback func(RateLimitInfo)) Option { return client.WithOnRateLimit(callback) }

// WithOnRequest sets a callback that fires after every API request completes. See [client.WithOnRequest].
func WithOnRequest(callback func(RequestInfo)) Option { return client.WithOnRequest(callback) }

// WithOnRetry sets a callback that fires before each retry attempt. See [client.WithOnRetry].
func WithOnRetry(callback func(RetryInfo)) Option { return client.WithOnRetry(callback) }

// WithBaseURL sets a custom base URL. See [client.WithBaseURL].
func WithBaseURL(url string) Option { return client.WithBaseURL(url) }

// WithReadOnly enables read-only mode, blocking all write operations. See [client.WithReadOnly].
func WithReadOnly() Option { return client.WithReadOnly() }

// WithLineErrorsAsErrors makes Upsert return a [LineErrorsError] when QuickBase rejects values. See [client.WithLineErrorsAsErrors].
func WithLineErrorsAsErrors() Option { return client.WithLineErrorsAsErrors() }

// WithAppToken sets an application token for XML API calls. See [client.WithAppToken].
func WithAppToken(token string) Option { return client.WithAppToken(token) }

// WithSchema sets the schema for table and field aliases. See [client.WithSchema].
func WithSchema(schema *Schema) Option { return client.WithSchema(schema) }

// WithSchemaOptions sets the schema with custom options. See [client.WithSchemaOptions].
func WithSchemaOptions(schema *Schema, opts SchemaOptions) Option {
	return client.WithSchemaOptions(schema, opts)
}

// New creates a new QuickBase client. One auth option is required:
// WithUserToken, WithTempTokens, WithSSOTokenAuth, WithTicketAuth or WithTicket.
func New(realm string, opts ...Option) (*Client, error) {
	if err := client.ValidateRealm(realm); err != nil {
		return nil, err
	}
	return client.New(realm, nil, opts...)
}

// Error represents a QuickBase SDK error.
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// Helper functions re-exported from core
var (
	// IsRetryableError returns true if the error should trigger a retry.
	IsRetryableError = core.IsRetryableError

	// ParseErrorResponse parses an HTTP response into an appropriate error type.
	ParseErrorResponse = core.ParseErrorResponse

	// IsISODateString checks if a string looks like an ISO 8601 date.
	IsISODateString = core.IsISODateString

	// ParseISODate parses an ISO 8601 date string to time.Time.
	ParseISODate = core.ParseISODate

	// DateOf returns t's calendar day in t's location, for a Date field.
	DateOf = core.DateOf

	// DateTimeOf returns t as a Date/Time field's value.
	DateTimeOf = core.DateTimeOf

	// ParseDate parses a Date field's value ("2024-01-15").
	ParseDate = core.ParseDate

	// ParseDateTime parses a Date/Time field's value ("2024-01-15T15:30:00Z").
	ParseDateTime = core.ParseDateTime

	// TransformDates recursively transforms ISO date strings to time.Time in a map.
	TransformDates = core.TransformDates
)

// NewSlidingWindowThrottle creates a new sliding window throttle.
func NewSlidingWindowThrottle(requestsPer10Seconds int) *SlidingWindowThrottle {
	return client.NewSlidingWindowThrottle(requestsPer10Seconds)
}

// NewNoOpThrottle creates a no-op throttle.
func NewNoOpThrottle() *NoOpThrottle {
	return client.NewNoOpThrottle()
}

// NewSchema creates a new SchemaBuilder for fluent schema definition.
//
// Example:
//
//	schema := quickbase.NewSchema().
//	    Table("projects", "bqxyz123").
//	        Field("recordId", 3).
//	        Field("name", 6).
//	        Field("status", 7).
//	    Table("tasks", "bqabc456").
//	        Field("recordId", 3).
//	        Field("title", 6).
//	    Build()
func NewSchema() *SchemaBuilder {
	return core.NewSchema()
}

// DefaultSchemaOptions returns the default schema options.
// Records are keyed by field ID (TransformResponses is off).
func DefaultSchemaOptions() SchemaOptions {
	return core.DefaultSchemaOptions()
}

// Pagination helper functions re-exported from client
var (
	// DetectPaginationType determines the pagination type from metadata.
	DetectPaginationType = client.DetectPaginationType

	// HasMorePages checks if a response has more pages available.
	HasMorePages = client.HasMorePages
)

// --- Friendly Type Aliases ---
// These provide cleaner names for the verbose oapi-codegen generated types.

// Request body types
type (
	// RunQueryBody is the request body for RunQuery
	RunQueryBody = generated.RunQueryJSONRequestBody

	// UpsertBody is the request body for Upsert (insert/update records)
	UpsertBody = generated.UpsertJSONRequestBody

	// DeleteRecordsBody is the request body for DeleteRecords
	DeleteRecordsBody = generated.DeleteRecordsJSONRequestBody

	// CreateAppBody is the request body for CreateApp
	CreateAppBody = generated.CreateAppJSONRequestBody

	// UpdateAppBody is the request body for UpdateApp
	UpdateAppBody = generated.UpdateAppJSONRequestBody

	// DeleteAppBody is the request body for DeleteApp
	DeleteAppBody = generated.DeleteAppJSONRequestBody

	// CopyAppBody is the request body for CopyApp
	CopyAppBody = generated.CopyAppJSONRequestBody

	// CreateTableBody is the request body for CreateTable
	CreateTableBody = generated.CreateTableJSONRequestBody

	// UpdateTableBody is the request body for UpdateTable
	UpdateTableBody = generated.UpdateTableJSONRequestBody

	// CreateFieldBody is the request body for CreateField
	CreateFieldBody = generated.CreateFieldJSONRequestBody

	// UpdateFieldBody is the request body for UpdateField
	UpdateFieldBody = generated.UpdateFieldJSONRequestBody

	// DeleteFieldsBody is the request body for DeleteFields
	DeleteFieldsBody = generated.DeleteFieldsJSONRequestBody

	// CreateRelationshipBody is the request body for CreateRelationship
	CreateRelationshipBody = generated.CreateRelationshipJSONRequestBody

	// UpdateRelationshipBody is the request body for UpdateRelationship
	UpdateRelationshipBody = generated.UpdateRelationshipJSONRequestBody

	// RunFormulaBody is the request body for RunFormula
	RunFormulaBody = generated.RunFormulaJSONRequestBody

	// GetUsersBody is the request body for GetUsers
	GetUsersBody = generated.GetUsersJSONRequestBody

	// AuditBody is the request body for Audit (get audit logs)
	AuditBody = generated.AuditJSONRequestBody
)

// Query options
type (
	// QueryOptions contains pagination and other options for RunQuery
	QueryOptions = generated.RunQueryJSONBody_Options
)

// Param types
type ()

// Core types
type (
	// Record is a QuickBase record (map of field ID to value)
	Record = generated.QuickbaseRecord
)

// FieldType is the type of a field for CreateField
type FieldType = generated.CreateFieldJSONBodyFieldType

// Field type constants
const (
	FieldTypeText      FieldType = "text"
	FieldTypeMultiText FieldType = "text-multi-line"
	FieldTypeRichText  FieldType = "rich-text"
	FieldTypeNumber    FieldType = "numeric"
	FieldTypeCurrency  FieldType = "currency"
	FieldTypePercent   FieldType = "percent"
	FieldTypeRating    FieldType = "rating"
	FieldTypeDate      FieldType = "date"
	FieldTypeDateTime  FieldType = "datetime"
	FieldTypeTimeOfDay FieldType = "timeofday"
	FieldTypeDuration  FieldType = "duration"
	FieldTypeCheckbox  FieldType = "checkbox"
	FieldTypeEmail     FieldType = "email"
	FieldTypePhone     FieldType = "phone"
	FieldTypeURL       FieldType = "url"
	FieldTypeAddress   FieldType = "address"
	FieldTypeFile      FieldType = "file"
	FieldTypeUser      FieldType = "user"
	FieldTypeMultiUser FieldType = "multiuser"
)

// --- Helper Functions ---

// Ptr returns a pointer to the given value.
// Useful for optional fields that require pointers.
//
// Example:
//
//	body := quickbase.RunQueryBody{
//	    From:  tableId,
//	    Where: quickbase.Ptr("{6.GT.100}"),
//	}
func Ptr[T any](v T) *T {
	return &v
}

// --- v2.0 Raw-First Helpers ---
// These opt-in helpers make working with raw generated types more convenient.

// UnwrapRecords converts QuickbaseRecord slices to []map[string]any.
// This extracts the actual field values from FieldValue wrappers.
//
// Example:
//
//	resp, _ := client.RunQuery(ctx, body)
//	if resp.JSON200.Data != nil {
//	    records := quickbase.UnwrapRecords(*resp.JSON200.Data)
//	    for _, rec := range records {
//	        name := rec["6"] // access by field ID
//	    }
//	}
var UnwrapRecords = client.UnwrapRecords

// UnwrapRecord converts a single QuickbaseRecord to map[string]any.
var UnwrapRecord = client.UnwrapRecord

// Deref returns the value of a pointer, or zero value if nil.
// Use this to safely access optional (pointer) fields in generated types.
//
// Example:
//
//	resp, _ := client.GetApp(appId).Run(ctx)
//	name := quickbase.Deref(resp.JSON200.Name) // string, not *string
func Deref[T any](ptr *T) T {
	return client.Deref(ptr)
}

// DerefOr returns the value of a pointer, or the default if nil.
//
// Example:
//
//	resp, _ := client.GetApp(appId).Run(ctx)
//	name := quickbase.DerefOr(resp.JSON200.Name, "Unknown")
func DerefOr[T any](ptr *T, defaultVal T) T {
	return client.DerefOr(ptr, defaultVal)
}

// Ints returns a pointer to a slice of ints.
// Useful for the Select field in RunQuery.
//
// Example:
//
//	body := quickbase.RunQueryBody{
//	    From:   tableId,
//	    Select: quickbase.Ints(3, 6, 7),
//	}
func Ints(ids ...int) *[]int {
	return &ids
}

// Strings returns a pointer to a slice of strings.
func Strings(strs ...string) *[]string {
	return &strs
}

// Value creates a FieldValue for use in record upserts.
// It accepts string, int, float, bool, []string, [Date] (a Date field) and
// [DateTime] (a Date/Time field); nil clears the field, and any other value is
// sent as its JSON (a file attachment's {"fileName", "data"}, for example).
//
// A time.Time panics: it could go to a Date field or a Date/Time field, which
// accept different forms (a Date field only "2024-01-15"), and QuickBase saves
// the record with the field blank when the form is wrong. Wrap it with
// [DateOf] or [DateTimeOf].
//
// FieldValue{Value: v} is the same, without the time.Time check.
//
// Example:
//
//	data := []quickbase.Record{
//	    {
//	        "name":   quickbase.Value("Alice"),
//	        "age":    quickbase.Value(30),
//	        "active": quickbase.Value(true),
//	        "tags":   quickbase.Value([]string{"a", "b"}),
//	    },
//	}
//	client.Upsert(ctx, quickbase.UpsertBody{
//	    To:   "projects",
//	    Data: &data,
//	})
func Value(v any) FieldValue {
	if _, ok := v.(time.Time); ok {
		panic("quickbase.Value: a time.Time could go to a Date field or a Date/Time field, which accept " +
			"different forms; use quickbase.Value(quickbase.DateOf(t)) or quickbase.Value(quickbase.DateTimeOf(t))")
	}
	// Date and DateTime marshal as their fields expect; nil clears the field
	return FieldValue{Value: v}
}

// Row creates a Record from alternating key-value pairs.
// Keys can be field IDs (int) or field aliases (string).
// Values are automatically wrapped using Value().
//
// This provides a concise way to create records for upserts without
// manually wrapping each value.
//
// Example:
//
//	data := []quickbase.Record{
//	    quickbase.Row("name", "Alice", "age", 30, "active", true),
//	    quickbase.Row("name", "Bob", "age", 25, "active", false),
//	}
//	client.Upsert(ctx, quickbase.UpsertBody{
//	    To:   "projects",
//	    Data: &data,
//	})
//
// With numeric field IDs:
//
//	quickbase.Row(6, "Alice", 7, 30)
func Row(pairs ...any) Record {
	record := make(Record)
	for i := 0; i < len(pairs)-1; i += 2 {
		key := pairs[i]
		val := pairs[i+1]

		// Convert key to string
		var keyStr string
		switch k := key.(type) {
		case string:
			keyStr = k
		case int:
			keyStr = strconv.Itoa(k)
		default:
			continue // Skip invalid keys
		}

		record[keyStr] = Value(val)
	}
	return record
}

// --- Sorting Helpers ---

// Sort order constants for use with Sort().
const (
	ASC  = generated.SortFieldOrderASC
	DESC = generated.SortFieldOrderDESC
)

// Sort creates a SortField for use in sortBy arrays.
// The order should be ASC or DESC.
//
// Example:
//
//	quickbase.Sort(6, quickbase.ASC)
//	quickbase.Sort(7, quickbase.DESC)
func Sort(fieldId int, order generated.SortFieldOrder) SortField {
	return SortField{
		FieldId: fieldId,
		Order:   order,
	}
}

// SortBy creates a sortBy parameter for RunQuery from SortField values.
// This wraps the fields in the union type required by the API.
//
// Example:
//
//	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:   tableId,
//	    Select: quickbase.Ints(3, 6, 7),
//	    SortBy: quickbase.SortBy(quickbase.Asc(6), quickbase.Desc(7)),
//	})
func SortBy(fields ...SortField) *SortByUnion {
	sortBy, err := client.SortFieldsToSortByUnion(fields)
	if err != nil {
		return nil
	}
	return sortBy
}

// --- Where Helper ---

// WhereUnion is the union type for where (query string or []int record IDs)
type WhereUnion = generated.RunQueryJSONBody_Where

// Where creates a where parameter for RunQuery from a query string.
// This wraps the string in the union type required by the API.
//
// Example:
//
//	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:   tableId,
//	    Where:  quickbase.Where("{6.EX.'Active'}"),
//	})
func Where(query string) *WhereUnion {
	where, err := client.StringToWhereUnion(query)
	if err != nil {
		return nil
	}
	return where
}

// DeleteWhereUnion is the union type for DeleteRecords where clause
type DeleteWhereUnion = generated.DeleteRecordsJSONBody_Where

// DeleteWhere creates a where parameter for DeleteRecords from a query string.
// This wraps the string in the union type required by the API.
//
// Example:
//
//	result, _ := client.DeleteRecords(ctx, tableId, quickbase.DeleteWhere("{6.EX.'Deleted'}"))
func DeleteWhere(query string) DeleteWhereUnion {
	where, err := client.StringToDeleteWhereUnion(query)
	if err != nil {
		return DeleteWhereUnion{}
	}
	return where
}

// --- Query Options Helper ---

// Options creates a QueryOptions with top (limit) and skip (offset).
// Pass -1 for either value to omit it.
//
// Example:
//
//	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:    tableId,
//	    Options: quickbase.Options(100, 0),   // top=100, skip=0
//	})
//
//	// Skip only:
//	quickbase.Options(-1, 50)  // skip=50, no top limit
func Options(top, skip int) *QueryOptions {
	opts := &QueryOptions{}
	if top >= 0 {
		opts.Top = &top
	}
	if skip >= 0 {
		opts.Skip = &skip
	}
	return opts
}

// --- GroupBy Helper ---

// GroupByItem is an alias for the verbose generated type.
type GroupByItem = generated.RunQueryJSONBody_GroupBy_Item

// GroupBy creates a groupBy array for RunQuery from field IDs.
//
// Example:
//
//	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:    tableId,
//	    GroupBy: quickbase.GroupBy(6, 7),
//	})
func GroupBy(fieldIds ...int) *[]GroupByItem {
	items := make([]GroupByItem, len(fieldIds))
	for i, id := range fieldIds {
		fid := id // avoid closure issue
		items[i] = GroupByItem{
			FieldId: &fid,
		}
	}
	return &items
}

// --- Schema Resolution Helpers ---

// Schema resolution functions re-exported from core
var (
	// ResolveSchema builds lookup maps from a schema definition.
	ResolveSchema = core.ResolveSchema

	// ResolveSchemaWithOptions builds lookup maps with custom options.
	ResolveSchemaWithOptions = core.ResolveSchemaWithOptions

	// ResolveTableAlias resolves a table alias to its ID.
	ResolveTableAlias = core.ResolveTableAlias

	// ResolveFieldAlias resolves a field alias to its ID.
	ResolveFieldAlias = core.ResolveFieldAlias

	// GetTableAlias returns the alias for a table ID.
	GetTableAlias = core.GetTableAlias

	// GetFieldAlias returns the alias for a field ID.
	GetFieldAlias = core.GetFieldAlias
)

// Fields resolves field aliases to IDs for use in Select arrays.
// This allows using readable field names instead of numeric IDs.
//
// Example:
//
//	schema := quickbase.NewSchema().
//	    Table("projects", "bqxyz123").
//	        Field("recordId", 3).
//	        Field("name", 6).
//	        Field("status", 7).
//	    Build()
//
//	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:   "projects",
//	    Select: quickbase.Fields(schema, "projects", "recordId", "name", "status"),
//	    Where:  quickbase.Ptr("{'status'.EX.'Active'}"),
//	})
//
// Returns nil if schema is nil or if any alias cannot be resolved.
// For error details, use ResolveFieldAlias directly.
func Fields(schema *Schema, table string, aliases ...string) *[]int {
	if schema == nil {
		return nil
	}

	resolved := core.ResolveSchema(schema)
	tableID, err := core.ResolveTableAlias(resolved, table)
	if err != nil {
		return nil
	}

	ids := make([]int, len(aliases))
	for i, alias := range aliases {
		id, err := core.ResolveFieldAlias(resolved, tableID, alias)
		if err != nil {
			return nil
		}
		ids[i] = id
	}
	return &ids
}
