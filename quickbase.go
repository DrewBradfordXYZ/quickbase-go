// Package quickbase provides a Go SDK for the QuickBase API.
//
// This SDK provides:
//   - Multiple authentication strategies (user token, temp token, SSO)
//   - Automatic retry with exponential backoff and jitter
//   - Proactive rate limiting with sliding window throttle
//   - Custom error types for different HTTP status codes
//   - Debug logging
//   - Date transformation (ISO strings to time.Time)
//
// Basic usage with user token:
//
//	qb, err := quickbase.New("your-realm", quickbase.WithUserToken("your-token"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	resp, err := qb.API().GetAppWithResponse(ctx, &generated.GetAppParams{
//	    AppId: "your-app-id",
//	})
//
// With proactive rate limiting:
//
//	qb, err := quickbase.New("your-realm",
//	    quickbase.WithUserToken("token"),
//	    quickbase.WithProactiveThrottle(100), // 100 req/10s (QuickBase's limit)
//	)
//
// With debug logging:
//
//	qb, err := quickbase.New("your-realm",
//	    quickbase.WithUserToken("token"),
//	    quickbase.WithDebug(true),
//	)
//
// With rate limit callback:
//
//	qb, err := quickbase.New("your-realm",
//	    quickbase.WithUserToken("token"),
//	    quickbase.WithOnRateLimit(func(info core.RateLimitInfo) {
//	        log.Printf("Rate limited! Retry after %ds", info.RetryAfter)
//	    }),
//	)
package quickbase

import (
	"fmt"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/client"
	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

// Client is the main QuickBase API client.
type Client = client.Client

// Re-export types for convenience
type (
	// Generated client types
	ClientWithResponses = generated.ClientWithResponses

	// Error types
	QuickbaseError      = core.QuickbaseError
	RateLimitError      = core.RateLimitError
	AuthenticationError = core.AuthenticationError
	AuthorizationError  = core.AuthorizationError
	NotFoundError       = core.NotFoundError
	ValidationError     = core.ValidationError
	TimeoutError        = core.TimeoutError
	ServerError         = core.ServerError
	RateLimitInfo       = core.RateLimitInfo

	// Throttle types
	SlidingWindowThrottle = client.SlidingWindowThrottle
	NoOpThrottle          = client.NoOpThrottle
	Throttle              = client.Throttle

	// Pagination types
	PaginationMetadata = client.PaginationMetadata
	PaginationOptions  = client.PaginationOptions
	PaginationType     = client.PaginationType
)

// Pagination type constants
const (
	PaginationTypeSkip  = client.PaginationTypeSkip
	PaginationTypeToken = client.PaginationTypeToken
	PaginationTypeNone  = client.PaginationTypeNone
)

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	authStrategy any // Can be auth.Strategy or a marker type
	clientOpts   []client.Option
	realm        string
}

// WithUserToken configures user token authentication.
func WithUserToken(token string) Option {
	return func(c *clientConfig) {
		c.authStrategy = auth.NewUserTokenStrategy(token)
	}
}

// tempTokenMarker and ssoTokenMarker are used to identify auth strategy type
type tempTokenMarker struct {
	opts []auth.TempTokenOption
}

type ssoTokenMarker struct {
	samlToken string
	opts      []auth.SSOTokenOption
}

// WithTempTokenAuth configures temporary token authentication with options.
func WithTempTokenAuth(opts ...auth.TempTokenOption) Option {
	return func(c *clientConfig) {
		c.authStrategy = &tempTokenMarker{opts: opts}
	}
}

// WithSSOTokenAuth configures SSO token authentication.
func WithSSOTokenAuth(samlToken string, opts ...auth.SSOTokenOption) Option {
	return func(c *clientConfig) {
		c.authStrategy = &ssoTokenMarker{samlToken: samlToken, opts: opts}
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithMaxRetries(n))
	}
}

// WithRetryDelay sets the initial delay between retries.
func WithRetryDelay(d time.Duration) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithRetryDelay(d))
	}
}

// WithMaxRetryDelay sets the maximum delay between retries.
func WithMaxRetryDelay(d time.Duration) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithMaxRetryDelay(d))
	}
}

// WithBackoffMultiplier sets the exponential backoff multiplier.
func WithBackoffMultiplier(m float64) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithBackoffMultiplier(m))
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithTimeout(d))
	}
}

// WithMaxIdleConns sets the maximum number of idle connections across all hosts.
// Default is 100. This controls total connection pool size.
func WithMaxIdleConns(n int) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithMaxIdleConns(n))
	}
}

// WithMaxIdleConnsPerHost sets maximum idle connections to QuickBase (default 6).
// The default of 6 matches browser standards and handles typical concurrency.
// For heavy batch operations, consider 10-20 alongside WithProactiveThrottle.
func WithMaxIdleConnsPerHost(n int) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithMaxIdleConnsPerHost(n))
	}
}

// WithIdleConnTimeout sets how long idle connections stay in the pool.
// Default is 90 seconds.
func WithIdleConnTimeout(d time.Duration) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithIdleConnTimeout(d))
	}
}

// WithProactiveThrottle enables sliding window throttling.
// QuickBase's limit is 100 requests per 10 seconds per user token.
func WithProactiveThrottle(requestsPer10Seconds int) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithProactiveThrottle(requestsPer10Seconds))
	}
}

// WithThrottle sets a custom throttle implementation.
func WithThrottle(t client.Throttle) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithThrottle(t))
	}
}

// WithDebug enables debug logging.
func WithDebug(enabled bool) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithDebug(enabled))
	}
}

// WithConvertDates enables/disables automatic ISO date string conversion.
func WithConvertDates(enabled bool) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithConvertDates(enabled))
	}
}

// WithOnRateLimit sets a callback for rate limit events.
func WithOnRateLimit(callback func(RateLimitInfo)) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithOnRateLimit(callback))
	}
}

// WithBaseURL sets a custom base URL.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithBaseURL(url))
	}
}

// New creates a new QuickBase client.
func New(realm string, opts ...Option) (*Client, error) {
	// Validate realm
	if err := client.ValidateRealm(realm); err != nil {
		return nil, err
	}

	cfg := &clientConfig{realm: realm}
	for _, opt := range opts {
		opt(cfg)
	}

	// Resolve auth strategy
	var authStrategy auth.Strategy
	switch s := cfg.authStrategy.(type) {
	case *tempTokenMarker:
		authStrategy = auth.NewTempTokenStrategy(realm, s.opts...)
	case *ssoTokenMarker:
		authStrategy = auth.NewSSOTokenStrategy(s.samlToken, realm, s.opts...)
	case auth.Strategy:
		authStrategy = s
	case nil:
		return nil, &Error{Message: "no authentication strategy configured; use WithUserToken, WithTempTokenAuth, or WithSSOTokenAuth"}
	default:
		return nil, &Error{Message: fmt.Sprintf("unknown auth strategy type: %T", cfg.authStrategy)}
	}

	return client.New(realm, authStrategy, cfg.clientOpts...)
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

	// RunReportBody is the request body for RunReport
	RunReportBody = generated.RunReportJSONRequestBody

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
type (
	// GetFieldsParams are the parameters for GetFields
	GetFieldsParams = generated.GetFieldsParams

	// GetAppTablesParams are the parameters for GetAppTables
	GetAppTablesParams = generated.GetAppTablesParams

	// GetTableParams are the parameters for GetTable
	GetTableParams = generated.GetTableParams

	// DeleteTableParams are the parameters for DeleteTable
	DeleteTableParams = generated.DeleteTableParams

	// CreateTableParams are the parameters for CreateTable
	CreateTableParams = generated.CreateTableParams

	// UpdateTableParams are the parameters for UpdateTable
	UpdateTableParams = generated.UpdateTableParams

	// GetFieldParams are the parameters for GetField
	GetFieldParams = generated.GetFieldParams

	// CreateFieldParams are the parameters for CreateField
	CreateFieldParams = generated.CreateFieldParams

	// UpdateFieldParams are the parameters for UpdateField
	UpdateFieldParams = generated.UpdateFieldParams

	// DeleteFieldsParams are the parameters for DeleteFields
	DeleteFieldsParams = generated.DeleteFieldsParams

	// GetRelationshipsParams are the parameters for GetRelationships
	GetRelationshipsParams = generated.GetRelationshipsParams

	// GetReportParams are the parameters for GetReport
	GetReportParams = generated.GetReportParams

	// GetTableReportsParams are the parameters for GetTableReports
	GetTableReportsParams = generated.GetTableReportsParams

	// RunReportParams are the parameters for RunReport
	RunReportParams = generated.RunReportParams

	// GetUsersParams are the parameters for GetUsers
	GetUsersParams = generated.GetUsersParams
)

// Core types
type (
	// Record is a QuickBase record (map of field ID to value)
	Record = generated.QuickbaseRecord

	// FieldValue is a field value in a record
	FieldValue = generated.FieldValue

	// FieldValueUnion is the union type for field values
	FieldValueUnion = generated.FieldValue_Value

	// SortField specifies a field to sort by
	SortField = generated.SortField
)

// Result types from wrapper methods
type (
	// RunQueryResult contains the result of a RunQuery call
	RunQueryResult = client.RunQueryResult

	// UpsertResult contains the result of an Upsert call
	UpsertResult = client.UpsertResult

	// DeleteRecordsResult contains the result of a DeleteRecords call
	DeleteRecordsResult = client.DeleteRecordsResult

	// GetAppResult contains the result of a GetApp call
	GetAppResult = client.GetAppResult

	// FieldDetails contains information about a field
	FieldDetails = client.FieldDetails

	// FieldInfo contains metadata about a field in query results
	FieldInfo = client.FieldInfo

	// QueryMetadata contains pagination metadata from a query
	QueryMetadata = client.QueryMetadata
)

// FieldType is the type of a field for CreateField
type FieldType = generated.CreateFieldJSONBodyFieldType

// Field type constants
const (
	FieldTypeText          FieldType = "text"
	FieldTypeMultiText     FieldType = "text-multi-line"
	FieldTypeRichText      FieldType = "rich-text"
	FieldTypeNumber        FieldType = "numeric"
	FieldTypeCurrency      FieldType = "currency"
	FieldTypePercent       FieldType = "percent"
	FieldTypeRating        FieldType = "rating"
	FieldTypeDate          FieldType = "date"
	FieldTypeDateTime      FieldType = "datetime"
	FieldTypeTimeOfDay     FieldType = "timeofday"
	FieldTypeDuration      FieldType = "duration"
	FieldTypeCheckbox      FieldType = "checkbox"
	FieldTypeEmail         FieldType = "email"
	FieldTypePhone         FieldType = "phone"
	FieldTypeURL           FieldType = "url"
	FieldTypeAddress       FieldType = "address"
	FieldTypeFile          FieldType = "file"
	FieldTypeUser          FieldType = "user"
	FieldTypeMultiUser     FieldType = "multiuser"
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
