package auth

// NOTE: This file uses tickets obtained elsewhere (e.g., from a browser client).
// Related to the XML API ticket flow but doesn't call API_Authenticate itself.
// Search for "XML-API-TICKET" to find related code.

import (
	"context"
	"net/http"
)

// ExistingTicketStrategy authenticates using a pre-existing ticket.
//
// Unlike TicketStrategy which obtains a ticket via API_Authenticate,
// this strategy uses a ticket that was already obtained elsewhere
// (e.g., by a browser client calling API_Authenticate directly).
//
// This is useful for Code Page scenarios where:
//  1. Browser calls API_Authenticate with user credentials
//  2. Browser sends ticket to Go server
//  3. Go server uses the ticket for REST API calls
//
// The server never sees the user's credentials - only the ticket.
type ExistingTicketStrategy struct {
	ticket string
}

// NewExistingTicketStrategy creates a strategy using a pre-existing ticket.
//
// Example:
//
//	strategy := auth.NewExistingTicketStrategy(ticketFromBrowser)
func NewExistingTicketStrategy(ticket string) *ExistingTicketStrategy {
	return &ExistingTicketStrategy{ticket: ticket}
}

// GetToken returns the existing ticket.
func (s *ExistingTicketStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	return s.ticket, nil
}

// ApplyAuth applies the ticket to the Authorization header.
func (s *ExistingTicketStrategy) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-TICKET "+token)
}

// HandleAuthError handles 401 errors. Since we don't have credentials,
// we cannot re-authenticate - the ticket has expired.
func (s *ExistingTicketStrategy) HandleAuthError(ctx context.Context, statusCode int, dbid string, attempt int, maxAttempts int) (string, error) {
	// Cannot refresh - we don't have credentials
	return "", nil
}
