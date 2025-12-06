package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestTicketStrategy_RequestFormat verifies the API_Authenticate request
// matches the QuickBase XML API documentation exactly.
func TestTicketStrategy_RequestFormat(t *testing.T) {
	var capturedReq struct {
		Method      string
		Path        string
		ContentType string
		Action      string
		Body        string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq.Method = r.Method
		capturedReq.Path = r.URL.Path
		capturedReq.ContentType = r.Header.Get("Content-Type")
		capturedReq.Action = r.Header.Get("QUICKBASE-ACTION")

		body, _ := io.ReadAll(r.Body)
		capturedReq.Body = string(body)

		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?>
<qdbapi>
	<action>API_Authenticate</action>
	<errcode>0</errcode>
	<errtext>No error</errtext>
	<ticket>test_ticket_abc123</ticket>
	<userid>12345.abcd</userid>
</qdbapi>`))
	}))
	defer server.Close()

	// Extract host from server URL for realm simulation
	strategy := &TicketStrategy{
		username: "user@example.com",
		password: "secret123",
		realm:    "testrealm",
		hours:    24,
		client:   server.Client(),
	}

	// Override the authenticate method to use test server
	ticket, userID, err := strategy.authenticateWithURL(context.Background(), server.URL+"/db/main")
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}

	// Verify the request format matches documentation
	if capturedReq.Method != "POST" {
		t.Errorf("Method = %q, want POST", capturedReq.Method)
	}

	if capturedReq.Path != "/db/main" {
		t.Errorf("Path = %q, want /db/main", capturedReq.Path)
	}

	if capturedReq.ContentType != "application/xml" {
		t.Errorf("Content-Type = %q, want application/xml", capturedReq.ContentType)
	}

	if capturedReq.Action != "API_Authenticate" {
		t.Errorf("QUICKBASE-ACTION = %q, want API_Authenticate", capturedReq.Action)
	}

	// Verify request body contains expected elements
	if !strings.Contains(capturedReq.Body, "<username>user@example.com</username>") {
		t.Errorf("Body missing username, got: %s", capturedReq.Body)
	}
	if !strings.Contains(capturedReq.Body, "<password>secret123</password>") {
		t.Errorf("Body missing password, got: %s", capturedReq.Body)
	}
	if !strings.Contains(capturedReq.Body, "<hours>24</hours>") {
		t.Errorf("Body missing hours, got: %s", capturedReq.Body)
	}

	if ticket != "test_ticket_abc123" {
		t.Errorf("ticket = %q, want test_ticket_abc123", ticket)
	}

	if userID != "12345.abcd" {
		t.Errorf("userID = %q, want 12345.abcd", userID)
	}
}

// TestTicketStrategy_ResponseParsing verifies we correctly parse XML responses.
func TestTicketStrategy_ResponseParsing(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantTicket string
		wantUserID string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "valid response",
			response: `<?xml version="1.0" ?>
<qdbapi>
	<action>API_Authenticate</action>
	<errcode>0</errcode>
	<errtext>No error</errtext>
	<ticket>valid_ticket_xyz</ticket>
	<userid>67890.efgh</userid>
</qdbapi>`,
			wantTicket: "valid_ticket_xyz",
			wantUserID: "67890.efgh",
		},
		{
			name: "invalid credentials",
			response: `<?xml version="1.0" ?>
<qdbapi>
	<action>API_Authenticate</action>
	<errcode>20</errcode>
	<errtext>Unknown username/password</errtext>
	<errdetail>Sorry! Something's wrong with your sign-in info.</errdetail>
</qdbapi>`,
			wantErr:    true,
			wantErrMsg: "Something's wrong with your sign-in info",
		},
		{
			name: "error without detail uses errtext",
			response: `<?xml version="1.0" ?>
<qdbapi>
	<action>API_Authenticate</action>
	<errcode>22</errcode>
	<errtext>User not found</errtext>
</qdbapi>`,
			wantErr:    true,
			wantErrMsg: "User not found",
		},
		{
			name: "empty ticket",
			response: `<?xml version="1.0" ?>
<qdbapi>
	<action>API_Authenticate</action>
	<errcode>0</errcode>
	<errtext>No error</errtext>
	<ticket></ticket>
</qdbapi>`,
			wantErr:    true,
			wantErrMsg: "no ticket returned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			strategy := &TicketStrategy{
				username: "test@example.com",
				password: "testpass",
				realm:    "testrealm",
				hours:    12,
				client:   server.Client(),
			}

			ticket, userID, err := strategy.authenticateWithURL(context.Background(), server.URL+"/db/main")

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ticket != tt.wantTicket {
				t.Errorf("ticket = %q, want %q", ticket, tt.wantTicket)
			}
			if userID != tt.wantUserID {
				t.Errorf("userID = %q, want %q", userID, tt.wantUserID)
			}
		})
	}
}

// TestTicketStrategy_TicketCaching verifies the ticket is cached after first auth.
func TestTicketStrategy_TicketCaching(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?>
<qdbapi>
	<errcode>0</errcode>
	<ticket>cached_ticket</ticket>
	<userid>12345.test</userid>
</qdbapi>`))
	}))
	defer server.Close()

	strategy := &TicketStrategy{
		username: "test@example.com",
		password: "testpass",
		realm:    "testrealm",
		hours:    12,
		client:   server.Client(),
	}
	// Set testURL for testing
	strategy.testURL = server.URL + "/db/main"

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
		t.Errorf("server called %d times, want 1 (ticket should be cached)", callCount)
	}
}

// TestTicketStrategy_PasswordCleared verifies password is cleared after auth.
func TestTicketStrategy_PasswordCleared(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?>
<qdbapi>
	<errcode>0</errcode>
	<ticket>test_ticket</ticket>
	<userid>12345.test</userid>
</qdbapi>`))
	}))
	defer server.Close()

	strategy := &TicketStrategy{
		username: "test@example.com",
		password: "secret_password",
		realm:    "testrealm",
		hours:    12,
		client:   server.Client(),
	}
	strategy.testURL = server.URL + "/db/main"

	// Before auth, password should be set
	if strategy.password != "secret_password" {
		t.Error("password should be set before auth")
	}

	// Authenticate
	_, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	// After auth, password should be cleared
	if strategy.password != "" {
		t.Errorf("password should be cleared after auth, got %q", strategy.password)
	}

	// authenticated flag should be set
	if !strategy.authenticated {
		t.Error("authenticated flag should be true")
	}
}

// TestTicketStrategy_NoRefreshAfterExpiry verifies no auto-refresh after ticket expires.
func TestTicketStrategy_NoRefreshAfterExpiry(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?>
<qdbapi>
	<errcode>0</errcode>
	<ticket>initial_ticket</ticket>
	<userid>12345.test</userid>
</qdbapi>`))
	}))
	defer server.Close()

	strategy := &TicketStrategy{
		username: "test@example.com",
		password: "testpass",
		realm:    "testrealm",
		hours:    12,
		client:   server.Client(),
	}
	strategy.testURL = server.URL + "/db/main"

	// Initial auth
	_, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err != nil {
		t.Fatalf("initial GetToken failed: %v", err)
	}

	// Simulate 401 by calling HandleAuthError
	newToken, err := strategy.HandleAuthError(context.Background(), 401, "bqxyz123", 0, 3)

	// Should return empty - can't refresh without password
	if newToken != "" {
		t.Errorf("HandleAuthError returned token %q, want empty (no refresh)", newToken)
	}

	// Ticket should be cleared
	if strategy.ticket != "" {
		t.Errorf("ticket should be cleared, got %q", strategy.ticket)
	}

	// Subsequent GetToken should fail
	_, err = strategy.GetToken(context.Background(), "bqxyz123")
	if err == nil {
		t.Error("expected error after ticket expired, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention expired, got: %v", err)
	}

	// Server should only have been called once (initial auth)
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("server called %d times, want 1", callCount)
	}
}

// TestTicketStrategy_ApplyAuth verifies the ticket is applied correctly.
func TestTicketStrategy_ApplyAuth(t *testing.T) {
	strategy := NewTicketStrategy("user@example.com", "password", "testrealm")

	req := httptest.NewRequest("GET", "https://api.quickbase.com/v1/apps/bqxyz123", nil)
	strategy.ApplyAuth(req, "my_ticket_token")

	got := req.Header.Get("Authorization")
	want := "QB-TICKET my_ticket_token"

	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

// TestTicketStrategy_XMLEscaping verifies special characters are escaped.
func TestTicketStrategy_XMLEscaping(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?>
<qdbapi>
	<errcode>0</errcode>
	<ticket>test_ticket</ticket>
	<userid>12345.test</userid>
</qdbapi>`))
	}))
	defer server.Close()

	strategy := &TicketStrategy{
		username: "user@example.com",
		password: "pass<>&\"'word",
		realm:    "testrealm",
		hours:    12,
		client:   server.Client(),
	}

	_, _, err := strategy.authenticateWithURL(context.Background(), server.URL+"/db/main")
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}

	// Verify special characters are escaped
	if strings.Contains(capturedBody, "pass<>&") {
		t.Errorf("XML special characters not escaped in body: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "&lt;") || !strings.Contains(capturedBody, "&gt;") || !strings.Contains(capturedBody, "&amp;") {
		t.Errorf("expected escaped characters in body: %s", capturedBody)
	}
}

// TestNewTicketStrategy_Options verifies option functions work.
func TestNewTicketStrategy_Options(t *testing.T) {
	customClient := &http.Client{}

	strategy := NewTicketStrategy("user@test.com", "pass123", "myrealm",
		WithTicketHours(48),
		WithTicketHTTPClient(customClient),
	)

	if strategy.client != customClient {
		t.Error("custom HTTP client was not applied")
	}

	if strategy.hours != 48 {
		t.Errorf("hours = %d, want 48", strategy.hours)
	}

	if strategy.realm != "myrealm" {
		t.Errorf("realm = %q, want myrealm", strategy.realm)
	}

	if strategy.username != "user@test.com" {
		t.Errorf("username = %q, want user@test.com", strategy.username)
	}
}

// TestNewTicketStrategy_HoursBounds verifies hours are bounded correctly.
func TestNewTicketStrategy_HoursBounds(t *testing.T) {
	// Test max bound
	strategy := NewTicketStrategy("user", "pass", "realm", WithTicketHours(10000))
	if strategy.hours != 4380 {
		t.Errorf("hours = %d, want 4380 (max)", strategy.hours)
	}

	// Test min bound
	strategy = NewTicketStrategy("user", "pass", "realm", WithTicketHours(0))
	if strategy.hours != 1 {
		t.Errorf("hours = %d, want 1 (min)", strategy.hours)
	}

	// Test negative
	strategy = NewTicketStrategy("user", "pass", "realm", WithTicketHours(-5))
	if strategy.hours != 1 {
		t.Errorf("hours = %d, want 1 (min)", strategy.hours)
	}
}

// TestTicketStrategy_UserID verifies UserID is available after auth.
func TestTicketStrategy_UserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?>
<qdbapi>
	<errcode>0</errcode>
	<ticket>test_ticket</ticket>
	<userid>99999.wxyz</userid>
</qdbapi>`))
	}))
	defer server.Close()

	strategy := &TicketStrategy{
		username: "test@example.com",
		password: "testpass",
		realm:    "testrealm",
		hours:    12,
		client:   server.Client(),
	}
	strategy.testURL = server.URL + "/db/main"

	// Before auth, UserID should be empty
	if strategy.UserID() != "" {
		t.Errorf("UserID before auth = %q, want empty", strategy.UserID())
	}

	// Authenticate
	_, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}

	// After auth, UserID should be set
	if strategy.UserID() != "99999.wxyz" {
		t.Errorf("UserID after auth = %q, want 99999.wxyz", strategy.UserID())
	}
}

// TestTicketStrategy_SignOut verifies SignOut clears credentials and prevents further use.
func TestTicketStrategy_SignOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?>
<qdbapi>
	<errcode>0</errcode>
	<ticket>test_ticket</ticket>
	<userid>12345.test</userid>
</qdbapi>`))
	}))
	defer server.Close()

	strategy := &TicketStrategy{
		username: "test@example.com",
		password: "testpass",
		realm:    "testrealm",
		hours:    12,
		client:   server.Client(),
	}
	strategy.testURL = server.URL + "/db/main"

	// Authenticate first
	ticket, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if ticket != "test_ticket" {
		t.Errorf("ticket = %q, want test_ticket", ticket)
	}

	// Sign out
	strategy.SignOut()

	// Verify ticket is cleared
	if strategy.ticket != "" {
		t.Errorf("ticket should be empty after SignOut, got %q", strategy.ticket)
	}

	// Verify password is cleared
	if strategy.password != "" {
		t.Errorf("password should be empty after SignOut, got %q", strategy.password)
	}

	// Subsequent GetToken should fail (can't re-authenticate without password)
	_, err = strategy.GetToken(context.Background(), "bqxyz123")
	if err == nil {
		t.Error("expected error after SignOut, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention expired, got: %v", err)
	}
}

// TestTicketStrategy_SignOut_BeforeAuth verifies SignOut works even before authentication.
func TestTicketStrategy_SignOut_BeforeAuth(t *testing.T) {
	strategy := NewTicketStrategy("user@example.com", "password", "testrealm")

	// Sign out before any auth
	strategy.SignOut()

	// Should not be able to authenticate (password is cleared, marked as authenticated)
	_, err := strategy.GetToken(context.Background(), "bqxyz123")
	if err == nil {
		t.Error("expected error after SignOut before auth, got nil")
	}
}
