package auth

import (
	"context"
	"net/http"
)

// UserTokenStrategy authenticates using a QuickBase user token.
//
// User tokens are long-lived tokens that don't expire and work across all apps
// the user has access to. This is the simplest and most common authentication method.
//
// Generate a user token at: https://YOUR-REALM.quickbase.com/db/main?a=UserTokens
type UserTokenStrategy struct {
	token string
}

// NewUserTokenStrategy creates a new user token authentication strategy.
//
// Example:
//
//	strategy := auth.NewUserTokenStrategy("b9f3pk_xxx_xxxxxxxxxxxxxx")
func NewUserTokenStrategy(token string) *UserTokenStrategy {
	return &UserTokenStrategy{token: token}
}

// GetToken returns the user token (ignores dbid since user tokens are global).
func (s *UserTokenStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	return s.token, nil
}

// ApplyAuth applies the user token to the Authorization header.
func (s *UserTokenStrategy) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-USER-TOKEN "+token)
}

// HandleAuthError handles 401 errors by returning the same token for retry.
// User tokens can't be refreshed, so we just retry with the same token.
func (s *UserTokenStrategy) HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error) {
	if statusCode != http.StatusUnauthorized || attempt >= maxAttempts-1 {
		return "", nil
	}
	// Return the same token for retry - user tokens can't be refreshed
	return s.token, nil
}
