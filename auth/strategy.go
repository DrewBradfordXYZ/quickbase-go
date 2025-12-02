// Package auth provides authentication strategies for the QuickBase API.
//
// QuickBase supports three authentication methods:
//
//   - User Token: Long-lived tokens for server-side applications
//   - Temporary Token: Short-lived tokens received from QuickBase (e.g., POST callbacks)
//   - SSO Token: SAML-based tokens for enterprise SSO integration
//
// User token authentication (most common for server-side):
//
//	strategy := auth.NewUserTokenStrategy("your-user-token")
//
// Temporary token authentication (for tokens received from QuickBase):
//
//	// Extract token from POST callback
//	token, _ := auth.ExtractPostTempToken(r)
//	strategy := auth.NewTempTokenStrategy(realm,
//	    auth.WithInitialTempToken(token),
//	)
//
// SSO token authentication:
//
//	strategy := auth.NewSSOTokenStrategy(samlToken, realm)
package auth

import (
	"context"
	"net/http"
)

// Strategy defines the interface for authentication strategies.
//
// All authentication strategies must implement this interface.
// The SDK provides three built-in implementations:
//   - [UserTokenStrategy]: For user token authentication
//   - [TempTokenStrategy]: For temporary token authentication
//   - [SSOTokenStrategy]: For SSO/SAML authentication
type Strategy interface {
	// GetToken returns the authentication token for the given table/app ID.
	// The dbid parameter is used for temp tokens which are scoped to specific tables.
	// For user tokens, dbid is ignored.
	GetToken(ctx context.Context, dbid string) (string, error)

	// ApplyAuth applies authentication headers to the request.
	// This typically sets the Authorization header with the token.
	ApplyAuth(req *http.Request, token string)

	// HandleAuthError handles authentication errors and potentially refreshes tokens.
	// Returns a new token if refresh was successful, empty string otherwise.
	// This is called when the API returns 401 Unauthorized.
	HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error)
}
