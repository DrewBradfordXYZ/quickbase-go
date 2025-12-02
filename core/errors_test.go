package core

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestQuickbaseError(t *testing.T) {
	t.Run("Error() with description", func(t *testing.T) {
		err := &QuickbaseError{
			Message:     "Bad Request",
			StatusCode:  400,
			Description: "Invalid field value",
		}

		expected := "Bad Request: Invalid field value (status: 400)"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Error() without description", func(t *testing.T) {
		err := &QuickbaseError{
			Message:    "Not Found",
			StatusCode: 404,
		}

		expected := "Not Found (status: 404)"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Unwrap() returns cause", func(t *testing.T) {
		cause := &ValidationError{QuickbaseError: QuickbaseError{Message: "cause"}}
		err := &QuickbaseError{
			Message: "wrapper",
			Cause:   cause,
		}

		if err.Unwrap() != cause {
			t.Errorf("Unwrap() did not return cause")
		}
	})
}

func TestRateLimitError(t *testing.T) {
	t.Run("Error() with RetryAfter", func(t *testing.T) {
		err := &RateLimitError{
			QuickbaseError: QuickbaseError{StatusCode: 429},
			RetryAfter:     30,
		}

		expected := "rate limited, retry after 30 seconds"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("Error() without RetryAfter", func(t *testing.T) {
		err := &RateLimitError{
			QuickbaseError: QuickbaseError{StatusCode: 429},
			RetryAfter:     0,
		}

		expected := "rate limited"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})
}

func TestNewRateLimitError(t *testing.T) {
	info := RateLimitInfo{
		Timestamp:  time.Now(),
		RequestURL: "https://api.quickbase.com/v1/records",
		HTTPStatus: 429,
		RetryAfter: 10,
		QBAPIRay:   "ray123",
		Attempt:    1,
	}

	err := NewRateLimitError(info, "")

	if err.RetryAfter != 10 {
		t.Errorf("RetryAfter = %d, want 10", err.RetryAfter)
	}
	if err.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", err.StatusCode)
	}
	if err.RayID != "ray123" {
		t.Errorf("RayID = %q, want 'ray123'", err.RayID)
	}
}

func TestNewAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("Invalid token", "ray456")

	if err.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", err.StatusCode)
	}
	if err.Message != "Invalid token" {
		t.Errorf("Message = %q, want 'Invalid token'", err.Message)
	}
	if err.RayID != "ray456" {
		t.Errorf("RayID = %q, want 'ray456'", err.RayID)
	}
}

func TestNewAuthorizationError(t *testing.T) {
	err := NewAuthorizationError("Access denied", "ray789")

	if err.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", err.StatusCode)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("Table not found", "ray101")

	if err.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", err.StatusCode)
	}
}

func TestNewValidationError(t *testing.T) {
	errors := []FieldError{
		{Field: "name", Message: "Name is required"},
		{Field: "email", Message: "Invalid email format"},
	}
	err := NewValidationError("Validation failed", "ray202", errors)

	if err.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", err.StatusCode)
	}
	if len(err.Errors) != 2 {
		t.Errorf("Errors length = %d, want 2", len(err.Errors))
	}
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError(30000)

	if err.TimeoutMs != 30000 {
		t.Errorf("TimeoutMs = %d, want 30000", err.TimeoutMs)
	}

	expected := "request timed out after 30000ms"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestNewServerError(t *testing.T) {
	err := NewServerError(503, "Service Unavailable", "ray303")

	if err.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503", err.StatusCode)
	}
	if err.Message != "Service Unavailable" {
		t.Errorf("Message = %q, want 'Service Unavailable'", err.Message)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "RateLimitError is retryable",
			err: &RateLimitError{
				RateLimitInfo: RateLimitInfo{HTTPStatus: 429},
			},
			expected: true,
		},
		{
			name:     "ServerError is retryable",
			err:      NewServerError(500, "Internal Server Error", ""),
			expected: true,
		},
		{
			name:     "TimeoutError is retryable",
			err:      NewTimeoutError(30000),
			expected: true,
		},
		{
			name:     "ValidationError is not retryable",
			err:      NewValidationError("Bad request", "", nil),
			expected: false,
		},
		{
			name:     "NotFoundError is not retryable",
			err:      NewNotFoundError("Not found", ""),
			expected: false,
		},
		{
			name:     "AuthenticationError is not retryable",
			err:      NewAuthenticationError("Unauthorized", ""),
			expected: false,
		},
		{
			name:     "AuthorizationError is not retryable",
			err:      NewAuthorizationError("Forbidden", ""),
			expected: false,
		},
		{
			name:     "Generic QuickbaseError is not retryable",
			err:      &QuickbaseError{Message: "Unknown error"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryableError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		body           string
		headers        map[string]string
		expectedType   string
		expectedStatus int
	}{
		{
			name:           "400 returns ValidationError",
			statusCode:     400,
			body:           `{"message": "Invalid request"}`,
			expectedType:   "*core.ValidationError",
			expectedStatus: 400,
		},
		{
			name:           "401 returns AuthenticationError",
			statusCode:     401,
			body:           `{"message": "Invalid token"}`,
			expectedType:   "*core.AuthenticationError",
			expectedStatus: 401,
		},
		{
			name:           "403 returns AuthorizationError",
			statusCode:     403,
			body:           `{"message": "Access denied"}`,
			expectedType:   "*core.AuthorizationError",
			expectedStatus: 403,
		},
		{
			name:           "404 returns NotFoundError",
			statusCode:     404,
			body:           `{"message": "Table not found"}`,
			expectedType:   "*core.NotFoundError",
			expectedStatus: 404,
		},
		{
			name:       "429 returns RateLimitError",
			statusCode: 429,
			body:       `{"message": "Too many requests"}`,
			headers: map[string]string{
				"Retry-After": "30",
			},
			expectedType:   "*core.RateLimitError",
			expectedStatus: 429,
		},
		{
			name:           "500 returns ServerError",
			statusCode:     500,
			body:           `{"message": "Internal server error"}`,
			expectedType:   "*core.ServerError",
			expectedStatus: 500,
		},
		{
			name:           "502 returns ServerError",
			statusCode:     502,
			body:           `{"message": "Bad gateway"}`,
			expectedType:   "*core.ServerError",
			expectedStatus: 502,
		},
		{
			name:           "503 returns ServerError",
			statusCode:     503,
			body:           `{"message": "Service unavailable"}`,
			expectedType:   "*core.ServerError",
			expectedStatus: 503,
		},
		{
			name:           "unknown 4xx returns QuickbaseError",
			statusCode:     418,
			body:           `{"message": "I'm a teapot"}`,
			expectedType:   "*core.QuickbaseError",
			expectedStatus: 418,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     header,
				Body:       io.NopCloser(bytes.NewBufferString(tt.body)),
			}

			err := ParseErrorResponse(resp, "https://api.quickbase.com/v1/test")

			// Check error type
			switch tt.expectedType {
			case "*core.ValidationError":
				if _, ok := err.(*ValidationError); !ok {
					t.Errorf("expected *ValidationError, got %T", err)
				}
			case "*core.AuthenticationError":
				if _, ok := err.(*AuthenticationError); !ok {
					t.Errorf("expected *AuthenticationError, got %T", err)
				}
			case "*core.AuthorizationError":
				if _, ok := err.(*AuthorizationError); !ok {
					t.Errorf("expected *AuthorizationError, got %T", err)
				}
			case "*core.NotFoundError":
				if _, ok := err.(*NotFoundError); !ok {
					t.Errorf("expected *NotFoundError, got %T", err)
				}
			case "*core.RateLimitError":
				if rle, ok := err.(*RateLimitError); !ok {
					t.Errorf("expected *RateLimitError, got %T", err)
				} else if rle.RetryAfter != 30 {
					t.Errorf("RetryAfter = %d, want 30", rle.RetryAfter)
				}
			case "*core.ServerError":
				if _, ok := err.(*ServerError); !ok {
					t.Errorf("expected *ServerError, got %T", err)
				}
			case "*core.QuickbaseError":
				if _, ok := err.(*QuickbaseError); !ok {
					t.Errorf("expected *QuickbaseError, got %T", err)
				}
			}
		})
	}
}

func TestParseErrorResponseWithRayID(t *testing.T) {
	t.Run("extracts qb-api-ray header", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 500,
			Header: http.Header{
				"Qb-Api-Ray": []string{"qb-ray-123"},
			},
			Body: io.NopCloser(bytes.NewBufferString(`{"message": "Error"}`)),
		}

		err := ParseErrorResponse(resp, "")
		serverErr := err.(*ServerError)
		if serverErr.RayID != "qb-ray-123" {
			t.Errorf("RayID = %q, want 'qb-ray-123'", serverErr.RayID)
		}
	})

	t.Run("falls back to cf-ray header", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 500,
			Header: http.Header{
				"Cf-Ray": []string{"cf-ray-456"},
			},
			Body: io.NopCloser(bytes.NewBufferString(`{"message": "Error"}`)),
		}

		err := ParseErrorResponse(resp, "")
		serverErr := err.(*ServerError)
		if serverErr.RayID != "cf-ray-456" {
			t.Errorf("RayID = %q, want 'cf-ray-456'", serverErr.RayID)
		}
	})
}
