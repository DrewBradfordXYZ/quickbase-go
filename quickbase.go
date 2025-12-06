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
	"fmt"
	"strconv"
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
	MissingTokenError   = core.MissingTokenError
	RateLimitInfo       = core.RateLimitInfo

	// Schema types
	Schema         = core.Schema
	TableSchema    = core.TableSchema
	ResolvedSchema = core.ResolvedSchema
	SchemaError    = core.SchemaError

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

// ticketMarker is used for ticket (XML API) authentication.
// XML-API-TICKET: Remove this struct if XML API is discontinued.
type ticketMarker struct {
	username string
	password string
	opts     []auth.TicketOption
}

// WithTempTokenAuth configures temporary token authentication.
//
// Temp tokens are short-lived (~5 min), table-scoped tokens that verify a user
// is logged into QuickBase. Go servers receive these tokens from browser clients
// (e.g., Code Pages) that can fetch them using the user's browser session.
//
// For the simpler map-based API, see [WithTempTokens].
//
// Deprecated: Use [WithTempTokens] instead for clearer token-to-table mapping.
func WithTempTokenAuth(opts ...auth.TempTokenOption) Option {
	return func(c *clientConfig) {
		c.authStrategy = &tempTokenMarker{opts: opts}
	}
}

// WithTempTokens configures temporary token authentication with a map of tokens.
//
// This is the preferred way to configure temp token auth when you have tokens
// mapped to table IDs (dbids).
//
// Example:
//
//	client, err := quickbase.New("myrealm",
//	    quickbase.WithTempTokens(map[string]string{
//	        "bqxyz123": tokenForTable1,
//	        "bqabc456": tokenForTable2,
//	    }),
//	)
func WithTempTokens(tokens map[string]string) Option {
	return func(c *clientConfig) {
		c.authStrategy = &tempTokenMarker{opts: []auth.TempTokenOption{auth.WithTempTokens(tokens)}}
	}
}

// WithSSOTokenAuth configures SSO (SAML) token authentication.
//
// SSO authentication lets your Go server make API calls as a specific QuickBase
// user rather than a shared service account. The SDK exchanges a SAML assertion
// for a QuickBase temp token using RFC 8693 token exchange.
//
// Benefits:
//   - Audit accuracy: "Created By" and "Modified By" show the actual user
//   - Security: No long-lived user token; each user gets a short-lived token
//   - Per-user permissions: API calls respect each user's individual QuickBase permissions
//
// Prerequisites:
//   - Your QuickBase realm has SAML SSO configured
//   - Your identity provider (Okta, Azure AD, etc.) can generate SAML assertions
//
// Example:
//
//	// Get SAML assertion from your IdP for the authenticated user
//	samlAssertion := getAssertionFromIdP(userId) // base64url-encoded
//
//	client, err := quickbase.New("myrealm",
//	    quickbase.WithSSOTokenAuth(samlAssertion),
//	)
//
//	// API calls are now made as that specific user
//
// See https://developer.quickbase.com/operation/exchangeSsoToken
func WithSSOTokenAuth(samlToken string, opts ...auth.SSOTokenOption) Option {
	return func(c *clientConfig) {
		c.authStrategy = &ssoTokenMarker{samlToken: samlToken, opts: opts}
	}
}

// WithTicketAuth configures ticket authentication using username/password.
//
// This calls the XML API_Authenticate endpoint to obtain a ticket, which is then
// used with REST API calls. Unlike user tokens, tickets properly attribute record
// changes (createdBy/modifiedBy) to the authenticated user.
//
// The password is used once for authentication and then discarded from memory.
// When the ticket expires (default 12 hours), an AuthenticationError is returned
// and a new client must be created with fresh credentials.
//
// Example:
//
//	qb, err := quickbase.New("myrealm",
//	    quickbase.WithTicketAuth("user@example.com", "password"),
//	)
//
// With custom ticket validity (max ~6 months):
//
//	qb, err := quickbase.New("myrealm",
//	    quickbase.WithTicketAuth("user@example.com", "password",
//	        auth.WithTicketHours(24*7), // 1 week
//	    ),
//	)
//
// XML-API-TICKET: Remove this function if XML API is discontinued.
func WithTicketAuth(username, password string, opts ...auth.TicketOption) Option {
	return func(c *clientConfig) {
		c.authStrategy = &ticketMarker{username: username, password: password, opts: opts}
	}
}

// WithTicket configures authentication using a pre-existing ticket.
//
// Use this when the ticket was obtained elsewhere (e.g., by a browser client
// calling API_Authenticate directly). The server never sees user credentials.
//
// This is the recommended approach for Code Page cookie mode:
//  1. Browser calls QuickBase API_Authenticate with user credentials
//  2. Browser sends ticket to Go server
//  3. Server stores encrypted ticket in HttpOnly cookie
//  4. Server uses WithTicket for API calls
//
// Example:
//
//	qb, err := quickbase.New("myrealm",
//	    quickbase.WithTicket(ticketFromCookie),
//	)
//
// XML-API-TICKET: Remove this function if XML API is discontinued.
func WithTicket(ticket string) Option {
	return func(c *clientConfig) {
		c.authStrategy = auth.NewExistingTicketStrategy(ticket)
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

// WithOnRequest sets a callback that fires after every API request completes.
// Use this for monitoring request latency, status codes, and errors.
//
// Example:
//
//	quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
//	    log.Printf("%s %s → %d (%v)", info.Method, info.Path, info.StatusCode, info.Duration)
//	})
func WithOnRequest(callback func(RequestInfo)) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithOnRequest(callback))
	}
}

// WithOnRetry sets a callback that fires before each retry attempt.
// Use this for monitoring retry behavior and debugging transient failures.
//
// Example:
//
//	quickbase.WithOnRetry(func(info quickbase.RetryInfo) {
//	    log.Printf("Retrying %s %s (attempt %d, reason: %s)", info.Method, info.Path, info.Attempt, info.Reason)
//	})
func WithOnRetry(callback func(RetryInfo)) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithOnRetry(callback))
	}
}

// WithBaseURL sets a custom base URL.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithBaseURL(url))
	}
}

// WithSchema sets the schema for table and field aliases.
//
// When configured, the client automatically:
//   - Transforms table aliases to IDs in requests (from, to fields)
//   - Transforms field aliases to IDs in requests (select, sortBy, groupBy, where, data)
//   - Transforms field IDs to aliases in responses
//   - Unwraps { value: X } to just X in response records
//
// Example:
//
//	schema := &quickbase.Schema{
//	    Tables: map[string]quickbase.TableSchema{
//	        "projects": {
//	            ID: "bqw3ryzab",
//	            Fields: map[string]int{
//	                "id":     3,
//	                "name":   6,
//	                "status": 7,
//	            },
//	        },
//	    },
//	}
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithUserToken("token"),
//	    quickbase.WithSchema(schema),
//	)
//
//	// Now use aliases in queries
//	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:   "projects",                    // alias instead of "bqw3ryzab"
//	    Select: quickbase.Ints(6, 7),          // can also use aliases in wrapper
//	    Where:  quickbase.Ptr("{'status'.EX.'Active'}"),
//	})
func WithSchema(schema *Schema) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithSchema(schema))
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
	case *ticketMarker: // XML-API-TICKET: Remove this case if XML API is discontinued.
		authStrategy = auth.NewTicketStrategy(s.username, s.password, realm, s.opts...)
	case auth.Strategy:
		authStrategy = s
	case nil:
		return nil, &Error{Message: "no authentication strategy configured; use WithUserToken, WithTempTokenAuth, WithSSOTokenAuth, or WithTicketAuth"}
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

// Value creates a FieldValue for use in record upserts.
// It accepts string, int, float, bool, or []string values.
//
// This helper hides the complexity of Go's lack of union types.
// The QuickBase API allows field values to be different types (text, number,
// boolean, multi-select), and oapi-codegen generates verbose wrapper types
// to handle this. Value() provides a clean interface.
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
	var fv generated.FieldValue_Value
	switch val := v.(type) {
	case string:
		fv.FromFieldValueValue0(val)
	case int:
		fv.FromFieldValueValue1(float32(val))
	case int32:
		fv.FromFieldValueValue1(float32(val))
	case int64:
		fv.FromFieldValueValue1(float32(val))
	case float32:
		fv.FromFieldValueValue1(val)
	case float64:
		fv.FromFieldValueValue1(float32(val))
	case bool:
		fv.FromFieldValueValue2(val)
	case []string:
		fv.FromFieldValueValue3(val)
	}
	return FieldValue{Value: fv}
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
