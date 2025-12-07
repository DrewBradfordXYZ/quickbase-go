package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// SSOTokenStrategy authenticates using SAML SSO token exchange.
// This strategy exchanges a SAML assertion for a QuickBase temp token.
type SSOTokenStrategy struct {
	samlToken string
	realm     string
	baseURL   string
	client    *http.Client

	mu           sync.RWMutex
	currentToken string
	pending      chan struct{}
}

// SSOTokenOption configures an SSOTokenStrategy.
type SSOTokenOption func(*SSOTokenStrategy)

// WithSSOHTTPClient sets a custom HTTP client for token exchange.
func WithSSOHTTPClient(client *http.Client) SSOTokenOption {
	return func(s *SSOTokenStrategy) {
		s.client = client
	}
}

// NewSSOTokenStrategy creates a new SSO token authentication strategy.
func NewSSOTokenStrategy(samlToken, realm string, opts ...SSOTokenOption) *SSOTokenStrategy {
	s := &SSOTokenStrategy{
		samlToken: samlToken,
		realm:     realm,
		baseURL:   "https://api.quickbase.com/v1",
		client:    http.DefaultClient,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetToken returns the SSO-derived token, exchanging the SAML token if needed.
func (s *SSOTokenStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	s.mu.RLock()
	if s.currentToken != "" {
		token := s.currentToken
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	// Acquire write lock and re-check state (double-check locking pattern)
	s.mu.Lock()

	// Re-check token - another goroutine may have set it
	if s.currentToken != "" {
		token := s.currentToken
		s.mu.Unlock()
		return token, nil
	}

	// Check if there's a pending fetch
	if s.pending != nil {
		pending := s.pending
		s.mu.Unlock()
		<-pending
		return s.GetToken(ctx, dbid)
	}

	s.pending = make(chan struct{})
	s.mu.Unlock()

	token, err := s.exchangeToken(ctx)

	s.mu.Lock()
	if err == nil {
		s.currentToken = token
	}
	close(s.pending)
	s.pending = nil
	s.mu.Unlock()

	return token, err
}

func (s *SSOTokenStrategy) exchangeToken(ctx context.Context) (string, error) {
	payload := map[string]string{
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"requested_token_type": "urn:quickbase:params:oauth:token-type:temp_token",
		"subject_token":        s.samlToken,
		"subject_token_type":   "urn:ietf:params:oauth:token-type:saml2",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	url := s.baseURL + "/auth/oauth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("QB-Realm-Hostname", s.realm+".quickbase.com")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchanging SSO token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Message string `json:"message"`
		}
		msg := "unknown error"
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Message != "" {
			msg = errResp.Message
		}
		return "", fmt.Errorf("SSO token exchange failed: %s (status: %d)", msg, resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token returned from SSO token exchange")
	}

	return result.AccessToken, nil
}

// ApplyAuth applies the SSO-derived token to the Authorization header.
func (s *SSOTokenStrategy) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-TEMP-TOKEN "+token)
}

// HandleAuthError handles 401 errors by refreshing the SSO token.
func (s *SSOTokenStrategy) HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error) {
	if statusCode != http.StatusUnauthorized || attempt >= maxAttempts-1 {
		return "", nil
	}

	// Clear current token
	s.mu.Lock()
	s.currentToken = ""
	s.mu.Unlock()

	// Fetch a new token
	return s.GetToken(ctx, dbid)
}
