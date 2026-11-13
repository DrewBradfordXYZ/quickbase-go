package quickbase_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
)

// Schema aliases are resolved in the HTTP layer, so every method takes them:
// bodies (from, to, where, record data, fieldsToReturn, mergeFieldId) and a
// tableId query parameter. A table ID the schema doesn't name goes as given.
func TestAliasesResolvedForEveryRequest(t *testing.T) {
	var got []sent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		got = append(got, sent{r.URL.Path + "?" + r.URL.RawQuery, body})
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/fields") {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		_, _ = io.WriteString(w, `{"metadata":{},"data":[]}`)
	}))
	defer srv.Close()

	schema := quickbase.NewSchema().Table("projects", "bqprojects").Field("name", 6).Field("status", 7).Field("code", 9).Build()
	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL), quickbase.WithSchema(schema))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := qb.DeleteRecords(ctx, quickbase.DeleteRecordsBody{From: "projects", Where: "{'status'.EX.'Done'}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := qb.Upsert(ctx, quickbase.UpsertBody{To: "projects", Data: []quickbase.Record{quickbase.Row("name", "Alpha", 6, "kept")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := qb.GetFields(ctx, "projects", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := qb.RunQuery(ctx, quickbase.RunQueryBody{From: "bqother1"}); err != nil {
		t.Fatalf("a table the schema doesn't name: %v", err)
	}

	if got[0].body["from"] != "bqprojects" || got[0].body["where"] != "{7.EX.'Done'}" {
		t.Errorf("DeleteRecords sent %v", got[0].body)
	}
	if got[1].body["to"] != "bqprojects" {
		t.Errorf("Upsert sent to=%v", got[1].body["to"])
	}
	if rec := got[1].body["data"].([]any)[0].(map[string]any); rec["6"] == nil {
		t.Errorf("Upsert record %v, want keyed by field ID", rec)
	}
	if !strings.Contains(got[2].path, "tableId=bqprojects") {
		t.Errorf("GetFields sent %s", got[2].path)
	}
	if got[3].body["from"] != "bqother1" {
		t.Errorf("RunQuery sent from=%v", got[3].body["from"])
	}

	// A misspelled alias still errors, before anything is sent
	before := len(got)
	if _, err := qb.RunQuery(ctx, quickbase.RunQueryBody{From: "projcts"}); err == nil || !strings.Contains(err.Error(), "projects") {
		t.Errorf("misspelled table: err = %v, want a suggestion", err)
	}
	if len(got) != before {
		t.Error("a misspelled table was sent")
	}
}

// transferUserToken's from and to are user IDs: with a schema they were
// resolved as table aliases, and the call failed. A table alias in a path
// (/tables/{tableId}) went to QuickBase unresolved.
func TestAliasesOnlyWhereTablesAre(t *testing.T) {
	var got []sent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		got = append(got, sent{r.URL.Path + "?" + r.URL.RawQuery, body})
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	schema := quickbase.NewSchema().Table("projects", "bqprojects").Field("name", 6).Build()
	qb, _ := quickbase.New("r", quickbase.WithUserToken("t"), quickbase.WithBaseURL(srv.URL), quickbase.WithSchema(schema))
	ctx := context.Background()

	if _, err := qb.TransferUserToken(ctx, quickbase.TransferUserTokenJSONRequestBody{From: "56789.abcd", To: "12345.wxyz", Id: 1}); err != nil {
		t.Fatalf("TransferUserToken with a schema: %v", err)
	}
	if got[0].body["from"] != "56789.abcd" {
		t.Errorf("user ID rewritten: %v", got[0].body)
	}
	if _, err := qb.GetTable(ctx, "projects", "bqapp"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got[1].path, "/tables/bqprojects?") {
		t.Errorf("GetTable sent %s", got[1].path)
	}
	// An older table ID (no "b" prefix) the schema doesn't name goes as given
	if _, err := qb.GetTable(ctx, "9kaw8phg", "bqapp"); err != nil {
		t.Errorf("an unlisted table ID: %v", err)
	}
}

type sent struct {
	path string
	body map[string]any
}
