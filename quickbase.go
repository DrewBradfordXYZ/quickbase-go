// Package quickbase provides a Go SDK for the QuickBase API.
//
// This SDK provides:
//   - Multiple authentication strategies (user token, temp token, SSO)
//   - Automatic retry with exponential backoff
//   - Rate limiting to avoid 429 errors
//   - Automatic pagination with iterators
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
// Using temp tokens (browser session):
//
//	qb, err := quickbase.New("your-realm", quickbase.WithTempToken())
//
// Using SSO:
//
//	qb, err := quickbase.New("your-realm", quickbase.WithSSOToken("saml-assertion"))
package quickbase

import (
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/client"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

// Client is the main QuickBase API client.
type Client = client.Client

// Re-export generated types for convenience
type (
	// API types are available through the generated package
	ClientWithResponses = generated.ClientWithResponses
)

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	authStrategy auth.Strategy
	clientOpts   []client.Option
}

// WithUserToken configures user token authentication.
func WithUserToken(token string) Option {
	return func(c *clientConfig) {
		c.authStrategy = auth.NewUserTokenStrategy(token)
	}
}

// WithTempToken configures temporary token authentication.
// Use opts to customize token caching behavior.
func WithTempToken(opts ...auth.TempTokenOption) Option {
	return func(c *clientConfig) {
		// Note: realm is set later when New() is called
		c.authStrategy = nil // Will be set in New()
	}
}

// tempTokenOptsKey is used to pass temp token options through clientConfig
type tempTokenOptsHolder struct {
	opts []auth.TempTokenOption
}

// WithTempTokenOpts configures temporary token authentication with options.
func WithTempTokenOpts(realm string, opts ...auth.TempTokenOption) Option {
	return func(c *clientConfig) {
		c.authStrategy = auth.NewTempTokenStrategy(realm, opts...)
	}
}

// WithSSOToken configures SSO token authentication.
func WithSSOToken(samlToken string) Option {
	return func(c *clientConfig) {
		// Note: realm is set later when New() is called
		c.authStrategy = nil // Will be set in New()
	}
}

// ssoTokenHolder is used to pass SSO token through clientConfig
type ssoTokenHolder struct {
	samlToken string
}

// WithSSOTokenOpts configures SSO token authentication with options.
func WithSSOTokenOpts(samlToken, realm string, opts ...auth.SSOTokenOption) Option {
	return func(c *clientConfig) {
		c.authStrategy = auth.NewSSOTokenStrategy(samlToken, realm, opts...)
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithMaxRetries(n))
	}
}

// WithRetryDelay sets the base delay between retries.
func WithRetryDelay(d time.Duration) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithRetryDelay(d))
	}
}

// WithRateLimiter sets a custom rate limiter.
func WithRateLimiter(rate float64, burst int) Option {
	return func(c *clientConfig) {
		c.clientOpts = append(c.clientOpts, client.WithRateLimiter(client.NewRateLimiter(rate, burst)))
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
	cfg := &clientConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.authStrategy == nil {
		// Default to user token if no auth strategy specified
		return nil, ErrNoAuthStrategy
	}

	return client.New(realm, cfg.authStrategy, cfg.clientOpts...)
}

// ErrNoAuthStrategy is returned when no authentication strategy is configured.
var ErrNoAuthStrategy = &Error{Message: "no authentication strategy configured; use WithUserToken, WithTempTokenOpts, or WithSSOTokenOpts"}

// Error represents a QuickBase SDK error.
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}
