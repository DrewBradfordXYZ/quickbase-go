package quickbase_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v2"
)

// WithMaxRetries(0) made no attempt: the request returned neither a response
// nor an error, and callers dereferenced nil. It now means one attempt.
func TestWithMaxRetriesZeroMakesOneAttempt(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"message":"Service Unavailable"}`)
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL), quickbase.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := qb.ExportSolution("abc").Run(context.Background()); err == nil {
		t.Error("a 503 should be an error")
	}
	if calls != 1 {
		t.Errorf("%d requests, want 1", calls)
	}
}
