package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TempTokenStrategy authenticates using QuickBase temporary tokens.
//
// Temp tokens are short-lived (~5 min), table-scoped tokens that verify
// a user is logged into QuickBase via their browser session.
//
// In a Go server, you receive temp tokens from QuickBase (e.g., via POST
// from a Formula-URL field) rather than fetching them. Use ExtractPostTempToken
// to extract tokens from incoming requests.
//
// Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    token, _ := auth.ExtractPostTempToken(r)
//	    client, _ := quickbase.New("realm",
//	        quickbase.WithTempTokenAuth(auth.WithInitialTempToken(token)),
//	    )
//	    // Use client...
//	}
type TempTokenStrategy struct {
	realm    string
	lifespan time.Duration

	mu    sync.RWMutex
	cache map[string]*cachedToken
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

// TempTokenOption configures a TempTokenStrategy.
type TempTokenOption func(*TempTokenStrategy)

// WithTempTokenLifespan sets the token cache lifespan (default 290 seconds).
// QuickBase temp tokens expire after ~5 minutes.
func WithTempTokenLifespan(d time.Duration) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.lifespan = d
	}
}

// initialTokenOption holds the initial token to set after construction.
type initialTokenOption struct {
	token string
	dbid  string
}

var pendingInitialTokens = make(map[*TempTokenStrategy]*initialTokenOption)
var pendingMu sync.Mutex

// WithInitialTempToken sets an initial temp token received from QuickBase.
//
// Use this when you've received a token from a POST callback (Formula-URL field)
// or from a browser client. The token will be cached and used for API requests.
//
// If dbid is not known at creation time, the token will be used for the first
// request and cached with that request's dbid.
func WithInitialTempToken(token string) TempTokenOption {
	return func(s *TempTokenStrategy) {
		pendingMu.Lock()
		pendingInitialTokens[s] = &initialTokenOption{token: token}
		pendingMu.Unlock()
	}
}

// WithInitialTempTokenForTable sets an initial temp token for a specific table.
func WithInitialTempTokenForTable(token string, dbid string) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.mu.Lock()
		s.cache[dbid] = &cachedToken{
			token:     token,
			expiresAt: time.Now().Add(s.lifespan),
		}
		s.mu.Unlock()
	}
}

// NewTempTokenStrategy creates a new temporary token authentication strategy.
func NewTempTokenStrategy(realm string, opts ...TempTokenOption) *TempTokenStrategy {
	s := &TempTokenStrategy{
		realm:    realm,
		lifespan: 290 * time.Second,
		cache:    make(map[string]*cachedToken),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetToken returns a temp token for the given table ID.
//
// Since Go servers can't fetch temp tokens (no browser cookies), this returns
// a cached token that was set via WithInitialTempToken or SetToken.
func (s *TempTokenStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	// Check for pending initial token (set via WithInitialTempToken)
	pendingMu.Lock()
	if pending, ok := pendingInitialTokens[s]; ok {
		delete(pendingInitialTokens, s)
		pendingMu.Unlock()

		// Cache it for this dbid
		if dbid != "" {
			s.mu.Lock()
			s.cache[dbid] = &cachedToken{
				token:     pending.token,
				expiresAt: time.Now().Add(s.lifespan),
			}
			s.mu.Unlock()
		}
		return pending.token, nil
	}
	pendingMu.Unlock()

	// Check cache
	s.mu.RLock()
	if cached, ok := s.cache[dbid]; ok && time.Now().Before(cached.expiresAt) {
		s.mu.RUnlock()
		return cached.token, nil
	}
	s.mu.RUnlock()

	return "", fmt.Errorf("no temp token available for dbid %s; use WithInitialTempToken or SetToken", dbid)
}

// SetToken caches a temp token for a specific table ID.
//
// Use this to add tokens received from QuickBase during the request lifecycle.
func (s *TempTokenStrategy) SetToken(dbid string, token string) {
	s.mu.Lock()
	s.cache[dbid] = &cachedToken{
		token:     token,
		expiresAt: time.Now().Add(s.lifespan),
	}
	s.mu.Unlock()
}

// ApplyAuth applies the temp token to the Authorization header.
func (s *TempTokenStrategy) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-TEMP-TOKEN "+token)
}

// HandleAuthError handles 401 errors. For temp tokens, we can't refresh
// server-side (no browser cookies), so we just invalidate the cache.
func (s *TempTokenStrategy) HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error) {
	if statusCode != http.StatusUnauthorized {
		return "", nil
	}

	// Invalidate the cached token
	if dbid != "" {
		s.mu.Lock()
		delete(s.cache, dbid)
		s.mu.Unlock()
	}

	// Can't refresh - no browser cookies on server
	return "", nil
}

// Invalidate removes a cached token.
func (s *TempTokenStrategy) Invalidate(dbid string) {
	s.mu.Lock()
	delete(s.cache, dbid)
	s.mu.Unlock()
}

// InvalidateAll removes all cached tokens.
func (s *TempTokenStrategy) InvalidateAll() {
	s.mu.Lock()
	s.cache = make(map[string]*cachedToken)
	s.mu.Unlock()
}
