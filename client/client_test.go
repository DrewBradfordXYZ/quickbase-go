package client

import (
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestExtractDBID(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		expected string
	}{
		// Query params - tableId
		{
			name: "tableId from query params",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/fields?tableId=bqtable123"),
			},
			expected: "bqtable123",
		},
		// Query params - appId
		{
			name: "appId from query params",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/apps?appId=bqapp123"),
			},
			expected: "bqapp123",
		},
		// Path - tableId
		{
			name: "tableId from path /tables/{tableId}",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/tables/bqpath456"),
			},
			expected: "bqpath456",
		},
		{
			name: "tableId from nested path /tables/{tableId}/records",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/tables/bqnested789/records"),
			},
			expected: "bqnested789",
		},
		// Path - appId
		{
			name: "appId from path /apps/{appId}",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/apps/bqapp456"),
			},
			expected: "bqapp456",
		},
		{
			name: "appId from nested path /apps/{appId}/tables",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/apps/bqapp789/tables"),
			},
			expected: "bqapp789",
		},
		// Priority: query params over path
		{
			name: "query params preferred over path",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/tables/path123?tableId=query456"),
			},
			expected: "query456",
		},
		// Priority: tableId over appId
		{
			name: "tableId preferred over appId in query",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/fields?tableId=table123&appId=app456"),
			},
			expected: "table123",
		},
		// No dbid found
		{
			name: "no dbid in request",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/users"),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDBID(tt.req)
			if result != tt.expected {
				t.Errorf("extractDBID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractDBIDFromBody(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "extracts from body.from (runQuery)",
			body:     []byte(`{"from": "bqfrom123", "select": [3, 6, 7]}`),
			expected: "bqfrom123",
		},
		{
			name:     "extracts from body.from (deleteRecords)",
			body:     []byte(`{"from": "bqdelete456", "where": "{3.GT.0}"}`),
			expected: "bqdelete456",
		},
		{
			name:     "extracts from body.to (upsert)",
			body:     []byte(`{"to": "bqto789", "data": []}`),
			expected: "bqto789",
		},
		{
			name:     "prefers from over to",
			body:     []byte(`{"from": "bqfrom111", "to": "bqto222"}`),
			expected: "bqfrom111",
		},
		{
			name:     "empty body",
			body:     []byte(``),
			expected: "",
		},
		{
			name:     "empty JSON object",
			body:     []byte(`{}`),
			expected: "",
		},
		{
			name:     "invalid JSON",
			body:     []byte(`not json`),
			expected: "",
		},
		{
			name:     "from is not a string",
			body:     []byte(`{"from": 12345}`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDBIDFromBody(tt.body)
			if result != tt.expected {
				t.Errorf("extractDBIDFromBody() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateRealm(t *testing.T) {
	tests := []struct {
		name      string
		realm     string
		expectErr bool
	}{
		{
			name:      "valid realm",
			realm:     "mycompany",
			expectErr: false,
		},
		{
			name:      "empty realm",
			realm:     "",
			expectErr: true,
		},
		{
			name:      "realm with dot",
			realm:     "mycompany.quickbase.com",
			expectErr: true,
		},
		{
			name:      "realm with subdomain dot",
			realm:     "my.company",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRealm(tt.realm)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateRealm(%q) expected error, got nil", tt.realm)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateRealm(%q) unexpected error: %v", tt.realm, err)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	// Create a mock authHTTPClient to test backoff calculation
	client := &Client{
		initialDelay: 100 * time.Millisecond,
		maxDelay:     1000 * time.Millisecond,
		backoffMult:  2,
	}
	h := &authHTTPClient{client: client}

	t.Run("first attempt delay around initialDelay", func(t *testing.T) {
		delay := h.calculateBackoff(1)
		// 100ms ± 10% jitter = 90-110ms
		if delay < 90*time.Millisecond || delay > 110*time.Millisecond {
			t.Errorf("calculateBackoff(1) = %v, want 90-110ms", delay)
		}
	})

	t.Run("second attempt doubles delay", func(t *testing.T) {
		delay := h.calculateBackoff(2)
		// 100 * 2^1 = 200ms ± 10% = 180-220ms
		if delay < 180*time.Millisecond || delay > 220*time.Millisecond {
			t.Errorf("calculateBackoff(2) = %v, want 180-220ms", delay)
		}
	})

	t.Run("third attempt quadruples delay", func(t *testing.T) {
		delay := h.calculateBackoff(3)
		// 100 * 2^2 = 400ms ± 10% = 360-440ms
		if delay < 360*time.Millisecond || delay > 440*time.Millisecond {
			t.Errorf("calculateBackoff(3) = %v, want 360-440ms", delay)
		}
	})

	t.Run("caps at maxDelay", func(t *testing.T) {
		delay := h.calculateBackoff(10)
		// Should be capped at 1000ms + jitter max
		if delay > 1100*time.Millisecond {
			t.Errorf("calculateBackoff(10) = %v, should be capped at ~1100ms", delay)
		}
	})

	t.Run("respects different multiplier", func(t *testing.T) {
		client := &Client{
			initialDelay: 100 * time.Millisecond,
			maxDelay:     10 * time.Second,
			backoffMult:  3,
		}
		h := &authHTTPClient{client: client}

		delay := h.calculateBackoff(2)
		// 100 * 3^1 = 300ms ± 10% = 270-330ms
		if delay < 270*time.Millisecond || delay > 330*time.Millisecond {
			t.Errorf("calculateBackoff(2) with mult=3 = %v, want 270-330ms", delay)
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected time.Duration
	}{
		{
			name:     "parses seconds",
			header:   "30",
			expected: 30 * time.Second,
		},
		{
			name:     "parses single digit",
			header:   "5",
			expected: 5 * time.Second,
		},
		{
			name:     "falls back on empty header",
			header:   "",
			expected: 2 * time.Second, // 1s * 2^1 for attempt 1
		},
		{
			name:     "falls back on invalid header",
			header:   "not-a-number",
			expected: 2 * time.Second, // fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRetryAfter(tt.header, time.Second, 1)
			if result != tt.expected {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, result, tt.expected)
			}
		})
	}
}

func TestBackoffJitter(t *testing.T) {
	// Run multiple times to verify jitter introduces variation
	client := &Client{
		initialDelay: 1 * time.Second,
		maxDelay:     30 * time.Second,
		backoffMult:  2,
	}
	h := &authHTTPClient{client: client}

	// Expected base delay for attempt 1: 1s
	baseDelay := 1 * time.Second
	minDelay := time.Duration(float64(baseDelay) * 0.9) // -10%
	maxDelay := time.Duration(float64(baseDelay) * 1.1) // +10%

	seenValues := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		delay := h.calculateBackoff(1)

		if delay < minDelay || delay > maxDelay {
			t.Errorf("calculateBackoff(1) = %v, want between %v and %v", delay, minDelay, maxDelay)
		}

		seenValues[delay] = true
	}

	// With jitter, we should see multiple different values
	if len(seenValues) < 5 {
		t.Errorf("Expected jitter to produce variation, got only %d unique values", len(seenValues))
	}
}

// Helper function
func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

// Test exponential growth
func TestExponentialBackoff(t *testing.T) {
	client := &Client{
		initialDelay: 100 * time.Millisecond,
		maxDelay:     10 * time.Second,
		backoffMult:  2,
	}
	h := &authHTTPClient{client: client}

	expectedDelays := []time.Duration{
		100 * time.Millisecond,  // 100 * 2^0
		200 * time.Millisecond,  // 100 * 2^1
		400 * time.Millisecond,  // 100 * 2^2
		800 * time.Millisecond,  // 100 * 2^3
		1600 * time.Millisecond, // 100 * 2^4
	}

	for attempt, expected := range expectedDelays {
		delay := h.calculateBackoff(attempt + 1)
		// Allow ±10% for jitter
		minExpected := time.Duration(float64(expected) * 0.9)
		maxExpected := time.Duration(float64(expected) * 1.1)

		if delay < minExpected || delay > maxExpected {
			t.Errorf("calculateBackoff(%d) = %v, want between %v and %v",
				attempt+1, delay, minExpected, maxExpected)
		}
	}
}

// Verify the exponential formula
func TestExponentialFormula(t *testing.T) {
	// delay = initialDelay * multiplier^(attempt-1)
	initialDelay := 100.0
	multiplier := 2.0

	for attempt := 1; attempt <= 5; attempt++ {
		expected := initialDelay * math.Pow(multiplier, float64(attempt-1))
		t.Logf("Attempt %d: expected base delay = %.0fms", attempt, expected)
	}
}
