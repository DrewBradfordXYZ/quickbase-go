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
// Example with struct:
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
// Example with builder:
//
//	schema := quickbase.NewSchema().
//	    Table("projects", "bqw3ryzab").
//	        Field("id", 3).
//	        Field("name", 6).
//	        Field("status", 7).
//	    Build()
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithUserToken("token"),
//	    quickbase.WithSchema(schema),
//	)
//
//	// Now use aliases in queries
//	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
//	    From:   "projects",                                            // alias instead of "bqw3ryzab"
//	    Select: quickbase.Fields(schema, "projects", "name", "status"), // aliases for select
//	    Where:  quickbase.Ptr("{'status'.EX.'Active'}"),                // aliases in where
//	})
func WithSchema(schema *Schema) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithSchema(schema))
	}
}

// WithSchemaOptions sets the schema with custom options.
// Use this to control schema behavior, such as disabling response transformation.
//
// Example:
//
//	quickbase.WithSchemaOptions(schema, quickbase.SchemaOptions{
//	    TransformResponses: false, // Keep field IDs in responses
//	})
func WithSchemaOptions(schema *Schema, opts SchemaOptions) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithSchemaOptions(schema, opts))
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
// Response transformation is enabled by default.
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

	// SortFieldOrder is the sort order (ASC, DESC)
	SortFieldOrder = generated.SortFieldOrder

	// SortByUnion is the union type for sortBy ([]SortField or false)
	SortByUnion = generated.SortByUnion
)

// Result types from wrapper methods
type (
	// RunQueryResult contains the result of a RunQuery call
	RunQueryResult = client.RunQueryResult

	// RunReportResult contains the result of a RunReport call
	RunReportResult = client.RunReportResult

	// UpsertResult contains the result of an Upsert call
	UpsertResult = client.UpsertResult

	// DeleteRecordsResult contains the result of a DeleteRecords call
	DeleteRecordsResult = client.DeleteRecordsResult

	// GetAppResult contains the result of GetApp, CreateApp, UpdateApp, CopyApp
	GetAppResult = client.GetAppResult

	// TableInfo contains the result of GetTable, CreateTable, UpdateTable, GetAppTables
	TableInfo = client.TableInfo

	// ReportInfo contains the result of GetReport, GetTableReports
	ReportInfo = client.ReportInfo

	// CreateFieldResult contains the result of CreateField, UpdateField, GetField
	CreateFieldResult = client.CreateFieldResult

	// DeleteFieldsResult contains the result of DeleteFields
	DeleteFieldsResult = client.DeleteFieldsResult

	// DeleteAppResult contains the result of DeleteApp
	DeleteAppResult = client.DeleteAppResult

	// DeleteTableResult contains the result of DeleteTable
	DeleteTableResult = client.DeleteTableResult

	// DeleteFileResult contains the result of DeleteFile
	DeleteFileResult = client.DeleteFileResult

	// RelationshipInfo contains the result of CreateRelationship, UpdateRelationship
	RelationshipInfo = client.RelationshipInfo

	// FormulaResult contains the result of RunFormula
	FormulaResult = client.FormulaResult

	// SchemaFieldInfo contains comprehensive field information for schema discovery
	SchemaFieldInfo = client.SchemaFieldInfo

	// FieldPermission represents a role's permission on a field
	FieldPermission = client.FieldPermission

	// RoleInfo contains basic role information extracted from field permissions
	RoleInfo = client.RoleInfo

	// GetFieldsResult wraps the getFields response with helper methods
	GetFieldsResult = client.GetFieldsResult

	// FieldInfo contains metadata about a field in query results
	FieldInfo = client.FieldInfo

	// QueryMetadata contains pagination metadata from a query
	QueryMetadata = client.QueryMetadata
)

// Builder types for fluent API
// These are auto-generated from the OpenAPI spec and provide a chainable API
// for building and executing API requests. Method names match QuickBase operation IDs.
type (
	// AddManagersToGroupBuilder provides a fluent API for adding managers to groups.
	AddManagersToGroupBuilder = client.AddManagersToGroupBuilder

	// AddMembersToGroupBuilder provides a fluent API for adding members to groups.
	AddMembersToGroupBuilder = client.AddMembersToGroupBuilder

	// AddSubgroupsToGroupBuilder provides a fluent API for adding subgroups to groups.
	AddSubgroupsToGroupBuilder = client.AddSubgroupsToGroupBuilder

	// AuditBuilder provides a fluent API for querying audit logs.
	AuditBuilder = client.AuditBuilder

	// ChangesetSolutionBuilder provides a fluent API for modifying solutions.
	ChangesetSolutionBuilder = client.ChangesetSolutionBuilder

	// ChangesetSolutionFromRecordBuilder provides a fluent API for solution changesets from records.
	ChangesetSolutionFromRecordBuilder = client.ChangesetSolutionFromRecordBuilder

	// CloneUserTokenBuilder provides a fluent API for cloning user tokens.
	CloneUserTokenBuilder = client.CloneUserTokenBuilder

	// CopyAppBuilder provides a fluent API for copying apps.
	CopyAppBuilder = client.CopyAppBuilder

	// CreateAppBuilder provides a fluent API for creating apps.
	CreateAppBuilder = client.CreateAppBuilder

	// CreateFieldBuilder provides a fluent API for creating fields.
	CreateFieldBuilder = client.CreateFieldBuilder

	// CreateRelationshipBuilder provides a fluent API for creating relationships.
	CreateRelationshipBuilder = client.CreateRelationshipBuilder

	// CreateSolutionBuilder provides a fluent API for creating solutions.
	CreateSolutionBuilder = client.CreateSolutionBuilder

	// CreateSolutionFromRecordBuilder provides a fluent API for creating solutions from records.
	CreateSolutionFromRecordBuilder = client.CreateSolutionFromRecordBuilder

	// CreateTableBuilder provides a fluent API for creating tables.
	CreateTableBuilder = client.CreateTableBuilder

	// DeactivateUserTokenBuilder provides a fluent API for deactivating user tokens.
	DeactivateUserTokenBuilder = client.DeactivateUserTokenBuilder

	// DeleteAppBuilder provides a fluent API for deleting apps.
	DeleteAppBuilder = client.DeleteAppBuilder

	// DeleteFieldsBuilder provides a fluent API for deleting fields.
	DeleteFieldsBuilder = client.DeleteFieldsBuilder

	// DeleteFileBuilder provides a fluent API for deleting files.
	DeleteFileBuilder = client.DeleteFileBuilder

	// DeleteRelationshipBuilder provides a fluent API for deleting relationships.
	DeleteRelationshipBuilder = client.DeleteRelationshipBuilder

	// DeleteTableBuilder provides a fluent API for deleting tables.
	DeleteTableBuilder = client.DeleteTableBuilder

	// DeleteUserTokenBuilder provides a fluent API for deleting user tokens.
	DeleteUserTokenBuilder = client.DeleteUserTokenBuilder

	// DenyUsersBuilder provides a fluent API for denying users.
	DenyUsersBuilder = client.DenyUsersBuilder

	// DenyUsersAndGroupsBuilder provides a fluent API for denying users and groups.
	DenyUsersAndGroupsBuilder = client.DenyUsersAndGroupsBuilder

	// DownloadFileBuilder provides a fluent API for downloading files.
	DownloadFileBuilder = client.DownloadFileBuilder

	// ExchangeSsoTokenBuilder provides a fluent API for exchanging SSO tokens.
	ExchangeSsoTokenBuilder = client.ExchangeSsoTokenBuilder

	// ExportSolutionBuilder provides a fluent API for exporting solutions.
	ExportSolutionBuilder = client.ExportSolutionBuilder

	// ExportSolutionToRecordBuilder provides a fluent API for exporting solutions to records.
	ExportSolutionToRecordBuilder = client.ExportSolutionToRecordBuilder

	// GenerateDocumentBuilder provides a fluent API for generating documents.
	GenerateDocumentBuilder = client.GenerateDocumentBuilder

	// GetAppEventsBuilder provides a fluent API for getting app events.
	GetAppEventsBuilder = client.GetAppEventsBuilder

	// GetAppTablesBuilder provides a fluent API for getting app tables.
	GetAppTablesBuilder = client.GetAppTablesBuilder

	// GetFieldBuilder provides a fluent API for getting a field.
	GetFieldBuilder = client.GetFieldBuilder

	// GetFieldUsageBuilder provides a fluent API for getting field usage.
	GetFieldUsageBuilder = client.GetFieldUsageBuilder

	// GetFieldsUsageBuilder provides a fluent API for getting fields usage.
	GetFieldsUsageBuilder = client.GetFieldsUsageBuilder

	// GetRelationshipsBuilder provides a fluent API for getting relationships.
	GetRelationshipsBuilder = client.GetRelationshipsBuilder

	// GetReportBuilder provides a fluent API for getting a report.
	GetReportBuilder = client.GetReportBuilder

	// GetTableBuilder provides a fluent API for getting a table.
	GetTableBuilder = client.GetTableBuilder

	// GetTableReportsBuilder provides a fluent API for getting table reports.
	GetTableReportsBuilder = client.GetTableReportsBuilder

	// GetTempTokenDBIDBuilder provides a fluent API for getting temp tokens.
	GetTempTokenDBIDBuilder = client.GetTempTokenDBIDBuilder

	// GetUsersBuilder provides a fluent API for getting users.
	GetUsersBuilder = client.GetUsersBuilder

	// PlatformAnalyticEventSummariesBuilder provides a fluent API for platform analytics.
	PlatformAnalyticEventSummariesBuilder = client.PlatformAnalyticEventSummariesBuilder

	// PlatformAnalyticReadsBuilder provides a fluent API for platform analytic reads.
	PlatformAnalyticReadsBuilder = client.PlatformAnalyticReadsBuilder

	// RemoveManagersFromGroupBuilder provides a fluent API for removing managers from groups.
	RemoveManagersFromGroupBuilder = client.RemoveManagersFromGroupBuilder

	// RemoveMembersFromGroupBuilder provides a fluent API for removing members from groups.
	RemoveMembersFromGroupBuilder = client.RemoveMembersFromGroupBuilder

	// RemoveSubgroupsFromGroupBuilder provides a fluent API for removing subgroups from groups.
	RemoveSubgroupsFromGroupBuilder = client.RemoveSubgroupsFromGroupBuilder

	// RunFormulaBuilder provides a fluent API for running formulas.
	RunFormulaBuilder = client.RunFormulaBuilder

	// RunReportBuilder provides a fluent API for running reports.
	RunReportBuilder = client.RunReportBuilder

	// TransferUserTokenBuilder provides a fluent API for transferring user tokens.
	TransferUserTokenBuilder = client.TransferUserTokenBuilder

	// UndenyUsersBuilder provides a fluent API for undenying users.
	UndenyUsersBuilder = client.UndenyUsersBuilder

	// UpdateAppBuilder provides a fluent API for updating apps.
	UpdateAppBuilder = client.UpdateAppBuilder

	// UpdateFieldBuilder provides a fluent API for updating fields.
	UpdateFieldBuilder = client.UpdateFieldBuilder

	// UpdateRelationshipBuilder provides a fluent API for updating relationships.
	UpdateRelationshipBuilder = client.UpdateRelationshipBuilder

	// UpdateSolutionBuilder provides a fluent API for updating solutions.
	UpdateSolutionBuilder = client.UpdateSolutionBuilder

	// UpdateSolutionToRecordBuilder provides a fluent API for updating solutions to records.
	UpdateSolutionToRecordBuilder = client.UpdateSolutionToRecordBuilder

	// UpdateTableBuilder provides a fluent API for updating tables.
	UpdateTableBuilder = client.UpdateTableBuilder

	// SortSpec specifies a sort field and order for RunQuery.
	SortSpec = client.SortSpec
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

// Asc creates a SortSpec for ascending order.
// Accepts field ID (int) or alias (string) when used with schema.
//
// Example:
//
//	quickbase.Asc(6)       // Sort by field 6 ascending
//	quickbase.Asc("name")  // Sort by "name" alias ascending (with schema)
func Asc(field any) SortSpec {
	return SortSpec{
		Field: field,
		Order: generated.SortFieldOrderASC,
	}
}

// Desc creates a SortSpec for descending order.
// Accepts field ID (int) or alias (string) when used with schema.
//
// Example:
//
//	quickbase.Desc(6)        // Sort by field 6 descending
//	quickbase.Desc("dueDate") // Sort by "dueDate" alias descending (with schema)
func Desc(field any) SortSpec {
	return SortSpec{
		Field: field,
		Order: generated.SortFieldOrderDESC,
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
	var sortBy SortByUnion
	_ = sortBy.FromSortByUnion0(fields)
	return &sortBy
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
