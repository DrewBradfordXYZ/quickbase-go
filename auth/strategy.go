// Package auth provides authentication strategies for the QuickBase API.
package auth

import (
	"context"
	"net/http"
)

// Strategy defines the interface for authentication strategies.
type Strategy interface {
	// GetToken returns the authentication token for the given table/app ID.
	// The dbid parameter is used for temp tokens which are scoped to specific tables.
	GetToken(ctx context.Context, dbid string) (string, error)

	// ApplyAuth applies authentication headers to the request.
	ApplyAuth(req *http.Request, token string)

	// HandleAuthError handles authentication errors and potentially refreshes tokens.
	// Returns a new token if refresh was successful, empty string otherwise.
	HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error)
}
