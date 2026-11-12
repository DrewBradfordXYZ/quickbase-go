package quickbase_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v2"
)

// Read-only mode let two solution GETs through that write: updating a
// solution from a record changes apps, and exporting one to a record writes
// that record. Neither request should reach the server.
func TestReadOnlyBlocksSolutionRecordWrites(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL), quickbase.WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var readOnly *quickbase.ReadOnlyError

	if _, err := qb.API().UpdateSolutionToRecord(ctx, "sol1", nil); !errors.As(err, &readOnly) {
		t.Errorf("updateSolutionToRecord: err = %v, want ReadOnlyError", err)
	}
	if _, err := qb.API().ExportSolutionToRecord(ctx, "sol1", nil); !errors.As(err, &readOnly) {
		t.Errorf("exportSolutionToRecord: err = %v, want ReadOnlyError", err)
	}
	if calls != 0 {
		t.Errorf("%d requests reached the server, want 0", calls)
	}
}

// A time.Time in upsert data went as a timestamp with its offset, which a
// Date field rejects, leaving the field blank on a record QuickBase saves.
// It's refused before anything is sent.
func TestUpsertRefusesPlainTime(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = qb.Upsert("bqtable").Data(map[string]any{"7": map[string]any{"value": time.Now()}}).Run(context.Background())
	var plain *quickbase.PlainTimeError
	if !errors.As(err, &plain) {
		t.Errorf("err = %v, want PlainTimeError", err)
	}
	if calls != 0 {
		t.Errorf("%d requests reached the server, want 0", calls)
	}
}
