package quickbase_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// Cancelling a request's context also cancels a retry's backoff wait: it
// slept with time.Sleep, so a 10s backoff held the call for 10s after its
// context was done.
func TestRetryWaitStopsWhenContextEnds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"message":"Service Unavailable"}`)
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithBaseURL(srv.URL),
		quickbase.WithRetryDelay(10*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = qb.GetApp("bqxyz123").Run(ctx)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("returned after %s; the backoff ignored the context", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}
