// Package core provides shared types and utilities for the QuickBase SDK.
//
// This package contains:
//   - Error types for different HTTP status codes (400, 401, 403, 404, 429, 5xx)
//   - Date parsing and transformation utilities
//   - Logging utilities
//
// Error types can be used for type assertions to handle specific error cases:
//
//	resp, err := client.API().GetAppWithResponse(ctx, appId)
//	if err != nil {
//	    var notFound *core.NotFoundError
//	    if errors.As(err, &notFound) {
//	        // Handle 404
//	    }
//	}
package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// QuickbaseError is the base error type for all QuickBase SDK errors.
//
// All specific error types (RateLimitError, NotFoundError, etc.) embed this type.
// The RayID field can be used for debugging with QuickBase support.
type QuickbaseError struct {
	Message     string `json:"message"`
	StatusCode  int    `json:"statusCode"`
	Description string `json:"description,omitempty"`
	RayID       string `json:"rayId,omitempty"`
	Cause       error  `json:"-"`
}

func (e *QuickbaseError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("%s: %s (status: %d)", e.Message, e.Description, e.StatusCode)
	}
	return fmt.Sprintf("%s (status: %d)", e.Message, e.StatusCode)
}

func (e *QuickbaseError) Unwrap() error {
	return e.Cause
}

// RateLimitInfo contains information about a rate limit event.
//
// This is passed to the OnRateLimit callback and included in RateLimitError.
// The RetryAfter field indicates how long to wait before retrying (in seconds).
type RateLimitInfo struct {
	Timestamp  time.Time `json:"timestamp"`
	RequestURL string    `json:"requestUrl"`
	HTTPStatus int       `json:"httpStatus"`
	RetryAfter int       `json:"retryAfter,omitempty"` // seconds
	CFRay      string    `json:"cfRay,omitempty"`
	TID        string    `json:"tid,omitempty"`
	QBAPIRay   string    `json:"qbApiRay,omitempty"`
	Attempt    int       `json:"attempt"`
}

// RateLimitError is returned when the API returns HTTP 429.
type RateLimitError struct {
	QuickbaseError
	RetryAfter    int           `json:"retryAfter,omitempty"`
	RateLimitInfo RateLimitInfo `json:"rateLimitInfo"`
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited, retry after %d seconds", e.RetryAfter)
	}
	return "rate limited"
}

// NewRateLimitError creates a new RateLimitError from rate limit info.
func NewRateLimitError(info RateLimitInfo, message string) *RateLimitError {
	if message == "" {
		if info.RetryAfter > 0 {
			message = fmt.Sprintf("Rate limited. Retry after %d seconds", info.RetryAfter)
		} else {
			message = "Rate limited"
		}
	}
	rayID := info.QBAPIRay
	if rayID == "" {
		rayID = info.CFRay
	}
	return &RateLimitError{
		QuickbaseError: QuickbaseError{
			Message:    message,
			StatusCode: 429,
			RayID:      rayID,
		},
		RetryAfter:    info.RetryAfter,
		RateLimitInfo: info,
	}
}

// AuthenticationError is returned when authentication fails (HTTP 401).
type AuthenticationError struct {
	QuickbaseError
}

// NewAuthenticationError creates a new AuthenticationError.
func NewAuthenticationError(message string, rayID string) *AuthenticationError {
	return &AuthenticationError{
		QuickbaseError: QuickbaseError{
			Message:    message,
			StatusCode: 401,
			RayID:      rayID,
		},
	}
}

// AuthorizationError is returned when authorization fails (HTTP 403).
type AuthorizationError struct {
	QuickbaseError
}

// NewAuthorizationError creates a new AuthorizationError.
func NewAuthorizationError(message string, rayID string) *AuthorizationError {
	return &AuthorizationError{
		QuickbaseError: QuickbaseError{
			Message:    message,
			StatusCode: 403,
			RayID:      rayID,
		},
	}
}

// NotFoundError is returned when a resource is not found (HTTP 404).
type NotFoundError struct {
	QuickbaseError
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(message string, rayID string) *NotFoundError {
	return &NotFoundError{
		QuickbaseError: QuickbaseError{
			Message:    message,
			StatusCode: 404,
			RayID:      rayID,
		},
	}
}

// ValidationError is returned for bad requests (HTTP 400).
type ValidationError struct {
	QuickbaseError
	Errors []FieldError `json:"errors,omitempty"`
}

// FieldError represents a validation error for a specific field.
type FieldError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// NewValidationError creates a new ValidationError.
func NewValidationError(message string, rayID string, errors []FieldError) *ValidationError {
	return &ValidationError{
		QuickbaseError: QuickbaseError{
			Message:    message,
			StatusCode: 400,
			RayID:      rayID,
		},
		Errors: errors,
	}
}

// TimeoutError is returned when a request times out.
type TimeoutError struct {
	QuickbaseError
	TimeoutMs int `json:"timeoutMs"`
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("request timed out after %dms", e.TimeoutMs)
}

// NewTimeoutError creates a new TimeoutError.
func NewTimeoutError(timeoutMs int) *TimeoutError {
	return &TimeoutError{
		QuickbaseError: QuickbaseError{
			Message:    fmt.Sprintf("Request timed out after %dms", timeoutMs),
			StatusCode: 0,
		},
		TimeoutMs: timeoutMs,
	}
}

// ServerError is returned for server errors (HTTP 5xx).
type ServerError struct {
	QuickbaseError
}

// NewServerError creates a new ServerError.
func NewServerError(statusCode int, message string, rayID string) *ServerError {
	return &ServerError{
		QuickbaseError: QuickbaseError{
			Message:    message,
			StatusCode: statusCode,
			RayID:      rayID,
		},
	}
}

// ParseErrorResponse parses an HTTP response into an appropriate error type.
func ParseErrorResponse(resp *http.Response, requestURL string) error {
	rayID := resp.Header.Get("qb-api-ray")
	if rayID == "" {
		rayID = resp.Header.Get("cf-ray")
	}

	var body struct {
		Message     string       `json:"message"`
		Description string       `json:"description"`
		Errors      []FieldError `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		body.Message = resp.Status
	}

	message := body.Message
	if message == "" {
		message = resp.Status
	}

	switch resp.StatusCode {
	case 400:
		return NewValidationError(message, rayID, body.Errors)
	case 401:
		return NewAuthenticationError(message, rayID)
	case 403:
		return NewAuthorizationError(message, rayID)
	case 404:
		return NewNotFoundError(message, rayID)
	case 429:
		retryAfter := 0
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			retryAfter, _ = strconv.Atoi(ra)
		}
		info := RateLimitInfo{
			Timestamp:  time.Now(),
			RequestURL: requestURL,
			HTTPStatus: 429,
			RetryAfter: retryAfter,
			CFRay:      resp.Header.Get("cf-ray"),
			TID:        resp.Header.Get("tid"),
			QBAPIRay:   resp.Header.Get("qb-api-ray"),
			Attempt:    1,
		}
		return NewRateLimitError(info, message)
	default:
		if resp.StatusCode >= 500 {
			return NewServerError(resp.StatusCode, message, rayID)
		}
		return &QuickbaseError{
			Message:     message,
			StatusCode:  resp.StatusCode,
			Description: body.Description,
			RayID:       rayID,
		}
	}
}

// IsRetryableError returns true if the error should trigger a retry.
func IsRetryableError(err error) bool {
	switch err.(type) {
	case *RateLimitError:
		return true
	case *ServerError:
		return true
	case *TimeoutError:
		return true
	}
	return false
}

// MissingTokenError is returned when a temp token is not available for a table.
//
// This error is useful for 401 negotiation: the server can catch this error
// and return the required table ID to the browser, which can then fetch
// the token and retry.
type MissingTokenError struct {
	DBID string `json:"dbid"`
}

func (e *MissingTokenError) Error() string {
	return fmt.Sprintf("no temp token available for table %s", e.DBID)
}

// NewMissingTokenError creates a new MissingTokenError.
func NewMissingTokenError(dbid string) *MissingTokenError {
	return &MissingTokenError{DBID: dbid}
}
