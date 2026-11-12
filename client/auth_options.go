package client

import "github.com/DrewBradfordXYZ/quickbase-go/v2/auth"

// Auth options. New's authStrategy argument, when not nil, takes precedence.

// WithAuth sets the auth strategy: any [auth.Strategy].
func WithAuth(strategy auth.Strategy) Option {
	return func(c *Client) {
		c.authFor = func(string) auth.Strategy { return strategy }
	}
}

// withAuthFor sets a strategy built from the realm when the client is created.
func withAuthFor(build func(realm string) auth.Strategy) Option {
	return func(c *Client) {
		c.authFor = build
	}
}

// WithUserToken configures user token authentication.
func WithUserToken(token string) Option {
	return WithAuth(auth.NewUserTokenStrategy(token))
}

// WithTempTokenAuth configures temporary token authentication.
//
// Temp tokens are short-lived (~5 min), table-scoped tokens that verify a user
// is logged into QuickBase. Go servers receive these tokens from browser clients
// (e.g., Code Pages) that can fetch them using the user's browser session.
//
// For the simpler map-based API, see [WithTempTokens].
//
// Deprecated: Use [WithTempTokens] instead for clearer token-to-table mapping.
func WithTempTokenAuth(opts ...auth.TempTokenOption) Option {
	return withAuthFor(func(realm string) auth.Strategy { return auth.NewTempTokenStrategy(realm, opts...) })
}

// WithTempTokens configures temporary token authentication with a map of tokens.
//
// This is the preferred way to configure temp token auth when you have tokens
// mapped to table IDs (dbids).
//
// Example:
//
//	client, err := quickbase.New("myrealm",
//	    quickbase.WithTempTokens(map[string]string{
//	        "bqxyz123": tokenForTable1,
//	        "bqabc456": tokenForTable2,
//	    }),
//	)
func WithTempTokens(tokens map[string]string) Option {
	return withAuthFor(func(realm string) auth.Strategy {
		return auth.NewTempTokenStrategy(realm, auth.WithTempTokens(tokens))
	})
}

// WithSSOTokenAuth configures SSO (SAML) token authentication.
//
// SSO authentication lets your Go server make API calls as a specific QuickBase
// user rather than a shared service account. The SDK exchanges a SAML assertion
// for a QuickBase temp token using RFC 8693 token exchange.
//
// Benefits:
//   - Audit accuracy: "Created By" and "Modified By" show the actual user
//   - Security: No long-lived user token; each user gets a short-lived token
//   - Per-user permissions: API calls respect each user's individual QuickBase permissions
//
// Prerequisites:
//   - Your QuickBase realm has SAML SSO configured
//   - Your identity provider (Okta, Azure AD, etc.) can generate SAML assertions
//
// Example:
//
//	// Get SAML assertion from your IdP for the authenticated user
//	samlAssertion := getAssertionFromIdP(userId) // base64url-encoded
//
//	client, err := quickbase.New("myrealm",
//	    quickbase.WithSSOTokenAuth(samlAssertion),
//	)
//
//	// API calls are now made as that specific user
//
// See https://developer.quickbase.com/operation/exchangeSsoToken
func WithSSOTokenAuth(samlToken string, opts ...auth.SSOTokenOption) Option {
	return withAuthFor(func(realm string) auth.Strategy { return auth.NewSSOTokenStrategy(samlToken, realm, opts...) })
}

// WithTicketAuth configures ticket authentication using username/password.
//
// This calls the XML API_Authenticate endpoint to obtain a ticket, which is then
// used with REST API calls. Unlike user tokens, tickets properly attribute record
// changes (createdBy/modifiedBy) to the authenticated user.
//
// The password is used once for authentication and then discarded from memory.
// When the ticket expires (default 12 hours), an AuthenticationError is returned
// and a new client must be created with fresh credentials.
//
// Example:
//
//	qb, err := quickbase.New("myrealm",
//	    quickbase.WithTicketAuth("user@example.com", "password"),
//	)
//
// With custom ticket validity (max ~6 months):
//
//	qb, err := quickbase.New("myrealm",
//	    quickbase.WithTicketAuth("user@example.com", "password",
//	        auth.WithTicketHours(24*7), // 1 week
//	    ),
//	)
//
// XML-API-TICKET: Remove this function if XML API is discontinued.
func WithTicketAuth(username, password string, opts ...auth.TicketOption) Option {
	return withAuthFor(func(realm string) auth.Strategy {
		return auth.NewTicketStrategy(username, password, realm, opts...)
	})
}

// WithTicket configures authentication using a pre-existing ticket.
//
// Use this when the ticket was obtained elsewhere (e.g., by a browser client
// calling API_Authenticate directly). The server never sees user credentials.
//
// This is the recommended approach for Code Page cookie mode:
//  1. Browser calls QuickBase API_Authenticate with user credentials
//  2. Browser sends ticket to Go server
//  3. Server stores encrypted ticket in HttpOnly cookie
//  4. Server uses WithTicket for API calls
//
// Example:
//
//	qb, err := quickbase.New("myrealm",
//	    quickbase.WithTicket(ticketFromCookie),
//	)
//
// XML-API-TICKET: Remove this function if XML API is discontinued.
func WithTicket(ticket string) Option {
	return WithAuth(auth.NewExistingTicketStrategy(ticket))
}
