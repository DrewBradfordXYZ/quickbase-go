package auth

import (
	"context"
	"net/http"
	"sync"

	"github.com/DrewBradfordXYZ/quickbase-go/v2/core"
)

// TempTokenStrategy authenticates using QuickBase temporary tokens.
//
// Temp tokens are short-lived (~5 min), table-scoped tokens that verify
// a user is logged into QuickBase via their browser session.
//
// Go servers receive temp tokens from browser clients (e.g., Code Pages)
// that fetch them using the user's QuickBase session. The browser sends
// tokens via HTTP headers (e.g., X-QB-Token-{dbid}).
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    tokens := map[string]string{
//	        "bqr1111": r.Header.Get("X-QB-Token-bqr1111"),
//	    }
//	    client, _ := quickbase.New("realm",
//	        quickbase.WithTempTokens(tokens),
//	    )
//	    // Use client...
//	}
type TempTokenStrategy struct {
	realm string

	mu           sync.RWMutex
	tokens       map[string]string // dbid → token
	pendingToken *string           // initial token not yet associated with a dbid
}

// TempTokenOption configures a TempTokenStrategy.
type TempTokenOption func(*TempTokenStrategy)

// WithInitialTempToken sets an initial temp token received from a browser client.
//
// Use this when you've received a single token and don't know the dbid yet.
// The token will be used for the first request and associated with that dbid.
//
// For multiple tokens with known dbids, use [WithTempTokens] instead.
func WithInitialTempToken(token string) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.pendingToken = &token
	}
}

// WithInitialTempTokenForTable sets an initial temp token for a specific table.
func WithInitialTempTokenForTable(token string, dbid string) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.mu.Lock()
		s.tokens[dbid] = token
		s.mu.Unlock()
	}
}

// WithTempTokens sets multiple temp tokens mapped by table ID.
//
// This is the preferred way to initialize tokens when you know the dbids.
func WithTempTokens(tokens map[string]string) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.mu.Lock()
		for dbid, token := range tokens {
			s.tokens[dbid] = token
		}
		s.mu.Unlock()
	}
}

// NewTempTokenStrategy creates a new temporary token authentication strategy.
func NewTempTokenStrategy(realm string, opts ...TempTokenOption) *TempTokenStrategy {
	s := &TempTokenStrategy{
		realm:  realm,
		tokens: make(map[string]string),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetToken returns a temp token for the given table ID.
//
// Since Go servers can't fetch temp tokens (no browser cookies), this returns
// a token that was set via WithInitialTempToken, WithTempTokens, or SetToken.
func (s *TempTokenStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for pending initial token (set via WithInitialTempToken)
	if s.pendingToken != nil {
		token := *s.pendingToken
		s.pendingToken = nil

		// Store it for this dbid
		if dbid != "" {
			s.tokens[dbid] = token
		}
		return token, nil
	}

	// Check tokens map
	if token, ok := s.tokens[dbid]; ok {
		return token, nil
	}

	return "", core.NewMissingTokenError(dbid)
}

// SetToken stores a temp token for a specific table ID.
//
// Use this to add tokens received from the browser during the request lifecycle.
func (s *TempTokenStrategy) SetToken(dbid string, token string) {
	s.mu.Lock()
	s.tokens[dbid] = token
	s.mu.Unlock()
}

// ApplyAuth applies the temp token to the Authorization header.
func (s *TempTokenStrategy) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-TEMP-TOKEN "+token)
}

// HandleAuthError handles 401 errors. For temp tokens, we can't refresh
// server-side (no browser cookies), so we just remove the invalid token.
func (s *TempTokenStrategy) HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error) {
	if statusCode != http.StatusUnauthorized {
		return "", nil
	}

	// Remove the invalid token
	if dbid != "" {
		s.mu.Lock()
		delete(s.tokens, dbid)
		s.mu.Unlock()
	}

	// Can't refresh - no browser cookies on server
	return "", nil
}

// Invalidate removes a token for a specific table.
func (s *TempTokenStrategy) Invalidate(dbid string) {
	s.mu.Lock()
	delete(s.tokens, dbid)
	s.mu.Unlock()
}

// InvalidateAll removes all tokens.
func (s *TempTokenStrategy) InvalidateAll() {
	s.mu.Lock()
	s.tokens = make(map[string]string)
	s.mu.Unlock()
}
