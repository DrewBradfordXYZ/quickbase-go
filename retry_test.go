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

// A caller cancelling its context got a TimeoutError naming the client's
// timeout, as if the request had run out of time. It now gets the context's
// error; TimeoutError is kept for WithTimeout running out.
func TestCancelIsNotATimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err = qb.ExportSolution("abc").Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	var timeout *quickbase.TimeoutError
	if errors.As(err, &timeout) {
		t.Errorf("a cancelled request reported a timeout: %v", err)
	}
}

func TestClientTimeoutIsATimeoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithBaseURL(srv.URL),
		quickbase.WithTimeout(50*time.Millisecond),
		quickbase.WithMaxRetries(1),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = qb.ExportSolution("abc").Run(context.Background())
	var timeout *quickbase.TimeoutError
	if !errors.As(err, &timeout) {
		t.Errorf("err = %v (%T), want TimeoutError", err, err)
	}
}

// A write that got a 5xx or timed out may have been applied: an upsert sent
// again could add its records twice. Writes retry only after a 429; reads
// (including runQuery, a POST) still retry after a 5xx.
func TestWritesRetryOnlyAfter429(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		write  bool
		want   int
	}{
		{"upsert 503", http.StatusServiceUnavailable, true, 1},
		{"upsert 429", http.StatusTooManyRequests, true, 3},
		{"query 503", http.StatusServiceUnavailable, false, 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(c.status)
				_, _ = io.WriteString(w, `{"message":"busy"}`)
			}))
			defer srv.Close()

			qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL),
				quickbase.WithRetryDelay(time.Millisecond), quickbase.WithMaxRetryDelay(time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if c.write {
				_, err = qb.Upsert("bqtable").Data(quickbase.Row(6, "a")).Run(ctx)
			} else {
				_, err = qb.RunQuery(ctx, quickbase.RunQueryBody{From: "bqtable"})
			}
			if err == nil {
				t.Error("want an error")
			}
			if calls != c.want {
				t.Errorf("%d requests, want %d", calls, c.want)
			}
		})
	}
}
