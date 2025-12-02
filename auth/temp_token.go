package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TempTokenStrategy authenticates using QuickBase temporary tokens.
// Temp tokens are short-lived, table-scoped tokens obtained via browser session.
type TempTokenStrategy struct {
	realm    string
	baseURL  string
	client   *http.Client
	lifespan time.Duration

	mu           sync.RWMutex
	cache        map[string]*cachedToken
	pending      map[string]chan struct{}
	initialToken string
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

// TempTokenOption configures a TempTokenStrategy.
type TempTokenOption func(*TempTokenStrategy)

// WithTempTokenLifespan sets the token cache lifespan (default 290 seconds).
func WithTempTokenLifespan(d time.Duration) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.lifespan = d
	}
}

// WithInitialTempToken sets an initial temp token to use before fetching.
func WithInitialTempToken(token string) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.initialToken = token
	}
}

// WithHTTPClient sets a custom HTTP client for token fetching.
func WithHTTPClient(client *http.Client) TempTokenOption {
	return func(s *TempTokenStrategy) {
		s.client = client
	}
}

// NewTempTokenStrategy creates a new temporary token authentication strategy.
func NewTempTokenStrategy(realm string, opts ...TempTokenOption) *TempTokenStrategy {
	s := &TempTokenStrategy{
		realm:    realm,
		baseURL:  "https://api.quickbase.com/v1",
		client:   http.DefaultClient,
		lifespan: 290 * time.Second,
		cache:    make(map[string]*cachedToken),
		pending:  make(map[string]chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetToken returns a temp token for the given table ID, fetching if needed.
func (s *TempTokenStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	if dbid == "" {
		if s.initialToken != "" {
			return s.initialToken, nil
		}
		return "", fmt.Errorf("dbid required for temp token authentication")
	}

	// Check cache first
	s.mu.RLock()
	if cached, ok := s.cache[dbid]; ok && time.Now().Before(cached.expiresAt) {
		s.mu.RUnlock()
		return cached.token, nil
	}
	s.mu.RUnlock()

	// Check if there's already a pending fetch
	s.mu.Lock()
	if pending, ok := s.pending[dbid]; ok {
		s.mu.Unlock()
		<-pending // Wait for the pending fetch to complete
		return s.GetToken(ctx, dbid)
	}

	// Start a new fetch
	pending := make(chan struct{})
	s.pending[dbid] = pending
	s.mu.Unlock()

	token, err := s.fetchToken(ctx, dbid)

	s.mu.Lock()
	delete(s.pending, dbid)
	close(pending)
	if err == nil {
		s.cache[dbid] = &cachedToken{
			token:     token,
			expiresAt: time.Now().Add(s.lifespan),
		}
	}
	s.mu.Unlock()

	return token, err
}

func (s *TempTokenStrategy) fetchToken(ctx context.Context, dbid string) (string, error) {
	url := fmt.Sprintf("%s/auth/temporary/%s", s.baseURL, dbid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("QB-Realm-Hostname", s.realm+".quickbase.com")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching temp token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp.Message
		if msg == "" {
			msg = "unknown error"
		}
		return "", fmt.Errorf("API error: %s (status: %d)", msg, resp.StatusCode)
	}

	var result struct {
		TemporaryAuthorization string `json:"temporaryAuthorization"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if result.TemporaryAuthorization == "" {
		return "", fmt.Errorf("no temporary token returned from API")
	}

	return result.TemporaryAuthorization, nil
}

// ApplyAuth applies the temp token to the Authorization header.
func (s *TempTokenStrategy) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-TEMP-TOKEN "+token)
}

// HandleAuthError handles 401 errors by invalidating the cache and fetching a new token.
func (s *TempTokenStrategy) HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error) {
	if statusCode != http.StatusUnauthorized || attempt >= maxAttempts-1 {
		return "", nil
	}

	if dbid == "" {
		return "", nil
	}

	// Invalidate the cached token
	s.mu.Lock()
	delete(s.cache, dbid)
	s.mu.Unlock()

	// Fetch a new token
	return s.GetToken(ctx, dbid)
}
