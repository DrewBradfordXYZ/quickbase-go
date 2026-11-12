package quickbase_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
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
	_, err = qb.Upsert(context.Background(), quickbase.UpsertBody{To: "bqtable", Data: []quickbase.Record{{"7": {Value: time.Now()}}}})
	var plain *quickbase.PlainTimeError
	if !errors.As(err, &plain) {
		t.Errorf("err = %v, want PlainTimeError", err)
	}
	if calls != 0 {
		t.Errorf("%d requests reached the server, want 0", calls)
	}
}

// QuickBase answers 207 when it rejected some values and saved the rest.
// Upsert returned "Multi-Status" as an error and no result, losing the
// saved records' IDs.
func TestUpsert207KeepsTheResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `{"data":[],"metadata":{"createdRecordIds":[1,2],"lineErrors":{"2":["Incompatible value for field with ID \"7\"."]},"totalNumberOfRecordsProcessed":2,"unchangedRecordIds":[],"updatedRecordIds":[]}}`)
	}))
	defer srv.Close()
	ctx := context.Background()

	qb, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))
	result, err := qb.Upsert(ctx, quickbase.UpsertBody{To: "bqtable", Data: []quickbase.Record{quickbase.Row(6, "a"), quickbase.Row(6, "b")}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := result.Metadata.CreatedRecordIds; len(got) != 2 {
		t.Errorf("CreatedRecordIds = %v", got)
	}
	if got := result.Metadata.LineErrors; len(got["2"]) != 1 {
		t.Errorf("LineErrors = %v", got)
	}

	strict, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL), quickbase.WithLineErrorsAsErrors())
	result, err = strict.Upsert(ctx, quickbase.UpsertBody{To: "bqtable", Data: []quickbase.Record{quickbase.Row(6, "a"), quickbase.Row(6, "b")}})
	var lineErrs *quickbase.LineErrorsError
	if !errors.As(err, &lineErrs) || !strings.Contains(err.Error(), "record 2: Incompatible value") {
		t.Errorf("err = %v, want LineErrorsError naming record 2", err)
	}
	if result == nil || len(result.Metadata.CreatedRecordIds) != 2 {
		t.Error("the result should come back with the error")
	}
}

// The endpoint lists name paths under /v1. Under another base URL (a proxy,
// or a test server) the paths didn't match: read-only mode let GETs that
// write through, and a query, a POST, counted as a write.
func TestReadOnlyUnderACustomBaseURL(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"fields":[],"metadata":{"totalRecords":0,"numRecords":0,"numFields":0,"skip":0}}`)
	}))
	defer srv.Close()

	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL+"/proxy/qb"), quickbase.WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var readOnly *quickbase.ReadOnlyError
	if _, err := qb.API().UpdateSolutionToRecord(ctx, "sol1", nil); !errors.As(err, &readOnly) {
		t.Errorf("updateSolutionToRecord: err = %v, want ReadOnlyError", err)
	}
	if _, err := qb.RunQuery(ctx, quickbase.RunQueryBody{From: "bqtable"}); err != nil {
		t.Errorf("runQuery: %v", err)
	}
	if calls != 1 {
		t.Errorf("%d requests, want 1 (the query)", calls)
	}
}
