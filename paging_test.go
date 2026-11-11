package quickbase_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v2"
)

// A fake runQuery that serves 3 records, 2 per page, and records each body.
func pagedServer(t *testing.T) (*httptest.Server, func() []map[string]any) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		skip := 0
		if opts, ok := body["options"].(map[string]any); ok {
			if s, ok := opts["skip"].(float64); ok {
				skip = int(s)
			}
		}
		all := []map[string]any{
			{"3": map[string]any{"value": 1}},
			{"3": map[string]any{"value": 2}},
			{"3": map[string]any{"value": 3}},
		}
		end := min(skip+2, len(all))
		page := all[skip:end]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":     page,
			"fields":   []any{},
			"metadata": map[string]any{"totalRecords": len(all), "numRecords": len(page), "numFields": 1, "skip": skip},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, func() []map[string]any { mu.Lock(); defer mu.Unlock(); return bodies }
}

// RunQueryIterator streams every page.
func TestRunQueryIteratorStreamsPages(t *testing.T) {
	srv, bodies := pagedServer(t)
	qb, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))

	var got []any
	for rec, err := range qb.RunQueryIterator(context.Background(), quickbase.RunQueryBody{From: "bqtable1"}) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, quickbase.UnwrapRecords([]quickbase.Record{rec})[0]["3"])
	}
	if len(got) != 3 || got[2] != float64(3) {
		t.Errorf("records = %v, want 1, 2, 3", got)
	}
	if n := len(bodies()); n != 2 {
		t.Errorf("%d requests, want 2 pages", n)
	}
}

// RunQueryAll resolved no schema aliases (RunQuery did), and it set the
// page's skip on the caller's Options.
func TestRunQueryAllResolvesAliasesAndKeepsCallerOptions(t *testing.T) {
	srv, bodies := pagedServer(t)
	schema := quickbase.NewSchema().Table("projects", "bqtable1").Field("id", 3).Field("name", 6).Build()
	qb, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL), quickbase.WithSchema(schema))

	body := quickbase.RunQueryBody{
		From:    "projects",
		Select:  quickbase.Fields(schema, "projects", "id", "name"),
		Where:   quickbase.Where("{'name'.EX.'Alpha'}"),
		Options: quickbase.Options(2, 0),
	}
	records, err := qb.RunQueryAll(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Errorf("%d records, want 3", len(records))
	}
	sent := bodies()[0]
	if sent["from"] != "bqtable1" || sent["where"] != "{6.EX.'Alpha'}" {
		t.Errorf("sent from=%v where=%v, want bqtable1 and {6.EX.'Alpha'}", sent["from"], sent["where"])
	}
	if body.Options.Skip == nil || *body.Options.Skip != 0 {
		t.Errorf("caller's Options.Skip = %v, want 0 (unchanged)", body.Options.Skip)
	}
}

// TransformResponses did nothing (no code read it). It now keys the query
// builder's records by alias, and is off by default.
func TestQueryBuilderKeysByAliasWhenAsked(t *testing.T) {
	srv, _ := pagedServer(t)
	schema := quickbase.NewSchema().Table("projects", "bqtable1").Field("id", 3).Build()

	byID, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL), quickbase.WithSchema(schema))
	recs, err := byID.Query("projects").Select("id").Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := recs[0]["3"]; !ok {
		t.Errorf("default: record %v, want keyed by field ID", recs[0])
	}

	byAlias, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL),
		quickbase.WithSchemaOptions(schema, quickbase.SchemaOptions{TransformResponses: true}))
	recs, err = byAlias.Query("projects").Select("id").RunN(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if recs[0]["id"] != float64(1) {
		t.Errorf("TransformResponses: record %v, want keyed by alias", recs[0])
	}
}
