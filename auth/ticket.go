package auth

// NOTE: Ticket authentication uses the legacy QuickBase XML API (API_Authenticate).
// If QuickBase discontinues the XML API in the future, this file and related code
// can be safely removed.
//
// To find all related code, search for the marker: grep -r "XML-API-TICKET"
//
// Files to remove/update:
//   - auth/ticket.go (this file)
//   - auth/ticket_test.go
//   - quickbase.go (search for XML-API-TICKET markers)
//   - tests/integration/auth_test.go: TestTicketAuth()
//   - README.md: "Ticket Auth (Username/Password)" section
//   - .env.example: QB_USERNAME, QB_PASSWORD

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sync"
)

// TicketStrategy authenticates using API_Authenticate (XML API).
//
// This strategy exchanges username/password credentials for an authentication
// ticket, which is then used with REST API calls. Unlike user tokens, tickets
// properly attribute record changes (createdBy/modifiedBy) to the authenticated user.
//
// The ticket is obtained lazily on the first API call. The password is discarded
// after authentication and not stored. When the ticket expires (401 error),
// an AuthenticationError is returned and a new client must be created.
//
// Tickets are valid for 12 hours by default, configurable up to ~6 months (4380 hours).
type TicketStrategy struct {
	username string
	password string // Cleared after first successful auth
	hours    int
	realm    string
	client   *http.Client

	mu            sync.RWMutex
	ticket        string
	userID        string
	pending       chan struct{}
	authenticated bool // True after first successful auth (password cleared)

	// testURL is used for testing to override the authentication URL
	testURL string
}

// TicketOption configures a TicketStrategy.
type TicketOption func(*TicketStrategy)

// WithTicketHours sets the ticket validity duration in hours.
// Default is 12 hours. Maximum is 4380 hours (~6 months).
func WithTicketHours(hours int) TicketOption {
	return func(s *TicketStrategy) {
		if hours > 4380 {
			hours = 4380
		}
		if hours < 1 {
			hours = 1
		}
		s.hours = hours
	}
}

// WithTicketHTTPClient sets a custom HTTP client for authentication requests.
func WithTicketHTTPClient(client *http.Client) TicketOption {
	return func(s *TicketStrategy) {
		s.client = client
	}
}

// NewTicketStrategy creates a new ticket authentication strategy.
//
// The username is typically the user's email address registered with QuickBase.
// The password is the user's QuickBase password.
//
// Authentication is performed lazily on the first API call. After successful
// authentication, the password is discarded from memory.
//
// Example:
//
//	strategy := auth.NewTicketStrategy("user@example.com", "password", "myrealm")
//
// With custom ticket validity:
//
//	strategy := auth.NewTicketStrategy("user@example.com", "password", "myrealm",
//	    auth.WithTicketHours(24*7), // 1 week
//	)
func NewTicketStrategy(username, password, realm string, opts ...TicketOption) *TicketStrategy {
	s := &TicketStrategy{
		username: username,
		password: password,
		realm:    realm,
		hours:    12, // Default: 12 hours
		client:   http.DefaultClient,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetToken returns the authentication ticket, calling API_Authenticate if needed.
func (s *TicketStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	s.mu.RLock()
	if s.ticket != "" {
		ticket := s.ticket
		s.mu.RUnlock()
		return ticket, nil
	}
	s.mu.RUnlock()

	// Check if there's a pending authentication
	s.mu.Lock()
	if s.pending != nil {
		pending := s.pending
		s.mu.Unlock()
		<-pending
		return s.GetToken(ctx, dbid)
	}

	// Check if we already authenticated (password is gone)
	if s.authenticated {
		s.mu.Unlock()
		return "", fmt.Errorf("ticket expired; create a new client with fresh credentials")
	}

	s.pending = make(chan struct{})
	s.mu.Unlock()

	ticket, userID, err := s.authenticate(ctx)

	s.mu.Lock()
	if err == nil {
		s.ticket = ticket
		s.userID = userID
		s.authenticated = true
	}
	s.password = "" // Clear password from memory regardless of success/failure
	close(s.pending)
	s.pending = nil
	s.mu.Unlock()

	return ticket, err
}

// authenticateResponse is the XML response from API_Authenticate.
type authenticateResponse struct {
	XMLName   xml.Name `xml:"qdbapi"`
	Action    string   `xml:"action"`
	ErrCode   int      `xml:"errcode"`
	ErrText   string   `xml:"errtext"`
	ErrDetail string   `xml:"errdetail"`
	Ticket    string   `xml:"ticket"`
	UserID    string   `xml:"userid"`
}

// authenticate calls the XML API_Authenticate endpoint.
func (s *TicketStrategy) authenticate(ctx context.Context) (string, string, error) {
	url := s.testURL
	if url == "" {
		url = fmt.Sprintf("https://%s.quickbase.com/db/main", s.realm)
	}
	return s.authenticateWithURL(ctx, url)
}

// authenticateWithURL calls API_Authenticate at the specified URL.
// This is separated from authenticate() to enable testing.
func (s *TicketStrategy) authenticateWithURL(ctx context.Context, url string) (string, string, error) {
	// Build XML request body
	reqBody := fmt.Sprintf(`<qdbapi>
    <username>%s</username>
    <password>%s</password>
    <hours>%d</hours>
</qdbapi>`, xmlEscape(s.username), xmlEscape(s.password), s.hours)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("QUICKBASE-ACTION", "API_Authenticate")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("authenticating: %w", err)
	}
	defer resp.Body.Close()

	var authResp authenticateResponse
	if err := xml.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", "", fmt.Errorf("decoding response: %w", err)
	}

	if authResp.ErrCode != 0 {
		errMsg := authResp.ErrText
		if authResp.ErrDetail != "" {
			errMsg = authResp.ErrDetail
		}
		return "", "", fmt.Errorf("authentication failed: %s (code: %d)", errMsg, authResp.ErrCode)
	}

	if authResp.Ticket == "" {
		return "", "", fmt.Errorf("no ticket returned from API_Authenticate")
	}

	return authResp.Ticket, authResp.UserID, nil
}

// ApplyAuth applies the ticket to the Authorization header.
func (s *TicketStrategy) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-TICKET "+token)
}

// HandleAuthError handles 401 errors. Since the password is discarded after
// initial authentication, this returns an empty string to signal that
// re-authentication is not possible and the user must create a new client.
func (s *TicketStrategy) HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error) {
	if statusCode != http.StatusUnauthorized {
		return "", nil
	}

	// Clear the expired ticket
	s.mu.Lock()
	s.ticket = ""
	s.mu.Unlock()

	// Cannot re-authenticate - password was discarded
	return "", nil
}

// UserID returns the authenticated user's ID (available after first API call).
func (s *TicketStrategy) UserID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userID
}

// SignOut clears the stored ticket from memory, preventing further API calls.
//
// This does NOT invalidate the ticket on QuickBase's servers - tickets remain
// valid until they expire. However, this client will no longer be able to make
// API calls after SignOut is called.
//
// To make API calls again, create a new client with fresh credentials.
//
// Use this when:
//   - A user logs out of your application
//   - You want to force re-authentication
//   - You're done with a session and want to clear credentials from memory
//
// Example:
//
//	// User clicks "logout"
//	client.SignOut()
//	// Next API call will fail with "signed out" error
func (s *TicketStrategy) SignOut() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ticket = ""
	s.password = ""
	s.authenticated = true // Prevents re-authentication (password is gone)
}

// xmlEscape escapes special XML characters in a string.
// If escaping fails (invalid characters), returns empty string for safety.
func xmlEscape(s string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return ""
	}
	return buf.String()
}
