package quickbase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
)

type project struct {
	ID     int            `qb:"3"`
	Name   string         `qb:"6"`
	Due    quickbase.Date `qb:"7"`
	Budget *float64       `qb:"9"`
}

type projectChanges struct {
	ID     *int            `qb:"3"`
	Name   *string         `qb:"6"`
	Due    *quickbase.Date `qb:"7"`
	Budget *float64        `qb:"9"`
}

var projects = quickbase.NewTable[project, projectChanges]("bqprojects")

// A fake table: runQuery serves rows (2 per page) and records each body.
func tableServer(t *testing.T, rows []map[string]any) (*quickbase.Client, func() []map[string]any) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/records/query"):
			skip := 0
			if opts, ok := body["options"].(map[string]any); ok {
				skip = int(opts["skip"].(float64))
			}
			page := rows[min(skip, len(rows)):min(skip+2, len(rows))]
			_ = json.NewEncoder(w).Encode(map[string]any{"data": page, "fields": []any{},
				"metadata": map[string]any{"totalRecords": len(rows), "numRecords": len(page), "numFields": 4, "skip": skip}})
		case r.Method == http.MethodDelete:
			_, _ = io.WriteString(w, `{"numberDeleted":2}`)
		default:
			_, _ = io.WriteString(w, `{"data":[],"metadata":{"createdRecordIds":[5]}}`)
		}
	}))
	t.Cleanup(srv.Close)
	qb, err := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	return qb, func() []map[string]any { mu.Lock(); defer mu.Unlock(); return bodies }
}

func row(id int, name, due string, budget any) map[string]any {
	return map[string]any{"3": map[string]any{"value": id}, "6": map[string]any{"value": name},
		"7": map[string]any{"value": due}, "9": map[string]any{"value": budget}}
}

func TestTableQuerySelectsTheStructsFieldsAndDecodes(t *testing.T) {
	qb, sent := tableServer(t, []map[string]any{row(1, "Alpha", "2024-01-15", 10), row(2, "Beta", "", nil), row(3, "Gamma", "", 30)})

	got, err := projects.Query(context.Background(), qb, quickbase.TableQuery{Where: "{6.EX.'x'}"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Name != "Alpha" || got[0].Due.String() != "2024-01-15" || *got[0].Budget != 10 || got[1].Budget != nil {
		t.Errorf("Query = %+v", got)
	}
	first := sent()[0]
	if sel, _ := json.Marshal(first["select"]); string(sel) != "[3,6,7,9]" || first["where"] != "{6.EX.'x'}" {
		t.Errorf("sent select=%s where=%v", sel, first["where"])
	}
	if len(sent()) != 2 {
		t.Errorf("%d pages fetched, want 2", len(sent()))
	}

	limited, _ := projects.Query(context.Background(), qb, quickbase.TableQuery{Limit: 1})
	if len(limited) != 1 {
		t.Errorf("Limit 1 returned %d", len(limited))
	}
}

func TestTableGet(t *testing.T) {
	qb, sent := tableServer(t, []map[string]any{row(7, "Alpha", "", nil)})
	p, err := projects.Get(context.Background(), qb, 7)
	if err != nil || p.ID != 7 || sent()[0]["where"] != "{3.EX.7}" {
		t.Errorf("Get = %+v, %v (where %v)", p, err, sent()[0]["where"])
	}

	empty, _ := tableServer(t, nil)
	var notFound *quickbase.NotFoundError
	if _, err := projects.Get(context.Background(), empty, 7); !errors.As(err, &notFound) {
		t.Errorf("missing record: err = %v, want NotFoundError", err)
	}
}

// A nil field in a change isn't sent, so only the fields set change.
func TestTableUpsertSendsOnlySetFields(t *testing.T) {
	qb, sent := tableServer(t, nil)
	result, err := projects.Upsert(context.Background(), qb,
		projectChanges{Name: quickbase.Ptr("Alpha"), Due: &quickbase.Date{Year: 2024, Month: 2, Day: 29}},
		projectChanges{ID: quickbase.Ptr(4), Budget: quickbase.Ptr(12.5)},
	)
	if err != nil || len(result.Metadata.CreatedRecordIds) != 1 {
		t.Fatalf("Upsert = %+v, %v", result, err)
	}
	data, _ := json.Marshal(sent()[0]["data"])
	if string(data) != `[{"6":{"value":"Alpha"},"7":{"value":"2024-02-29"}},{"3":{"value":4},"9":{"value":12.5}}]` {
		t.Errorf("sent %s", data)
	}
}

func TestTableDeleteRefusesAnEmptyQuery(t *testing.T) {
	qb, sent := tableServer(t, nil)
	if _, err := projects.Delete(context.Background(), qb, "  "); err == nil {
		t.Error("an empty query would delete every record")
	}
	if n, err := projects.Delete(context.Background(), qb, "{6.EX.'old'}"); err != nil || n != 2 || len(sent()) != 1 {
		t.Errorf("Delete = %d, %v", n, err)
	}
}

// R's tags must be field IDs: alias tags panic at NewTable, not mid-query.
func TestNewTableNeedsFieldIDTags(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("alias tags didn't panic")
		}
	}()
	type byAlias struct {
		Name string `qb:"name"`
	}
	quickbase.NewTable[byAlias, byAlias]("bqx")
}

func TestTableUpsertRefusesNoChanges(t *testing.T) {
	qb, sent := tableServer(t, nil)
	if _, err := projects.Upsert(context.Background(), qb); err == nil || len(sent()) != 0 {
		t.Errorf("an empty upsert: err = %v, %d sent", err, len(sent()))
	}
}

// Limit asks QuickBase for that many records, not a full page
func TestTableLimitSendsTop(t *testing.T) {
	qb, sent := tableServer(t, []map[string]any{row(1, "a", "", nil), row(2, "b", "", nil)})
	_, _ = projects.Query(context.Background(), qb, quickbase.TableQuery{Limit: 1})
	opts, _ := sent()[0]["options"].(map[string]any)
	if opts["top"] != float64(1) {
		t.Errorf("sent options %v, want top 1", opts)
	}
}

func TestListIterators(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/relationships"):
			if r.URL.Query().Get("skip") == "0" {
				_, _ = io.WriteString(w, `{"relationships":[{"id":6},{"id":7}],"metadata":{"skip":0,"numRelationships":2,"totalRelationships":3}}`)
			} else {
				_, _ = io.WriteString(w, `{"relationships":[{"id":8}],"metadata":{"skip":2,"numRelationships":1,"totalRelationships":3}}`)
			}
		default: // users
			raw, _ := io.ReadAll(r.Body)
			if strings.Contains(string(raw), "page2") {
				_, _ = io.WriteString(w, `{"users":[{"userName":"c"}],"metadata":{"nextPageToken":""}}`)
			} else {
				_, _ = io.WriteString(w, `{"users":[{"userName":"a"},{"userName":"b"}],"metadata":{"nextPageToken":"page2"}}`)
			}
		}
	}))
	defer srv.Close()
	qb, _ := quickbase.New("r", quickbase.WithUserToken("t"), quickbase.WithBaseURL(srv.URL))
	ctx := context.Background()

	var rels []int
	for rel, err := range qb.GetRelationshipsIterator(ctx, "bqchild") {
		if err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel.Id)
	}
	if len(rels) != 3 || rels[2] != 8 {
		t.Errorf("relationships = %v", rels)
	}

	var users []string
	for u, err := range qb.GetUsersIterator(ctx, quickbase.GetUsersParams{}, quickbase.GetUsersJSONRequestBody{}) {
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, u.UserName)
	}
	if strings.Join(users, ",") != "a,b,c" {
		t.Errorf("users = %v", users)
	}
}

func TestAuditIterator(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		tok, _ := body["nextToken"].(string)
		tokens = append(tokens, tok+"|"+fmt.Sprint(body["date"]))
		w.Header().Set("Content-Type", "application/json")
		if tok == "" {
			_, _ = io.WriteString(w, `{"queryId":"q1","nextToken":"t2","events":[{"id":"1"},{"id":"2"}]}`)
		} else {
			_, _ = io.WriteString(w, `{"queryId":"q1","nextToken":"","events":[{"id":"3"}]}`)
		}
	}))
	defer srv.Close()
	qb, _ := quickbase.New("r", quickbase.WithUserToken("t"), quickbase.WithBaseURL(srv.URL))

	var ids []string
	day := quickbase.Date{Year: 2024, Month: 1, Day: 15}
	for e, err := range qb.AuditIterator(context.Background(), quickbase.AuditBody{Date: &day}) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.Id)
	}
	if strings.Join(ids, ",") != "1,2,3" || tokens[1] != "t2|2024-01-15" {
		t.Errorf("events %v, requests %v", ids, tokens)
	}
}
