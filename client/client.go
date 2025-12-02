// Package client provides a QuickBase API client with retry and rate limiting.
//
// This package is typically used through the top-level quickbase package,
// but can be used directly for advanced use cases like custom pagination.
//
// Basic usage:
//
//	client, err := client.New(realm, authStrategy)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	resp, err := client.API().GetAppWithResponse(ctx, appId)
//
// With options:
//
//	client, err := client.New(realm, authStrategy,
//	    client.WithMaxRetries(5),
//	    client.WithTimeout(60 * time.Second),
//	    client.WithProactiveThrottle(100),
//	)
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

// Client wraps the generated QuickBase client with authentication, automatic retry,
// and rate limiting. It provides a robust interface for making QuickBase API requests.
//
// The Client automatically:
//   - Adds authentication headers to all requests
//   - Retries failed requests with exponential backoff and jitter
//   - Handles rate limiting (429) responses with automatic retry
//   - Refreshes expired tokens for temp token and SSO authentication
//   - Applies proactive throttling to avoid hitting rate limits
//
// Access the underlying generated API client via the API() method.
type Client struct {
	generated *generated.ClientWithResponses
	auth      auth.Strategy
	realm     string
	baseURL   string

	// Retry configuration
	maxRetries     int
	initialDelay   time.Duration
	maxDelay       time.Duration
	backoffMult    float64

	// Request timeout
	timeout time.Duration

	// HTTP transport settings
	maxIdleConns        int
	maxIdleConnsPerHost int
	idleConnTimeout     time.Duration

	// Rate limiting / throttling
	throttle Throttle

	// Logging
	logger *core.Logger

	// Date conversion
	convertDates bool

	// Callbacks
	onRateLimit func(core.RateLimitInfo)
}

// Option configures a Client.
type Option func(*Client)

// WithMaxRetries sets the maximum number of retry attempts (default 3).
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithRetryDelay sets the initial delay between retries (default 1s).
func WithRetryDelay(d time.Duration) Option {
	return func(c *Client) {
		c.initialDelay = d
	}
}

// WithMaxRetryDelay sets the maximum delay between retries (default 30s).
func WithMaxRetryDelay(d time.Duration) Option {
	return func(c *Client) {
		c.maxDelay = d
	}
}

// WithBackoffMultiplier sets the exponential backoff multiplier (default 2).
func WithBackoffMultiplier(m float64) Option {
	return func(c *Client) {
		c.backoffMult = m
	}
}

// WithTimeout sets the request timeout (default 30s).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = d
	}
}

// WithMaxIdleConns sets the maximum number of idle connections across all hosts (default 100).
// This controls total connection pool size.
func WithMaxIdleConns(n int) Option {
	return func(c *Client) {
		c.maxIdleConns = n
	}
}

// WithMaxIdleConnsPerHost sets maximum idle connections per host (default 2).
// For high-throughput QuickBase usage, consider setting this to 10-20.
// This is the most impactful setting for concurrent request performance.
func WithMaxIdleConnsPerHost(n int) Option {
	return func(c *Client) {
		c.maxIdleConnsPerHost = n
	}
}

// WithIdleConnTimeout sets how long idle connections stay in the pool (default 90s).
func WithIdleConnTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.idleConnTimeout = d
	}
}

// WithThrottle sets a custom throttle.
func WithThrottle(t Throttle) Option {
	return func(c *Client) {
		c.throttle = t
	}
}

// WithProactiveThrottle enables sliding window throttling (100 req/10s by default).
func WithProactiveThrottle(requestsPer10Seconds int) Option {
	return func(c *Client) {
		c.throttle = NewSlidingWindowThrottle(requestsPer10Seconds)
	}
}

// WithDebug enables debug logging.
func WithDebug(enabled bool) Option {
	return func(c *Client) {
		c.logger = core.NewLogger(enabled)
	}
}

// WithConvertDates enables/disables automatic ISO date string conversion (default true).
func WithConvertDates(enabled bool) Option {
	return func(c *Client) {
		c.convertDates = enabled
	}
}

// WithOnRateLimit sets a callback for rate limit events.
func WithOnRateLimit(callback func(core.RateLimitInfo)) Option {
	return func(c *Client) {
		c.onRateLimit = callback
	}
}

// WithBaseURL sets a custom base URL (default https://api.quickbase.com/v1).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// New creates a new QuickBase client.
func New(realm string, authStrategy auth.Strategy, opts ...Option) (*Client, error) {
	c := &Client{
		auth:         authStrategy,
		realm:        realm,
		baseURL:      "https://api.quickbase.com/v1",
		maxRetries:   3,
		initialDelay: time.Second,
		maxDelay:     30 * time.Second,
		backoffMult:  2,
		timeout:      30 * time.Second,
		logger:       core.NewLogger(false),
		convertDates: true,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Create throttle if not provided (disabled by default, like JS SDK)
	if c.throttle == nil {
		c.throttle = NewNoOpThrottle()
	}

	// Create HTTP transport with connection pool settings
	transport := &http.Transport{
		MaxIdleConns:        100, // default
		MaxIdleConnsPerHost: 2,   // default (Go's default is too low for high-throughput)
		IdleConnTimeout:     90 * time.Second,
	}
	if c.maxIdleConns > 0 {
		transport.MaxIdleConns = c.maxIdleConns
	}
	if c.maxIdleConnsPerHost > 0 {
		transport.MaxIdleConnsPerHost = c.maxIdleConnsPerHost
	}
	if c.idleConnTimeout > 0 {
		transport.IdleConnTimeout = c.idleConnTimeout
	}

	// Create the generated client with our custom HTTP doer
	httpClient := &authHTTPClient{
		client: c,
		httpClient: &http.Client{
			Timeout:   c.timeout,
			Transport: transport,
		},
	}

	genClient, err := generated.NewClientWithResponses(
		c.baseURL,
		generated.WithHTTPClient(httpClient),
		generated.WithRequestEditorFn(c.addHeaders),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	c.generated = genClient
	return c, nil
}

// addHeaders adds required QuickBase headers to each request.
func (c *Client) addHeaders(ctx context.Context, req *http.Request) error {
	req.Header.Set("QB-Realm-Hostname", c.realm+".quickbase.com")
	req.Header.Set("Content-Type", "application/json")
	return nil
}

// API returns the generated API client for making requests.
//
// The returned client provides strongly-typed methods for all QuickBase API endpoints.
// Each method has two variants:
//   - Method(ctx, params) - returns (*http.Response, error)
//   - MethodWithResponse(ctx, params) - returns parsed response with JSON200, JSON400, etc. fields
//
// Example:
//
//	resp, err := client.API().GetAppWithResponse(ctx, appId)
//	if err != nil {
//	    return err
//	}
//	if resp.JSON200 != nil {
//	    fmt.Println("App name:", resp.JSON200.Name)
//	}
func (c *Client) API() *generated.ClientWithResponses {
	return c.generated
}

// Logger returns the client's logger.
func (c *Client) Logger() *core.Logger {
	return c.logger
}

// authHTTPClient wraps http.Client to add auth, retry, and rate limiting.
type authHTTPClient struct {
	client     *Client
	httpClient *http.Client
}

func (h *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	c := h.client
	startTime := time.Now()

	// Extract dbid from request for temp token auth
	dbid := extractDBID(req)

	// Read body once for potential retries
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		req.Body.Close()

		// Also check body for dbid if not found elsewhere
		if dbid == "" {
			dbid = extractDBIDFromBody(bodyBytes)
		}
	}

	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Throttling
		if err := c.throttle.Acquire(ctx); err != nil {
			return nil, fmt.Errorf("throttle: %w", err)
		}

		// Get auth token
		token, err := c.auth.GetToken(ctx, dbid)
		if err != nil {
			return nil, fmt.Errorf("getting auth token: %w", err)
		}

		// Clone request for retry
		reqCopy := req.Clone(ctx)
		if bodyBytes != nil {
			reqCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Apply auth
		c.auth.ApplyAuth(reqCopy, token)

		// Make request
		reqStartTime := time.Now()
		resp, err := h.httpClient.Do(reqCopy)
		duration := time.Since(reqStartTime)

		if err != nil {
			lastErr = err

			// Check for timeout
			if ctx.Err() != nil {
				return nil, core.NewTimeoutError(int(c.timeout.Milliseconds()))
			}

			if attempt < c.maxRetries {
				delay := h.calculateBackoff(attempt)
				c.logger.Retry(attempt, c.maxRetries, delay, "network error")
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		c.logger.Timing(req.Method, req.URL.String(), duration)

		// Handle 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()

			info := core.RateLimitInfo{
				Timestamp:  time.Now(),
				RequestURL: req.URL.String(),
				HTTPStatus: 429,
				CFRay:      resp.Header.Get("cf-ray"),
				TID:        resp.Header.Get("tid"),
				QBAPIRay:   resp.Header.Get("qb-api-ray"),
				Attempt:    attempt,
			}

			if ra := resp.Header.Get("Retry-After"); ra != "" {
				info.RetryAfter, _ = strconv.Atoi(ra)
			}

			c.logger.RateLimit(info)

			// Notify callback
			if c.onRateLimit != nil {
				c.onRateLimit(info)
			}

			if attempt < c.maxRetries {
				var delay time.Duration
				if info.RetryAfter > 0 {
					delay = time.Duration(info.RetryAfter) * time.Second
				} else {
					delay = h.calculateBackoff(attempt)
				}
				c.logger.Retry(attempt, c.maxRetries, delay, "rate limited (429)")
				time.Sleep(delay)
				continue
			}

			return nil, core.NewRateLimitError(info, "")
		}

		// Handle 401 Unauthorized - try to refresh token
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			newToken, err := c.auth.HandleAuthError(ctx, resp.StatusCode, dbid, attempt, c.maxRetries)
			if err != nil {
				return nil, err
			}
			if newToken != "" {
				c.logger.Debug("Token refreshed, retrying request")
				continue
			}
		}

		// Handle 5xx server errors with retry
		if resp.StatusCode >= 500 && attempt < c.maxRetries {
			resp.Body.Close()
			delay := h.calculateBackoff(attempt)
			c.logger.Retry(attempt, c.maxRetries, delay, fmt.Sprintf("server error (%d)", resp.StatusCode))
			time.Sleep(delay)
			continue
		}

		lastResp = resp
		lastErr = nil

		c.logger.Timing(req.Method, req.URL.String(), time.Since(startTime))
		break
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return lastResp, nil
}

// calculateBackoff calculates exponential backoff with jitter.
func (h *authHTTPClient) calculateBackoff(attempt int) time.Duration {
	c := h.client
	delay := float64(c.initialDelay) * math.Pow(c.backoffMult, float64(attempt-1))

	// Add jitter: ±10%
	jitter := delay * 0.1 * (rand.Float64()*2 - 1)
	delay += jitter

	if delay > float64(c.maxDelay) {
		delay = float64(c.maxDelay)
	}

	return time.Duration(delay)
}

// Path patterns for extracting dbid
var (
	tableIDPattern = regexp.MustCompile(`/tables/([^/?]+)`)
	appIDPattern   = regexp.MustCompile(`/apps/([^/?]+)`)
)

// extractDBID extracts the table/app ID from a request for temp token auth.
func extractDBID(req *http.Request) string {
	// 1. Check query params for tableId
	if dbid := req.URL.Query().Get("tableId"); dbid != "" {
		return dbid
	}

	// 2. Check query params for appId
	if dbid := req.URL.Query().Get("appId"); dbid != "" {
		return dbid
	}

	// 3. Check path for tableId (e.g., /v1/tables/{tableId}/...)
	if matches := tableIDPattern.FindStringSubmatch(req.URL.Path); len(matches) > 1 {
		return matches[1]
	}

	// 4. Check path for appId (e.g., /v1/apps/{appId}/...)
	if matches := appIDPattern.FindStringSubmatch(req.URL.Path); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// extractDBIDFromBody extracts dbid from request body JSON.
func extractDBIDFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}

	// Check for 'from' field (runQuery, deleteRecords)
	if from, ok := data["from"].(string); ok && from != "" {
		return from
	}

	// Check for 'to' field (upsert)
	if to, ok := data["to"].(string); ok && to != "" {
		return to
	}

	return ""
}

// ValidateRealm validates the realm format.
//
// The realm should be just the subdomain portion, not the full hostname.
// For example, use "mycompany" not "mycompany.quickbase.com".
func ValidateRealm(realm string) error {
	if realm == "" {
		return fmt.Errorf("realm is required")
	}
	if strings.Contains(realm, ".") {
		return fmt.Errorf("realm should be just the subdomain (e.g., \"mycompany\" not \"mycompany.quickbase.com\")")
	}
	return nil
}
