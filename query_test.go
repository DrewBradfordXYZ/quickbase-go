package quickbase_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
)

// The body a query sends for each sort order and where clause.
func TestQueryBodyWhereAndSortBy(t *testing.T) {
	tests := []struct {
		name string
		body quickbase.RunQueryBody
		want string
	}{
		{"unset", quickbase.RunQueryBody{From: "bqt"}, `{"from":"bqt"}`},
		{"where", quickbase.RunQueryBody{From: "bqt", Where: "{7.EX.'Active'}"}, `{"from":"bqt","where":"{7.EX.'Active'}"}`},
		{"record IDs", quickbase.RunQueryBody{From: "bqt", Where: quickbase.RecordIDs(12, 15)}, `{"from":"bqt","where":"{3.EX.12}OR{3.EX.15}"}`},
		{"sort", quickbase.RunQueryBody{From: "bqt", SortBy: quickbase.SortBy{quickbase.Asc(6), quickbase.Desc(3)}},
			`{"from":"bqt","sortBy":[{"fieldId":6,"order":"ASC"},{"fieldId":3,"order":"DESC"}]}`},
		{"no sort", quickbase.RunQueryBody{From: "bqt", SortBy: quickbase.NoSort}, `{"from":"bqt","sortBy":false}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

// A sort order read back: fields, or false as NoSort.
func TestSortByUnmarshal(t *testing.T) {
	var s quickbase.SortBy
	if err := json.Unmarshal([]byte(`false`), &s); err != nil || s == nil || len(s) != 0 {
		t.Errorf("false: got %#v, %v; want NoSort", s, err)
	}
	if err := json.Unmarshal([]byte(`[{"fieldId":6,"order":"DESC"}]`), &s); err != nil || len(s) != 1 || s[0] != quickbase.Desc(6) {
		t.Errorf("fields: got %#v, %v", s, err)
	}
}

// Required query parameters are arguments; optional ones come from opts,
// which can be nil.
func TestRequiredArgumentsAndOptions(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	qb, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))
	ctx := context.Background()

	if _, err := qb.GetFields(ctx, "bqtable1", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := qb.GetFields(ctx, "bqtable1", &quickbase.GetFieldsOptions{IncludeFieldPerms: quickbase.Ptr(true)}); err != nil {
		t.Fatal(err)
	}
	want := []string{"tableId=bqtable1", "tableId=bqtable1&includeFieldPerms=true"}
	if len(queries) != 2 || queries[0] != want[0] || queries[1] != want[1] {
		t.Errorf("got %q, want %q", queries, want)
	}
}

// A date parameter goes in the URL as YYYY-MM-DD.
func TestDateParameter(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	qb, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))

	day := quickbase.Date{Year: 2026, Month: 11, Day: 2}
	_, _ = qb.PlatformAnalyticReads(context.Background(), day)
	if query != "day=2026-11-02" {
		t.Errorf("got %q, want day=2026-11-02", query)
	}
}
