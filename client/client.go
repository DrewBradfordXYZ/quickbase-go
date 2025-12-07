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
//	app, err := client.GetApp(ctx, appId)
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
//   - Transforms table/field aliases to IDs (if schema configured)
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

	// Schema for table/field aliases
	schema *core.ResolvedSchema

	// Callbacks
	onRateLimit func(core.RateLimitInfo)
	onRequest   func(RequestInfo)
	onRetry     func(RetryInfo)

	// Transport for cleanup
	transport *http.Transport

	// Read-only mode blocks all write operations
	readOnly bool
}

// RequestInfo contains information about a completed API request.
// This is passed to the OnRequest callback after each request completes.
type RequestInfo struct {
	Method      string        // HTTP method (GET, POST, etc.)
	Path        string        // URL path (e.g., /v1/apps/bqxyz123)
	StatusCode  int           // HTTP status code
	Duration    time.Duration // Total request duration
	Attempt     int           // Attempt number (1 = first try, 2+ = retries)
	Error       error         // Non-nil if request failed
	RequestBody []byte        // Request body bytes (for debugging failed requests)
}

// RetryInfo contains information about a retry attempt.
// This is passed to the OnRetry callback before each retry.
type RetryInfo struct {
	Method     string        // HTTP method
	Path       string        // URL path
	Attempt    int           // Which attempt is about to happen (2 = first retry)
	Reason     string        // Why we're retrying (e.g., "429", "503", "timeout")
	WaitTime   time.Duration // How long we'll wait before retrying
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

// WithMaxIdleConnsPerHost sets maximum idle connections per host (default 6).
// The default of 6 matches browser standards and handles typical concurrency.
// For heavy batch operations, consider 10-20 alongside WithProactiveThrottle.
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

// WithOnRequest sets a callback that fires after every API request completes.
// Use this for monitoring request latency, status codes, and errors.
//
// Example:
//
//	quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
//	    log.Printf("%s %s → %d (%v)", info.Method, info.Path, info.StatusCode, info.Duration)
//	})
func WithOnRequest(callback func(RequestInfo)) Option {
	return func(c *Client) {
		c.onRequest = callback
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
	return func(c *Client) {
		c.onRetry = callback
	}
}

// WithBaseURL sets a custom base URL (default https://api.quickbase.com/v1).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithReadOnly enables read-only mode, blocking all write operations.
//
// When enabled, any attempt to make a write request (POST, PUT, DELETE, PATCH
// for JSON API, or write actions for XML API) returns a [core.ReadOnlyError].
//
// This is useful for MCP servers or other contexts where you want to ensure
// the client can only read data, never modify it.
//
// Example:
//
//	client, _ := quickbase.New(realm,
//	    quickbase.WithUserToken(token),
//	    quickbase.WithReadOnly(),
//	)
//
//	// These work:
//	app, _ := client.GetApp(appId).Run(ctx)
//	fields, _ := client.GetFields(tableId).Run(ctx)
//
//	// These fail with ReadOnlyError:
//	_, err := client.Upsert(tableId).Data(records).Run(ctx)
//	// err = &core.ReadOnlyError{Method: "POST", Path: "/v1/records"}
func WithReadOnly() Option {
	return func(c *Client) {
		c.readOnly = true
	}
}

// WithSchema sets the schema for table and field aliases.
// When configured, the client automatically:
//   - Transforms table aliases to IDs in requests (from, to fields)
//   - Transforms field aliases to IDs in requests (select, sortBy, groupBy, where, data)
//   - Transforms field IDs to aliases in responses (unless disabled via WithSchemaOptions)
//   - Unwraps { value: X } to just X in response records
func WithSchema(schema *core.Schema) Option {
	return func(c *Client) {
		c.schema = core.ResolveSchema(schema)
	}
}

// WithSchemaOptions sets the schema with custom options.
// Use this to control schema behavior, such as disabling response transformation.
//
// Example:
//
//	quickbase.WithSchemaOptions(schema, core.SchemaOptions{
//	    TransformResponses: false, // Keep field IDs in responses
//	})
func WithSchemaOptions(schema *core.Schema, opts core.SchemaOptions) Option {
	return func(c *Client) {
		c.schema = core.ResolveSchemaWithOptions(schema, opts)
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
	// Go's default MaxIdleConnsPerHost (2) is based on obsolete RFC 2616 (1999).
	// We default to 6, matching browser standards, for reasonable concurrency.
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 6,
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
	c.transport = transport
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

// Schema returns the resolved schema, if configured.
func (c *Client) Schema() *core.ResolvedSchema {
	return c.schema
}

// Realm returns the QuickBase realm name.
func (c *Client) Realm() string {
	return c.realm
}

// DoXML makes an XML API request to the legacy QuickBase XML API.
//
// This method is used by the xml sub-package to call legacy XML API endpoints
// that have no JSON API equivalent (e.g., API_GetRoleInfo, API_GetSchema).
// It uses the client's existing auth, retry, and throttling infrastructure.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - dbid: The database/table ID to call (e.g., app ID or table ID)
//   - action: The XML API action (e.g., "API_GetRoleInfo", "API_GetSchema")
//   - body: The XML request body (without the outer <qdbapi> tags, which are added automatically)
//
// Returns the raw XML response body.
//
// Deprecated: This method supports legacy XML API endpoints. Use JSON API methods
// where possible. This will be removed when QuickBase discontinues the XML API.
func (c *Client) DoXML(ctx context.Context, dbid, action string, body []byte) ([]byte, error) {
	// Check read-only mode for XML write actions
	if c.readOnly && isXMLWriteAction(action) {
		return nil, core.NewReadOnlyError(http.MethodPost, "/db/"+dbid, action)
	}

	url := fmt.Sprintf("https://%s.quickbase.com/db/%s", c.realm, dbid)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating XML request: %w", err)
	}

	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("QUICKBASE-ACTION", action)

	// Use the same retry/throttle logic as JSON API
	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
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
		reqCopy.Body = io.NopCloser(bytes.NewReader(body))

		// Apply auth
		c.auth.ApplyAuth(reqCopy, token)

		// Make request using the transport directly (not authHTTPClient to avoid double-auth)
		httpClient := &http.Client{
			Timeout:   c.timeout,
			Transport: c.transport,
		}
		resp, err := httpClient.Do(reqCopy)
		if err != nil {
			lastErr = err
			if attempt < c.maxRetries {
				delay := c.calculateXMLBackoff(attempt)
				c.logger.Retry(attempt, c.maxRetries, delay, "network error")
				time.Sleep(delay)
				continue
			}
			return nil, err
		}
		defer resp.Body.Close()

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading XML response: %w", err)
		}

		// Handle 429 rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt < c.maxRetries {
				delay := c.calculateXMLBackoff(attempt)
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if seconds, err := strconv.Atoi(ra); err == nil {
						delay = time.Duration(seconds) * time.Second
					}
				}
				c.logger.Retry(attempt, c.maxRetries, delay, "rate limited (429)")
				time.Sleep(delay)
				continue
			}
			return nil, fmt.Errorf("rate limited after %d attempts", c.maxRetries)
		}

		// Handle 5xx server errors
		if resp.StatusCode >= 500 && attempt < c.maxRetries {
			delay := c.calculateXMLBackoff(attempt)
			c.logger.Retry(attempt, c.maxRetries, delay, fmt.Sprintf("server error (%d)", resp.StatusCode))
			time.Sleep(delay)
			continue
		}

		return respBody, nil
	}

	return nil, lastErr
}

// calculateXMLBackoff calculates exponential backoff with jitter for XML requests.
func (c *Client) calculateXMLBackoff(attempt int) time.Duration {
	delay := float64(c.initialDelay) * math.Pow(c.backoffMult, float64(attempt-1))
	jitter := delay * 0.1 * (rand.Float64()*2 - 1)
	delay += jitter
	if delay > float64(c.maxDelay) {
		delay = float64(c.maxDelay)
	}
	return time.Duration(delay)
}

// Close closes idle connections and releases resources.
// After calling Close, the client should not be used.
func (c *Client) Close() {
	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}
}

// SignOut clears credentials from memory if the auth strategy supports it.
//
// Currently only [auth.TicketStrategy] supports signing out. For other strategies
// (user token, temp token, SSO), this method returns false and has no effect.
//
// After signing out, API calls will fail with an authentication error.
// Create a new client with fresh credentials to continue making API calls.
//
// This does NOT invalidate tokens on QuickBase's servers - tickets remain
// valid until they expire. This only clears credentials from local memory.
//
// Returns true if sign out was performed, false if the strategy doesn't support it.
func (c *Client) SignOut() bool {
	if signOuter, ok := c.auth.(auth.SignOuter); ok {
		signOuter.SignOut()
		return true
	}
	return false
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

	// Check read-only mode before making request
	if err := c.checkReadOnly(req); err != nil {
		return nil, err
	}

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

			// Notify onRequest callback (with error)
			if c.onRequest != nil {
				c.onRequest(RequestInfo{
					Method:      req.Method,
					Path:        req.URL.Path,
					StatusCode:  0,
					Duration:    duration,
					Attempt:     attempt,
					Error:       err,
					RequestBody: bodyBytes,
				})
			}

			// Check for timeout
			if ctx.Err() != nil {
				return nil, core.NewTimeoutError(int(c.timeout.Milliseconds()))
			}

			if attempt < c.maxRetries {
				delay := h.calculateBackoff(attempt)
				c.logger.Retry(attempt, c.maxRetries, delay, "network error")

				// Notify onRetry callback
				if c.onRetry != nil {
					c.onRetry(RetryInfo{
						Method:   req.Method,
						Path:     req.URL.Path,
						Attempt:  attempt + 1,
						Reason:   "network error",
						WaitTime: delay,
					})
				}

				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		c.logger.Timing(req.Method, req.URL.String(), duration)

		// Handle 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			// Notify onRequest callback (429)
			if c.onRequest != nil {
				c.onRequest(RequestInfo{
					Method:      req.Method,
					Path:        req.URL.Path,
					StatusCode:  resp.StatusCode,
					Duration:    duration,
					Attempt:     attempt,
					RequestBody: bodyBytes,
				})
			}

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

				// Notify onRetry callback
				if c.onRetry != nil {
					c.onRetry(RetryInfo{
						Method:   req.Method,
						Path:     req.URL.Path,
						Attempt:  attempt + 1,
						Reason:   "429",
						WaitTime: delay,
					})
				}

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
			// Notify onRequest callback (5xx)
			if c.onRequest != nil {
				c.onRequest(RequestInfo{
					Method:      req.Method,
					Path:        req.URL.Path,
					StatusCode:  resp.StatusCode,
					Duration:    duration,
					Attempt:     attempt,
					RequestBody: bodyBytes,
				})
			}

			resp.Body.Close()
			delay := h.calculateBackoff(attempt)
			c.logger.Retry(attempt, c.maxRetries, delay, fmt.Sprintf("server error (%d)", resp.StatusCode))

			// Notify onRetry callback
			if c.onRetry != nil {
				c.onRetry(RetryInfo{
					Method:   req.Method,
					Path:     req.URL.Path,
					Attempt:  attempt + 1,
					Reason:   fmt.Sprintf("%d", resp.StatusCode),
					WaitTime: delay,
				})
			}

			time.Sleep(delay)
			continue
		}

		// Notify onRequest callback (success or final attempt)
		if c.onRequest != nil {
			c.onRequest(RequestInfo{
				Method:      req.Method,
				Path:        req.URL.Path,
				StatusCode:  resp.StatusCode,
				Duration:    duration,
				Attempt:     attempt,
				RequestBody: bodyBytes,
			})
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

// jsonWriteEndpoints contains all JSON API endpoints that modify data.
// This provides defense-in-depth beyond HTTP method checking.
// Format: "METHOD /path" or "METHOD /path/" for prefix matching.
var jsonWriteEndpoints = map[string]bool{
	// Records
	"POST /v1/records":   true, // Upsert
	"DELETE /v1/records": true, // DeleteRecords

	// Apps
	"POST /v1/apps":   true, // CreateApp
	"DELETE /v1/apps": true, // DeleteApp (prefix)

	// App operations (with appId)
	"POST /v1/apps/":   true, // UpdateApp, CopyApp (prefix)
	"DELETE /v1/apps/": true, // DeleteApp (prefix)

	// Tables
	"POST /v1/tables":   true, // CreateTable
	"DELETE /v1/tables": true, // DeleteTable (prefix)
	"POST /v1/tables/":  true, // UpdateTable, relationships (prefix)

	// Relationships
	"DELETE /v1/tables/": true, // DeleteRelationship (covered above)

	// Fields
	"POST /v1/fields":   true, // CreateField
	"DELETE /v1/fields": true, // DeleteFields
	"POST /v1/fields/":  true, // UpdateField (prefix)

	// Files
	"DELETE /v1/files/": true, // DeleteFile (prefix)

	// User tokens
	"POST /v1/usertoken":   true, // Clone, Transfer, Deactivate
	"DELETE /v1/usertoken": true, // Delete

	// Users and groups
	"POST /v1/users":     true, // GetUsers is POST but read-only - handled specially
	"PUT /v1/users":      true, // DenyUsers, UndenyUsers (prefix)
	"PUT /v1/users/":     true, // DenyUsersAndGroups (prefix)
	"POST /v1/groups/":   true, // AddMembers, AddManagers, AddSubgroups (prefix)
	"DELETE /v1/groups/": true, // RemoveMembers, RemoveManagers, RemoveSubgroups (prefix)

	// Solutions
	"POST /v1/solutions": true, // CreateSolution
	"PUT /v1/solutions/": true, // UpdateSolution, ChangesetSolution (prefix)

	// Document generation (GET but writes)
	"GET /v1/docTemplates/": true, // GenerateDocument (prefix)

	// Solution from/to record (GET but writes)
	"GET /v1/solutions/fromrecord": true, // CreateSolutionFromRecord
}

// jsonReadOnlyPOSTEndpoints are POST endpoints that are actually read-only.
// These are exceptions to the "POST = write" rule.
var jsonReadOnlyPOSTEndpoints = map[string]bool{
	"POST /v1/records/query": true, // RunQuery - read-only
	"POST /v1/reports/":      true, // RunReport - read-only (prefix)
	"POST /v1/formula/run":   true, // RunFormula - read-only
	"POST /v1/audit":         true, // Audit logs - read-only
	"POST /v1/users":         true, // GetUsers - read-only despite POST
	"POST /v1/analytics/":    true, // Analytics - read-only (prefix)
}

// xmlWriteActions contains all XML API actions that modify data.
// These are blocked when the client is in read-only mode.
var xmlWriteActions = map[string]bool{
	// User/Role management
	"API_AddUserToRole":       true,
	"API_RemoveUserFromRole":  true,
	"API_ChangeUserRole":      true,
	"API_ProvisionUser":       true,
	"API_SendInvitation":      true,
	"API_ChangeManager":       true,
	"API_ChangeRecordOwner":   true,

	// Group management
	"API_CreateGroup":         true,
	"API_DeleteGroup":         true,
	"API_AddUserToGroup":      true,
	"API_RemoveUserFromGroup": true,
	"API_AddGroupToRole":      true,
	"API_RemoveGroupFromRole": true,
	"API_CopyGroup":           true,
	"API_ChangeGroupInfo":     true,
	"API_AddSubGroup":         true,
	"API_RemoveSubGroup":      true,

	// Variables
	"API_SetDBVar": true,

	// Code pages
	"API_AddReplaceDBPage": true,

	// Fields
	"API_FieldAddChoices":    true,
	"API_FieldRemoveChoices": true,
	"API_SetKeyField":        true,

	// Webhooks
	"API_Webhooks_Create":     true,
	"API_Webhooks_Edit":       true,
	"API_Webhooks_Delete":     true,
	"API_Webhooks_Activate":   true,
	"API_Webhooks_Deactivate": true,
	"API_Webhooks_Copy":       true,

	// Records/Import
	"API_ImportFromCSV":    true,
	"API_RunImport":        true,
	"API_CopyMasterDetail": true,
	"API_PurgeRecords":     true,
	"API_AddRecord":        true,
	"API_EditRecord":       true,
	"API_DeleteRecord":     true,

	// Auth (clears session state)
	"API_SignOut": true,
}

// isWriteMethod returns true if the HTTP method is a write operation.
func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}

// isXMLWriteAction returns true if the XML API action modifies data.
func isXMLWriteAction(action string) bool {
	return xmlWriteActions[action]
}

// isJSONWriteEndpoint checks if the request matches a known write endpoint.
// Uses both exact matching and prefix matching for paths with IDs.
func isJSONWriteEndpoint(method, path string) bool {
	key := method + " " + path

	// Exact match
	if jsonWriteEndpoints[key] {
		return true
	}

	// Prefix match for paths with IDs (e.g., "POST /v1/apps/" matches "POST /v1/apps/abc123")
	// Try progressively shorter prefixes
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			prefixKey := method + " " + path[:i+1]
			if jsonWriteEndpoints[prefixKey] {
				return true
			}
		}
	}

	return false
}

// isJSONReadOnlyPOSTEndpoint checks if a POST request is actually read-only.
func isJSONReadOnlyPOSTEndpoint(path string) bool {
	key := "POST " + path

	// Exact match
	if jsonReadOnlyPOSTEndpoints[key] {
		return true
	}

	// Prefix match
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			prefixKey := "POST " + path[:i+1]
			if jsonReadOnlyPOSTEndpoints[prefixKey] {
				return true
			}
		}
	}

	return false
}

// checkReadOnly returns an error if the request is a write operation and read-only mode is enabled.
func (c *Client) checkReadOnly(req *http.Request) error {
	if !c.readOnly {
		return nil
	}

	method := req.Method
	path := req.URL.Path

	// Check for XML API requests first (they use POST for everything)
	if action := req.Header.Get("QUICKBASE-ACTION"); action != "" {
		if isXMLWriteAction(action) {
			return core.NewReadOnlyError(method, path, action)
		}
		return nil // Allow read-only XML actions
	}

	// Layer 1: Explicit blocklist check (defense-in-depth)
	if isJSONWriteEndpoint(method, path) {
		// Exception: Some POST endpoints are read-only (RunQuery, RunReport, etc.)
		if method == http.MethodPost && isJSONReadOnlyPOSTEndpoint(path) {
			return nil
		}
		return core.NewReadOnlyError(method, path, "")
	}

	// Layer 2: HTTP method check (catch-all for any endpoints not in blocklist)
	if isWriteMethod(method) {
		// Exception: Some POST endpoints are read-only
		if method == http.MethodPost && isJSONReadOnlyPOSTEndpoint(path) {
			return nil
		}
		return core.NewReadOnlyError(method, path, "")
	}

	return nil
}
