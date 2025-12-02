// Package client provides a QuickBase API client with retry and rate limiting.
package client

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

// Client wraps the generated QuickBase client with auth, retry, and rate limiting.
type Client struct {
	generated *generated.ClientWithResponses
	auth      auth.Strategy
	realm     string
	baseURL   string

	// Retry configuration
	maxRetries int
	retryDelay time.Duration

	// Rate limiting
	rateLimiter *RateLimiter
}

// Option configures a Client.
type Option func(*Client)

// WithMaxRetries sets the maximum number of retry attempts (default 3).
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithRetryDelay sets the base delay between retries (default 1s).
func WithRetryDelay(d time.Duration) Option {
	return func(c *Client) {
		c.retryDelay = d
	}
}

// WithRateLimiter sets a custom rate limiter.
func WithRateLimiter(rl *RateLimiter) Option {
	return func(c *Client) {
		c.rateLimiter = rl
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
		auth:       authStrategy,
		realm:      realm,
		baseURL:    "https://api.quickbase.com/v1",
		maxRetries: 3,
		retryDelay: time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Create rate limiter if not provided
	if c.rateLimiter == nil {
		c.rateLimiter = NewRateLimiter(5, 50) // 5 req/s, burst of 50
	}

	// Create the generated client with our custom HTTP doer
	httpClient := &authHTTPClient{
		client:      c,
		httpClient:  http.DefaultClient,
		rateLimiter: c.rateLimiter,
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
func (c *Client) API() *generated.ClientWithResponses {
	return c.generated
}

// authHTTPClient wraps http.Client to add auth, retry, and rate limiting.
type authHTTPClient struct {
	client      *Client
	httpClient  *http.Client
	rateLimiter *RateLimiter
}

func (h *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Extract dbid from request for temp token auth
	dbid := extractDBID(req)

	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= h.client.maxRetries; attempt++ {
		// Rate limiting
		if err := h.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}

		// Get auth token
		token, err := h.client.auth.GetToken(ctx, dbid)
		if err != nil {
			return nil, fmt.Errorf("getting auth token: %w", err)
		}

		// Clone request for retry
		reqCopy := req.Clone(ctx)
		if req.Body != nil {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("reading request body: %w", err)
			}
			req.Body = io.NopCloser(io.MultiReader(io.NopCloser(
				&bytesReader{data: body, pos: 0},
			)))
			reqCopy.Body = io.NopCloser(&bytesReader{data: body, pos: 0})
		}

		// Apply auth
		h.client.auth.ApplyAuth(reqCopy, token)

		// Make request
		resp, err := h.httpClient.Do(reqCopy)
		if err != nil {
			lastErr = err
			if attempt < h.client.maxRetries {
				time.Sleep(h.client.retryDelay * time.Duration(math.Pow(2, float64(attempt))))
				continue
			}
			return nil, err
		}

		// Handle 429 Too Many Requests
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), h.client.retryDelay, attempt)
			time.Sleep(retryAfter)
			continue
		}

		// Handle 401 Unauthorized - try to refresh token
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			newToken, err := h.client.auth.HandleAuthError(ctx, resp.StatusCode, dbid, attempt, h.client.maxRetries)
			if err != nil {
				return nil, err
			}
			if newToken != "" {
				continue
			}
		}

		// Handle 5xx server errors with retry
		if resp.StatusCode >= 500 && attempt < h.client.maxRetries {
			resp.Body.Close()
			time.Sleep(h.client.retryDelay * time.Duration(math.Pow(2, float64(attempt))))
			continue
		}

		lastResp = resp
		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return lastResp, nil
}

// bytesReader is a simple io.Reader for byte slices.
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// extractDBID extracts the table/app ID from a request URL for temp token auth.
func extractDBID(req *http.Request) string {
	q := req.URL.Query()
	if dbid := q.Get("tableId"); dbid != "" {
		return dbid
	}
	if dbid := q.Get("appId"); dbid != "" {
		return dbid
	}
	// TODO: Parse from path or body if needed
	return ""
}

// parseRetryAfter parses the Retry-After header or returns exponential backoff.
func parseRetryAfter(header string, baseDelay time.Duration, attempt int) time.Duration {
	if header != "" {
		if seconds, err := strconv.Atoi(header); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return baseDelay * time.Duration(math.Pow(2, float64(attempt)))
}
