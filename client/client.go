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
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/generated"
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
	maxRetries   int
	initialDelay time.Duration
	maxDelay     time.Duration
	backoffMult  float64

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

	// One HTTP client for every attempt; its transport is closed by Close
	http *http.Client
	// basePath is the base URL's path ("/v1" by default), which apiPath swaps
	// for "/v1" so endpoint lists match under any base URL
	basePath  string
	transport *http.Transport

	// Read-only mode blocks all write operations
	readOnly bool

	// App token for apps requiring application tokens
	// Only needed when NOT using user tokens (which bypass app token checks)
	appToken string

	// Upsert returns a LineErrorsError when QuickBase rejects values
	lineErrorsAsErrors bool

	// authFor builds the auth strategy from the realm (the auth options)
	authFor func(realm string) auth.Strategy
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
	Method   string        // HTTP method
	Path     string        // URL path
	Attempt  int           // Which attempt is about to happen (2 = first retry)
	Reason   string        // Why we're retrying (e.g., "429", "503", "timeout")
	WaitTime time.Duration // How long we'll wait before retrying
}

// Option configures a Client.
type Option func(*Client)

// WithMaxRetries sets how many times a request is attempted in all,
// counting the first (default 3: one try and up to two retries). Values
// below 1 mean a single attempt with no retries.
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

// WithConvertDates has no effect: the client never converted dates. Responses
// keep dates as the strings QuickBase sends; parse them with [core.ParseISODate].
//
// Deprecated: it does nothing and will be removed.
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
// This is useful for data extraction tools, reporting systems, or any context
// where you want to ensure the client can only read data, never modify it.
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

// WithLineErrorsAsErrors makes Upsert return a [core.LineErrorsError] when
// QuickBase rejects values. QuickBase still saves those records, with the
// rejected fields blank, and answers 207; without this option the rejections
// are only in the result's metadata (LineErrors). The result is returned with
// the error, so the saved records' IDs aren't lost.
func WithLineErrorsAsErrors() Option {
	return func(c *Client) {
		c.lineErrorsAsErrors = true
	}
}

// WithAppToken sets an application token for XML API calls.
//
// App tokens are only needed for the XML API when:
//   - The app has "Require Application Tokens" enabled, AND
//   - You're NOT using user token authentication (which bypasses app token checks)
//
// Common scenarios requiring app tokens:
//   - Using ticket auth (WithTicketAuth) with XML API methods
//   - Using temp tokens (WithTempTokens) with XML API methods
//
// The JSON API does not use app tokens - user tokens and temp tokens provide
// sufficient authentication. This option only affects XML API calls made via
// the xml sub-package.
//
// Example:
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithTicketAuth("user@example.com", "password"),
//	    quickbase.WithAppToken("your-app-token"),
//	)
//
//	// XML API calls will include the app token
//	xmlClient := xml.New(client)
//	roles, _ := xmlClient.GetRoleInfo(ctx, appId)
func WithAppToken(token string) Option {
	return func(c *Client) {
		c.appToken = token
	}
}

// WithSchema sets the schema for table and field aliases.
// When configured, the client automatically:
//   - Transforms table aliases to IDs in requests (from, to fields)
//   - Transforms field aliases to IDs in requests (select, sortBy, groupBy, where, data)
//   - Keys the query builder's records by alias, if WithSchemaOptions sets
//     TransformResponses (off by default: records are keyed by field ID)
//   - Unwraps { value: X } to just X in response records
func WithSchema(schema *core.Schema) Option {
	return func(c *Client) {
		c.schema = core.ResolveSchema(schema)
	}
}

// WithSchemaOptions sets the schema with custom options.
// Use this to control schema behavior, such as keying the query builder's
// records by field alias.
//
// Example:
//
//	quickbase.WithSchemaOptions(schema, core.SchemaOptions{
//	    TransformResponses: true, // records keyed by alias, not field ID
//	})
func WithSchemaOptions(schema *core.Schema, opts core.SchemaOptions) Option {
	return func(c *Client) {
		c.schema = core.ResolveSchemaWithOptions(schema, opts)
	}
}

// New creates a new QuickBase client. authStrategy may be nil when an option
// sets one (WithUserToken, WithTempTokens, WithAuth, ...).
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
	if c.auth == nil && c.authFor != nil {
		c.auth = c.authFor(realm)
	}
	if c.auth == nil {
		return nil, fmt.Errorf("no authentication strategy configured; use WithUserToken, WithTempTokens, WithSSOTokenAuth, WithTicketAuth or WithTicket")
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

	c.transport = transport
	c.http = &http.Client{Timeout: c.timeout, Transport: transport}
	if u, err := url.Parse(c.baseURL); err == nil {
		c.basePath = strings.TrimSuffix(u.Path, "/")
	}

	// The generated client sends through authHTTPClient: auth, retry, throttle
	genClient, err := generated.NewClientWithResponses(
		c.baseURL,
		generated.WithHTTPClient(&authHTTPClient{client: c}),
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
	// Default to JSON; keep a body's own type (QBL is application/x-yaml)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
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

// authHTTPClient is the generated client's HTTP doer: it adds auth, retry,
// and rate limiting.
type authHTTPClient struct {
	client *Client
}

func (h *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c := h.client
	ctx := req.Context()

	// Check read-only mode before making request
	if err := c.checkReadOnly(req); err != nil {
		return nil, err
	}

	// Read body once for potential retries
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		req.Body.Close()
	}

	// Schema aliases, for every request
	if c.schema != nil {
		var err error
		if bodyBytes, err = c.resolveAliases(req, bodyBytes); err != nil {
			return nil, err
		}
	}

	// Extract dbid from request for temp token auth
	dbid := extractDBID(req)
	if bodyBytes != nil {
		// Also check body for dbid if not found elsewhere
		if dbid == "" {
			dbid = extractDBIDFromBody(bodyBytes)
		}
	}

	// Request factory for JSON requests
	requestFactory := func(ctx context.Context, token string) (*http.Request, error) {
		reqCopy := req.Clone(ctx)
		if bodyBytes != nil {
			// The body may have changed (aliases resolved), so its length too
			reqCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			reqCopy.ContentLength = int64(len(bodyBytes))
		}
		c.auth.ApplyAuth(reqCopy, token)
		return reqCopy, nil
	}

	// Response handler for JSON requests
	responseHandler := func(resp *http.Response) (bool, error) {
		// Handle 401 Unauthorized - try to refresh token
		if resp.StatusCode == http.StatusUnauthorized {
			return true, nil // Signal retry with refresh
		}
		return false, nil // No special handling, use standard logic
	}

	retrySafe := !isJSONWrite(req.Method, c.apiPath(req.URL.Path))
	return c.do(ctx, dbid, req.Method, req.URL.Path, bodyBytes, retrySafe, requestFactory, responseHandler)
}

// tableBodies are the operations whose JSON body names a table and its
// fields. Other bodies are left alone: transferUserToken's from and to are
// user IDs, and analytics' where and groupBy aren't record queries.
var tableBodies = map[string]bool{
	"/v1/records":               true, // upsert (to), deleteRecords (from)
	"/v1/records/query":         true,
	"/v1/records/modifiedSince": true,
	"/v1/formula/run":           true,
}

// tablePath matches a table ID in a path: /tables/{tableId}, /files/{tableId}.
var tablePath = regexp.MustCompile(`^(.*/(?:tables|files)/)([^/]+)(.*)$`)

// resolveAliases resolves the schema's table and field aliases in a request:
// a table in the path (/tables/{tableId}, /files/{tableId}) or a tableId
// query parameter; and, for the record operations, a JSON body's from, to,
// select, sortBy, groupBy, where, record data, fieldsToReturn and
// mergeFieldId. Numbers in the body keep their exact values.
func (c *Client) resolveAliases(req *http.Request, body []byte) ([]byte, error) {
	if m := tablePath.FindStringSubmatch(req.URL.Path); m != nil {
		resolved, err := core.ResolveTableAlias(c.schema, m[2])
		if err != nil {
			return nil, err
		}
		if resolved != m[2] {
			req.URL.Path = m[1] + resolved + m[3]
			req.URL.RawPath = ""
		}
	}

	query := req.URL.Query()
	if tableID := query.Get("tableId"); tableID != "" {
		resolved, err := core.ResolveTableAlias(c.schema, tableID)
		if err != nil {
			return nil, err
		}
		if resolved != tableID {
			query.Set("tableId", resolved)
			req.URL.RawQuery = query.Encode()
		}
	}

	if len(body) == 0 || !tableBodies[c.apiPath(req.URL.Path)] || !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") {
		return body, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil {
		return body, nil // not an object; nothing to resolve
	}
	resolved, _, err := core.TransformRequest(fields, c.schema)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resolved)
}

// requestFactory creates an authenticated request for a retry attempt.
type requestFactory func(ctx context.Context, token string) (*http.Request, error)

// responseHandler handles a response and returns true if a retry with token refresh is needed.
type responseHandler func(resp *http.Response) (bool, error)

// do sends a request, retrying it with backoff:
//   - after a 429, always (QuickBase didn't process it), waiting Retry-After
//     when given;
//   - after a 401 the auth strategy has a new token for;
//   - after a network error or a 5xx only when retrySafe. A write (an upsert,
//     a create) that timed out or got a 502 may have been applied, and
//     sending it again could, for example, add its records twice. A write
//     whose connection never opened is retried: it wasn't sent.
func (c *Client) do(
	ctx context.Context,
	dbid string,
	method string,
	path string,
	bodyBytes []byte,
	retrySafe bool,
	factory requestFactory,
	handler responseHandler,
) (*http.Response, error) {
	startTime := time.Now()

	// At least one attempt: with maxRetries 0 the loop never ran and the
	// caller got neither a response nor an error.
	maxAttempts := max(c.maxRetries, 1)
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := c.throttle.Acquire(ctx); err != nil {
			return nil, fmt.Errorf("throttle: %w", err)
		}
		token, err := c.auth.GetToken(ctx, dbid)
		if err != nil {
			return nil, fmt.Errorf("getting auth token: %w", err)
		}
		req, err := factory(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		sent := time.Now()
		resp, err := c.http.Do(req)
		info := RequestInfo{Method: method, Path: path, Duration: time.Since(sent), Attempt: attempt, Error: err, RequestBody: bodyBytes}
		if resp != nil {
			info.StatusCode = resp.StatusCode
		}
		if c.onRequest != nil {
			c.onRequest(info)
		}
		last := attempt >= maxAttempts

		var wait time.Duration
		var reason, logReason string
		switch {
		case err != nil:
			// The caller's context ended (cancelled, or its own deadline):
			// report that, not a timeout of ours
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if last || !(retrySafe || neverSent(err)) {
				return nil, timeoutOr(err, c.timeout)
			}
			wait, reason, logReason = c.calculateBackoff(attempt), "network error", "network error"

		case resp.StatusCode == http.StatusTooManyRequests:
			resp.Body.Close()
			rl := core.RateLimitInfo{
				Timestamp:  time.Now(),
				RequestURL: req.URL.String(),
				HTTPStatus: http.StatusTooManyRequests,
				CFRay:      resp.Header.Get("cf-ray"),
				TID:        resp.Header.Get("tid"),
				QBAPIRay:   resp.Header.Get("qb-api-ray"),
				Attempt:    attempt,
			}
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				rl.RetryAfter, _ = strconv.Atoi(ra)
			}
			c.logger.RateLimit(rl)
			if c.onRateLimit != nil {
				c.onRateLimit(rl)
			}
			if last {
				return nil, core.NewRateLimitError(rl, "")
			}
			wait = c.calculateBackoff(attempt)
			if rl.RetryAfter > 0 {
				wait = time.Duration(rl.RetryAfter) * time.Second
			}
			reason, logReason = "429", "rate limited (429)"

		default:
			// The JSON handler asks for a token refresh on 401; the XML one
			// on errcode 8 (and reads the body)
			refresh, err := handler(resp)
			if err != nil {
				resp.Body.Close()
				return nil, err
			}
			if refresh && !last {
				newToken, err := c.auth.HandleAuthError(ctx, resp.StatusCode, dbid, attempt, maxAttempts)
				if err != nil {
					resp.Body.Close()
					return nil, err
				}
				if newToken != "" {
					resp.Body.Close()
					c.logger.Debug("Token refreshed, retrying request")
					continue
				}
			}
			if resp.StatusCode < 500 || last || !retrySafe {
				c.logger.Timing(method, path, time.Since(startTime))
				return resp, nil
			}
			resp.Body.Close()
			wait, reason, logReason = c.calculateBackoff(attempt), strconv.Itoa(resp.StatusCode), fmt.Sprintf("server error (%d)", resp.StatusCode)
		}

		c.logger.Retry(attempt, maxAttempts, wait, logReason)
		if c.onRetry != nil {
			c.onRetry(RetryInfo{Method: method, Path: path, Attempt: attempt + 1, Reason: reason, WaitTime: wait})
		}
		if err := sleepCtx(ctx, wait); err != nil {
			return nil, err
		}
	}
}

// neverSent reports whether a request failed before its connection opened
// (DNS, connection refused), so QuickBase can't have received it.
func neverSent(err error) bool {
	var op *net.OpError
	return errors.As(err, &op) && op.Op == "dial"
}

// timeoutOr returns a TimeoutError if err is the client's own timeout
// (WithTimeout) running out, and err otherwise.
func timeoutOr(err error, timeout time.Duration) error {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return core.NewTimeoutError(int(timeout.Milliseconds()))
	}
	return err
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

	// Request factory for XML requests
	requestFactory := func(ctx context.Context, token string) (*http.Request, error) {
		xmlBody := body

		// Inject app token into XML body if configured
		if c.appToken != "" {
			xmlBody = injectXMLElement(xmlBody, "apptoken", c.appToken)
		}

		// Inject auth token into XML body if the strategy supports it
		// The XML API requires tokens as body elements, not Authorization headers
		usesXMLAuth := false
		if xmlAuth, ok := c.auth.(auth.XMLAuthProvider); ok {
			elemName, elemValue := xmlAuth.XMLAuthElement(token)
			xmlBody = injectXMLElement(xmlBody, elemName, elemValue)
			usesXMLAuth = true
		}

		url := fmt.Sprintf("https://%s.quickbase.com/db/%s", c.realm, dbid)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(xmlBody))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/xml")
		req.Header.Set("QUICKBASE-ACTION", action)

		// Apply auth via header only if strategy doesn't support XML body auth
		if !usesXMLAuth {
			c.auth.ApplyAuth(req, token)
		}

		return req, nil
	}

	var respBody []byte

	// Response handler for XML requests
	responseHandler := func(resp *http.Response) (bool, error) {
		defer resp.Body.Close()

		// Read response body
		var err error
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("reading XML response: %w", err)
		}

		// Check for XML-level errors (errcode)
		// QuickBase XML API often returns 200 OK but with an errcode in the body
		type xmlBaseResponse struct {
			XMLName xml.Name `xml:"qdbapi"`
			ErrCode int      `xml:"errcode"`
		}
		var base xmlBaseResponse
		if err := xml.Unmarshal(respBody, &base); err == nil {
			// errcode 8 means invalid or expired ticket
			if base.ErrCode == 8 {
				return true, nil // Signal retry with refresh
			}
		}

		return false, nil
	}

	_, err := c.do(ctx, dbid, http.MethodPost, "/db/"+dbid, body, !isXMLWriteAction(action), requestFactory, responseHandler)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// calculateBackoff calculates exponential backoff with jitter.
func (c *Client) calculateBackoff(attempt int) time.Duration {
	delay := float64(c.initialDelay) * math.Pow(c.backoffMult, float64(attempt-1))

	// Add jitter: ±10%
	jitter := delay * 0.1 * (rand.Float64()*2 - 1)
	delay += jitter

	if delay > float64(c.maxDelay) {
		delay = float64(c.maxDelay)
	}

	return time.Duration(delay)
}

// sleepCtx waits for d, or returns the context's error if it ends first, so
// cancelling a request also cancels a retry's backoff wait.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// injectXMLElement inserts <name>value</name> right after the opening
// <qdbapi> tag of an XML request body: the XML API takes app tokens and auth
// tokens as body elements, not headers. The value is escaped.
// See: https://help.quickbase.com/docs/api-getdbpage
func injectXMLElement(body []byte, name, value string) []byte {
	const openTag = "<qdbapi>"
	idx := bytes.Index(body, []byte(openTag))
	if idx == -1 {
		return body // Can't find tag, return unchanged
	}
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	insertPos := idx + len(openTag)
	out := make([]byte, 0, len(body)+len(name)*2+escaped.Len()+5)
	out = append(out, body[:insertPos]...)
	out = append(out, "<"+name+">"...)
	out = append(out, escaped.Bytes()...)
	out = append(out, "</"+name+">"...)
	return append(out, body[insertPos:]...)
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

	// Solutions to and from records are GETs that write: see solutionRecordWritePath
}

// solutionRecordWritePath matches the solution endpoints that are GETs but
// write: createSolutionFromRecord (/solutions/fromrecord) and
// updateSolutionToRecord (/solutions/{id}/fromrecord) change apps, and
// exportSolutionToRecord (/solutions/{id}/torecord) writes a record.
// changesetSolutionFromRecord (/solutions/{id}/changeset/fromrecord) only
// reads, and doesn't match. The path may carry a base URL's prefix.
var solutionRecordWritePath = regexp.MustCompile(`/solutions/(fromrecord|[^/]+/(fromrecord|torecord))$`)

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
	"API_AddUserToRole":      true,
	"API_RemoveUserFromRole": true,
	"API_ChangeUserRole":     true,
	"API_ProvisionUser":      true,
	"API_SendInvitation":     true,
	"API_ChangeManager":      true,
	"API_ChangeRecordOwner":  true,

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
	if method == http.MethodGet && solutionRecordWritePath.MatchString(path) {
		return true
	}

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
	path := c.apiPath(req.URL.Path)

	// Check for XML API requests first (they use POST for everything)
	if action := req.Header.Get("QUICKBASE-ACTION"); action != "" {
		if isXMLWriteAction(action) {
			return core.NewReadOnlyError(method, path, action)
		}
		return nil // Allow read-only XML actions
	}

	if isJSONWrite(method, path) {
		return core.NewReadOnlyError(method, path, "")
	}
	return nil
}

// apiPath returns a request's path as the endpoint lists name it, under
// /v1, whatever the base URL: WithBaseURL("https://proxy/qb") sends
// /qb/records/query, which is /v1/records/query.
func (c *Client) apiPath(path string) string {
	return "/v1" + strings.TrimPrefix(path, c.basePath)
}

// isJSONWrite reports whether a JSON API request can change data: a known
// write endpoint (including GETs that write), or any POST, PUT, DELETE or
// PATCH that isn't a known read sent as POST (runQuery, runReport, ...).
// Read-only mode refuses these, and they aren't retried after a timeout or
// 5xx.
func isJSONWrite(method, path string) bool {
	if method == http.MethodPost && isJSONReadOnlyPOSTEndpoint(path) {
		return false
	}
	return isJSONWriteEndpoint(method, path) || isWriteMethod(method)
}
