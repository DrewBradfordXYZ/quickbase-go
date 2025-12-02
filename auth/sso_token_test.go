package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestSSOTokenStrategy_RequestFormat verifies the token exchange request
// matches the QuickBase API documentation exactly.
func TestSSOTokenStrategy_RequestFormat(t *testing.T) {
	var capturedReq struct {
		Method      string
		Path        string
		ContentType string
		RealmHeader string
		Body        map[string]string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq.Method = r.Method
		capturedReq.Path = r.URL.Path
		capturedReq.ContentType = r.Header.Get("Content-Type")
		capturedReq.RealmHeader = r.Header.Get("QB-Realm-Hostname")

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedReq.Body)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token":      "test_token_abc123",
			"issued_token_type": "urn:quickbase:params:oauth:token-type:temp_token",
			"token_type":        "N_A",
		})
	}))
	defer server.Close()

	strategy := &SSOTokenStrategy{
		samlToken: "PHNhbWxwOlJlc3BvbnNlIHhtbG5zOnNhbWxwPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6cHJvdG9jb2wiPjwvc2FtbHA6UmVzcG9uc2U+",
		realm:     "testrealm",
		baseURL:   server.URL,
		client:    server.Client(),
	}

	token, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	// Verify the request format matches documentation
	if capturedReq.Method != "POST" {
		t.Errorf("Method = %q, want POST", capturedReq.Method)
	}

	if capturedReq.Path != "/auth/oauth/token" {
		t.Errorf("Path = %q, want /auth/oauth/token", capturedReq.Path)
	}

	if capturedReq.ContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", capturedReq.ContentType)
	}

	if capturedReq.RealmHeader != "testrealm.quickbase.com" {
		t.Errorf("QB-Realm-Hostname = %q, want testrealm.quickbase.com", capturedReq.RealmHeader)
	}

	// Verify request body matches RFC 8693 token exchange format
	expectedBody := map[string]string{
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"requested_token_type": "urn:quickbase:params:oauth:token-type:temp_token",
		"subject_token":        strategy.samlToken,
		"subject_token_type":   "urn:ietf:params:oauth:token-type:saml2",
	}

	for key, want := range expectedBody {
		if got := capturedReq.Body[key]; got != want {
			t.Errorf("Body[%q] = %q, want %q", key, got, want)
		}
	}

	if token != "test_token_abc123" {
		t.Errorf("token = %q, want test_token_abc123", token)
	}
}

// TestSSOTokenStrategy_ResponseParsing verifies we correctly parse the token response.
func TestSSOTokenStrategy_ResponseParsing(t *testing.T) {
	tests := []struct {
		name         string
		response     map[string]string
		wantToken    string
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "valid response with temp_token type",
			response: map[string]string{
				"access_token":      "valid_token_xyz",
				"issued_token_type": "urn:quickbase:params:oauth:token-type:temp_token",
				"token_type":        "N_A",
			},
			wantToken: "valid_token_xyz",
		},
		{
			name: "valid response with temp_ticket type",
			response: map[string]string{
				"access_token":      "ticket_token_abc",
				"issued_token_type": "urn:quickbase:params:oauth:token-type:temp_ticket",
				"token_type":        "N_A",
			},
			wantToken: "ticket_token_abc",
		},
		{
			name: "empty access_token",
			response: map[string]string{
				"access_token":      "",
				"issued_token_type": "urn:quickbase:params:oauth:token-type:temp_token",
				"token_type":        "N_A",
			},
			wantErr:    true,
			wantErrMsg: "no access token returned",
		},
		{
			name:       "missing access_token field",
			response:   map[string]string{},
			wantErr:    true,
			wantErrMsg: "no access token returned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			strategy := &SSOTokenStrategy{
				samlToken: "test_saml_token",
				realm:     "testrealm",
				baseURL:   server.URL,
				client:    server.Client(),
			}

			token, err := strategy.GetToken(context.Background(), "bqxyz123")

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

// TestSSOTokenStrategy_ErrorHandling verifies error responses are handled correctly.
func TestSSOTokenStrategy_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   map[string]string
		wantErrMsg string
	}{
		{
			name:       "400 bad request with message",
			statusCode: 400,
			response:   map[string]string{"message": "Invalid SAML assertion"},
			wantErrMsg: "Invalid SAML assertion",
		},
		{
			name:       "401 unauthorized",
			statusCode: 401,
			response:   map[string]string{"message": "SAML assertion expired"},
			wantErrMsg: "SAML assertion expired",
		},
		{
			name:       "403 forbidden",
			statusCode: 403,
			response:   map[string]string{"message": "SSO not configured for realm"},
			wantErrMsg: "SSO not configured for realm",
		},
		{
			name:       "500 server error without message",
			statusCode: 500,
			response:   map[string]string{},
			wantErrMsg: "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			strategy := &SSOTokenStrategy{
				samlToken: "test_saml_token",
				realm:     "testrealm",
				baseURL:   server.URL,
				client:    server.Client(),
			}

			_, err := strategy.GetToken(context.Background(), "bqxyz123")
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErrMsg)
			}

			if !contains(err.Error(), "status:") {
				t.Errorf("error should contain status code, got: %q", err.Error())
			}
		})
	}
}

// TestSSOTokenStrategy_TokenCaching verifies the token is cached after first fetch.
func TestSSOTokenStrategy_TokenCaching(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "cached_token",
		})
	}))
	defer server.Close()

	strategy := &SSOTokenStrategy{
		samlToken: "test_saml_token",
		realm:     "testrealm",
		baseURL:   server.URL,
		client:    server.Client(),
	}

	// First call should hit the server
	token1, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err != nil {
		t.Fatalf("first GetToken failed: %v", err)
	}

	// Second call should use cached token
	token2, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err != nil {
		t.Fatalf("second GetToken failed: %v", err)
	}

	if token1 != token2 {
		t.Errorf("tokens differ: %q vs %q", token1, token2)
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("server called %d times, want 1 (token should be cached)", callCount)
	}
}

// TestSSOTokenStrategy_ApplyAuth verifies the token is applied correctly.
func TestSSOTokenStrategy_ApplyAuth(t *testing.T) {
	strategy := NewSSOTokenStrategy("saml_token", "testrealm")

	req := httptest.NewRequest("GET", "https://api.quickbase.com/v1/apps/bqxyz123", nil)
	strategy.ApplyAuth(req, "my_temp_token")

	got := req.Header.Get("Authorization")
	want := "QB-TEMP-TOKEN my_temp_token"

	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

// TestSSOTokenStrategy_HandleAuthError verifies 401 errors trigger token refresh.
func TestSSOTokenStrategy_HandleAuthError(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "token_" + string(rune('a'+count-1)),
		})
	}))
	defer server.Close()

	strategy := &SSOTokenStrategy{
		samlToken: "test_saml_token",
		realm:     "testrealm",
		baseURL:   server.URL,
		client:    server.Client(),
	}

	// Get initial token
	token1, _ := strategy.GetToken(context.Background(), "bqxyz123")
	if token1 != "token_a" {
		t.Errorf("initial token = %q, want token_a", token1)
	}

	// Simulate 401 error - should clear and refresh
	newToken, err := strategy.HandleAuthError(context.Background(), 401, "bqxyz123", 0, 3)
	if err != nil {
		t.Fatalf("HandleAuthError failed: %v", err)
	}

	if newToken != "token_b" {
		t.Errorf("refreshed token = %q, want token_b", newToken)
	}

	// Non-401 errors should not refresh
	noToken, _ := strategy.HandleAuthError(context.Background(), 403, "bqxyz123", 0, 3)
	if noToken != "" {
		t.Errorf("403 error returned token %q, want empty", noToken)
	}

	// Last attempt should not refresh
	noToken, _ = strategy.HandleAuthError(context.Background(), 401, "bqxyz123", 2, 3)
	if noToken != "" {
		t.Errorf("last attempt returned token %q, want empty", noToken)
	}
}

// TestNewSSOTokenStrategy_Options verifies option functions work.
func TestNewSSOTokenStrategy_Options(t *testing.T) {
	customClient := &http.Client{}

	strategy := NewSSOTokenStrategy("saml_token", "myrealm",
		WithSSOHTTPClient(customClient),
	)

	if strategy.client != customClient {
		t.Error("custom HTTP client was not applied")
	}

	if strategy.realm != "myrealm" {
		t.Errorf("realm = %q, want myrealm", strategy.realm)
	}

	if strategy.samlToken != "saml_token" {
		t.Errorf("samlToken = %q, want saml_token", strategy.samlToken)
	}

	if strategy.baseURL != "https://api.quickbase.com/v1" {
		t.Errorf("baseURL = %q, want https://api.quickbase.com/v1", strategy.baseURL)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
