// Package auth provides authentication strategies for the QuickBase API.
//
// QuickBase supports four authentication methods:
//
//   - User Token: Long-lived tokens for server-side applications
//   - Temporary Token: Short-lived tokens received from QuickBase (e.g., POST callbacks)
//   - SSO Token: SAML-based tokens for enterprise SSO integration
//   - Ticket: Username/password authentication with proper user attribution
//
// # User Token (Recommended for Server-Side)
//
// User tokens are long-lived tokens that don't expire. Generate one at:
// https://YOUR-REALM.quickbase.com/db/main?a=UserTokens
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithUserToken("b9f3pk_xxxx_xxxxxxxxxxxxxxx"),
//	)
//
// # Temporary Token (POST Callbacks)
//
// Temp tokens are short-lived (~5 min), table-scoped tokens. Go servers receive
// them from QuickBase via POST callbacks (Formula-URL fields), not by fetching.
//
//	token, _ := auth.ExtractPostTempToken(r)
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithTempTokenAuth(auth.WithInitialTempToken(token)),
//	)
//
// See: https://help.quickbase.com/docs/post-temporary-token-from-a-quickbase-field
//
// # SSO Token (SAML)
//
// Exchange a SAML assertion for a QuickBase temp token. Requires SAML SSO
// configured on your realm.
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithSSOTokenAuth(samlAssertion),
//	)
//
// See: https://developer.quickbase.com/operation/exchangeSsoToken
//
// # Ticket Authentication (Username/Password)
//
// Ticket auth uses the legacy XML API (API_Authenticate) to exchange credentials
// for an authentication ticket. Unlike user tokens, tickets properly attribute
// record changes (createdBy/modifiedBy) to the authenticated user.
//
//	client, _ := quickbase.New("myrealm",
//	    quickbase.WithTicketAuth("user@example.com", "password"),
//	)
//
// The XML API call:
//
//	POST https://{realm}.quickbase.com/db/main
//	QUICKBASE-ACTION: API_Authenticate
//	Content-Type: application/xml
//
//	<qdbapi>
//	    <username>user@example.com</username>
//	    <password>secret</password>
//	    <hours>12</hours>
//	</qdbapi>
//
// Returns a ticket valid for 12 hours (configurable up to ~6 months). The ticket
// is then used with REST API calls via the QB-TICKET authorization header.
//
// See: https://help.quickbase.com/docs/api-authenticate
package auth

import (
	"context"
	"net/http"
)

// Strategy defines the interface for authentication strategies.
//
// All authentication strategies must implement this interface.
// The SDK provides four built-in implementations:
//   - [UserTokenStrategy]: For user token authentication
//   - [TempTokenStrategy]: For temporary token authentication
//   - [SSOTokenStrategy]: For SSO/SAML authentication
//   - [TicketStrategy]: For username/password authentication with proper user attribution
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
