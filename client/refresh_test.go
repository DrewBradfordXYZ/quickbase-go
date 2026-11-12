package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// refreshingStrategy always has a new token after a 401, as SSO and ticket
// auth can.
type refreshingStrategy struct{ refreshes int }

func (s *refreshingStrategy) GetToken(ctx context.Context, dbid string) (string, error) {
	return "token", nil
}
func (s *refreshingStrategy) ApplyAuth(req *http.Request, token string) {}
func (s *refreshingStrategy) HandleAuthError(ctx context.Context, status int, dbid string, attempt, max int) (string, error) {
	s.refreshes++
	return "new-token", nil
}

// A token refreshed after the last attempt's 401 ended the loop with neither
// a response nor an error, and the caller dereferenced nil. The last 401 is
// now the result.
func TestRefreshOnLastAttemptReturnsTheResponse(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"Invalid token"}`)
	}))
	defer srv.Close()

	strategy := &refreshingStrategy{}
	c, err := New("testrealm", strategy, WithBaseURL(srv.URL), WithMaxRetries(2))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.API().GetAppWithResponse(context.Background(), "bqapp")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if resp.StatusCode() != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode())
	}
	if calls != 2 || strategy.refreshes != 1 {
		t.Errorf("%d requests and %d refreshes, want 2 and 1", calls, strategy.refreshes)
	}
}
