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

// WithTempToken configures temporary token authentication.
func WithTempToken(opts ...auth.TempTokenOption) Option {
	return func(c *clientConfig) {
		// Realm will be set when New() is called
		c.authStrategy = nil // Marker to create temp token strategy with realm
	}
}

// WithSSOToken configures SSO token authentication.
func WithSSOToken(samlToken string, opts ...auth.SSOTokenOption) Option {
	return func(c *clientConfig) {
		// Realm will be set when New() is called
		c.authStrategy = nil // Marker to create SSO strategy with realm
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

// Note: The generic pagination functions (Paginate, CollectAll, CollectN,
// NewPaginatedRequest) must be accessed via the client package directly:
//
//     import "github.com/DrewBradfordXYZ/quickbase-go/client"
//
//     // Iterator pattern
//     for record, err := range client.Paginate(ctx, fetcher) { ... }
//
//     // Collect all
//     records, err := client.CollectAll(ctx, fetcher)
//
//     // Collect with limit
//     records, err := client.CollectN(ctx, fetcher, 100)
//
//     // Fluent API
//     req := client.NewPaginatedRequest(ctx, fetcher, autoPaginate)
//     result, err := req.All()
//     result, err := req.Paginate(client.PaginationOptions{Limit: 500})
//     result, err := req.NoPaginate()
