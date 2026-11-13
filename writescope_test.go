package quickbase_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
)

// scopeServer is a fake QuickBase with two apps: "appin" (tables
// "tin1", "tin2") and "appout" (table "tout1"). It answers getAppTables
// and records every other request that reaches it.
type scopeServer struct {
	*httptest.Server
	mu     sync.Mutex
	tables map[string][]string // app -> tables
	// child table -> relationship ID -> parent table
	relationships map[string]map[int]string
	writes        []string
	listCalls     int
	listFails     bool
}

func newScopeServer(t *testing.T) *scopeServer {
	s := &scopeServer{
		tables: map[string][]string{
			"appin":  {"tin1", "tin2"},
			"appout": {"tout1"},
		},
		relationships: map[string]map[int]string{
			"tin2": {10: "tout1", 11: "tin1"},
		},
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if m := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/tables/"), "/relationships"); r.Method == http.MethodGet && m != r.URL.Path && strings.HasSuffix(r.URL.Path, "/relationships") {
			rels := []map[string]any{}
			for id, parent := range s.relationships[m] {
				rels = append(rels, map[string]any{"id": id, "parentTableId": parent, "childTableId": m})
			}
			json.NewEncoder(w).Encode(map[string]any{"relationships": rels, "metadata": map[string]int{"totalRelationships": len(rels)}})
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/tables" {
			s.listCalls++
			if s.listFails {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message":"Access denied"}`))
				return
			}
			var items []map[string]string
			for _, id := range s.tables[r.URL.Query().Get("appId")] {
				items = append(items, map[string]string{"id": id})
			}
			json.NewEncoder(w).Encode(items)
			return
		}
		s.writes = append(s.writes, r.Method+" "+r.URL.Path)
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *scopeServer) reached() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.writes...)
}

func scopedClient(t *testing.T, s *scopeServer, opts ...quickbase.Option) *quickbase.Client {
	t.Helper()
	all := append([]quickbase.Option{quickbase.WithUserToken("token"), quickbase.WithBaseURL(s.URL), quickbase.WithMaxRetries(0)}, opts...)
	qb, err := quickbase.New("testrealm", all...)
	if err != nil {
		t.Fatal(err)
	}
	return qb
}

func wantScopeError(t *testing.T, name string, err error) {
	t.Helper()
	var scopeErr *quickbase.WriteScopeError
	if !errors.As(err, &scopeErr) {
		t.Errorf("%s: err = %v, want WriteScopeError", name, err)
	}
}

// wantAllowed fails when either setting refused the call. Tests also count
// the writes that reached the server, so an allowed call that never left
// the client is caught.
func wantAllowed(t *testing.T, name string, err error) {
	t.Helper()
	if errors.Is(err, quickbase.ErrWriteBlocked) {
		t.Errorf("%s: refused, want allowed: %v", name, err)
	}
}

func wantReached(t *testing.T, s *scopeServer, want int) {
	t.Helper()
	if got := s.reached(); len(got) != want {
		t.Errorf("%d requests reached the server, want %d: %v", len(got), want, got)
	}
}

func TestWriteScopeAllowsTheScopesApps(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	ctx := context.Background()

	_, err := qb.CreateField(ctx, "tin1", quickbase.CreateFieldJSONRequestBody{Label: "Priority", FieldType: "text"})
	wantAllowed(t, "createField", err)
	_, err = qb.DeleteFields(ctx, "tin2", quickbase.DeleteFieldsJSONRequestBody{FieldIds: []int{7}})
	wantAllowed(t, "deleteFields", err)
	_, err = qb.Upsert(ctx, quickbase.UpsertBody{To: "tin1"})
	wantAllowed(t, "upsert", err)
	_, err = qb.DeleteRecords(ctx, quickbase.DeleteRecordsJSONRequestBody{From: "tin2", Where: "{3.GT.0}"})
	wantAllowed(t, "deleteRecords", err)
	_, err = qb.CreateTable(ctx, "appin", quickbase.CreateTableJSONRequestBody{Name: "New"})
	wantAllowed(t, "createTable", err)
	_, err = qb.UpdateApp(ctx, "appin", quickbase.UpdateAppJSONRequestBody{})
	wantAllowed(t, "updateApp", err)
	_, err = qb.UpdateTable(ctx, "tin1", "appin", quickbase.UpdateTableJSONRequestBody{})
	wantAllowed(t, "updateTable", err)
	_, err = qb.CreateRelationship(ctx, "tin2", quickbase.CreateRelationshipJSONRequestBody{ParentTableId: "tin1"})
	wantAllowed(t, "createRelationship", err)

	if got := len(s.reached()); got != 8 {
		t.Errorf("%d writes reached the server, want 8: %v", got, s.reached())
	}
	if s.listCalls != 1 {
		t.Errorf("listed tables %d times, want 1 (cached)", s.listCalls)
	}
}

func TestWriteScopeRefusesEverythingElse(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	ctx := context.Background()

	_, err := qb.CreateField(ctx, "tout1", quickbase.CreateFieldJSONRequestBody{Label: "Priority", FieldType: "text"})
	wantScopeError(t, "createField outside the scope", err)
	_, err = qb.DeleteFields(ctx, "tout1", quickbase.DeleteFieldsJSONRequestBody{FieldIds: []int{7}})
	wantScopeError(t, "deleteFields outside the scope", err)
	_, err = qb.Upsert(ctx, quickbase.UpsertBody{To: "tout1"})
	wantScopeError(t, "upsert outside the scope", err)
	_, err = qb.DeleteRecords(ctx, quickbase.DeleteRecordsJSONRequestBody{From: "tout1", Where: "{3.GT.0}"})
	wantScopeError(t, "deleteRecords outside the scope", err)
	_, err = qb.CreateTable(ctx, "appout", quickbase.CreateTableJSONRequestBody{Name: "New"})
	wantScopeError(t, "createTable outside the scope", err)
	_, err = qb.UpdateApp(ctx, "appout", quickbase.UpdateAppJSONRequestBody{})
	wantScopeError(t, "updateApp outside the scope", err)
	_, err = qb.DeleteApp(ctx, "appout", quickbase.DeleteAppJSONRequestBody{Name: "Out"})
	wantScopeError(t, "deleteApp outside the scope", err)
	_, err = qb.UpdateTable(ctx, "tout1", "appin", quickbase.UpdateTableJSONRequestBody{})
	wantScopeError(t, "updateTable table outside the scope under an in-scope appId", err)
	_, err = qb.CreateRelationship(ctx, "tin1", quickbase.CreateRelationshipJSONRequestBody{ParentTableId: "tout1"})
	wantScopeError(t, "relationship whose parent is outside the scope", err)
	_, err = qb.DeleteFile(ctx, "tout1", 1, 6, 0)
	wantScopeError(t, "deleteFile outside the scope", err)

	// No app to check
	_, err = qb.CreateApp(ctx, quickbase.CreateAppJSONRequestBody{Name: "New"})
	wantScopeError(t, "createApp", err)
	_, err = qb.CopyApp(ctx, "appin", quickbase.CopyAppJSONRequestBody{Name: "Copy"})
	wantScopeError(t, "copyApp", err)
	_, err = qb.DenyUsers(ctx, quickbase.DenyUsersJSONRequestBody{"u1"}, nil)
	wantScopeError(t, "denyUsers", err)
	_, err = qb.AddMembersToGroup(ctx, 1, quickbase.AddMembersToGroupJSONRequestBody{"u1"})
	wantScopeError(t, "addMembersToGroup", err)
	_, err = qb.CloneUserToken(ctx, quickbase.CloneUserTokenJSONRequestBody{})
	wantScopeError(t, "cloneUserToken", err)
	_, err = qb.UpdateSolution(ctx, "sol1", []byte("qbl"), nil)
	wantScopeError(t, "updateSolution", err)
	_, err = qb.API().UpdateSolutionToRecord(ctx, "sol1", nil)
	wantScopeError(t, "updateSolutionToRecord", err)

	if got := s.reached(); len(got) != 0 {
		t.Errorf("writes reached the server: %v", got)
	}
}

func TestWriteScopeLeavesReadsAlone(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{}))
	_, err := qb.GetFields(context.Background(), "tout1", nil)
	wantAllowed(t, "getFields outside the scope", err)
	_, err = qb.RunQuery(context.Background(), quickbase.RunQueryBody{From: "tout1"})
	wantAllowed(t, "runQuery outside the scope", err)
	wantReached(t, s, 2)
}

func TestWriteScopeEmptyRefusesAllWrites(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{}))
	_, err := qb.CreateField(context.Background(), "tin1", quickbase.CreateFieldJSONRequestBody{Label: "X", FieldType: "text"})
	wantScopeError(t, "createField with an empty scope", err)
	if s.listCalls != 0 || len(s.reached()) != 0 {
		t.Errorf("an empty scope sent requests: %d lists, writes %v", s.listCalls, s.reached())
	}
}

func TestWriteScopeCreateApps(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{CreateApps: true}))
	ctx := context.Background()
	_, err := qb.CreateApp(ctx, quickbase.CreateAppJSONRequestBody{Name: "New"})
	wantAllowed(t, "createApp", err)
	_, err = qb.CopyApp(ctx, "appout", quickbase.CopyAppJSONRequestBody{Name: "Copy"})
	wantAllowed(t, "copyApp", err)
	wantReached(t, s, 2)
	_, err = qb.UpdateApp(ctx, "appout", quickbase.UpdateAppJSONRequestBody{})
	wantScopeError(t, "updateApp with only CreateApps", err)
}

// A table created after the first lookup is found by listing again.
func TestWriteScopeFindsNewTables(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	ctx := context.Background()
	_, err := qb.CreateField(ctx, "tin1", quickbase.CreateFieldJSONRequestBody{Label: "A", FieldType: "text"})
	wantAllowed(t, "createField", err)

	s.mu.Lock()
	s.tables["appin"] = append(s.tables["appin"], "tin3")
	s.mu.Unlock()

	_, err = qb.CreateField(ctx, "tin3", quickbase.CreateFieldJSONRequestBody{Label: "B", FieldType: "text"})
	wantAllowed(t, "createField in a new table", err)
	wantReached(t, s, 2)
	if s.listCalls != 2 {
		t.Errorf("listed tables %d times, want 2", s.listCalls)
	}
}

// If the scope's tables can't be listed, table writes are refused.
func TestWriteScopeFailsClosed(t *testing.T) {
	s := newScopeServer(t)
	s.listFails = true
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	_, err := qb.CreateField(context.Background(), "tin1", quickbase.CreateFieldJSONRequestBody{Label: "A", FieldType: "text"})
	wantScopeError(t, "createField when listing fails", err)
	if len(s.reached()) != 0 {
		t.Errorf("writes reached the server: %v", s.reached())
	}
}

func TestReadOnlyWinsOverWriteScope(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}, CreateApps: true}), quickbase.WithReadOnly())
	_, err := qb.CreateField(context.Background(), "tin1", quickbase.CreateFieldJSONRequestBody{Label: "A", FieldType: "text"})
	var readOnly *quickbase.ReadOnlyError
	if !errors.As(err, &readOnly) {
		t.Errorf("err = %v, want ReadOnlyError", err)
	}
	if len(s.reached()) != 0 {
		t.Errorf("writes reached the server: %v", s.reached())
	}
}

// XML writes are checked by dbid; this one is refused before anything is sent.
func TestWriteScopeRefusesXMLWritesOutside(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	_, err := qb.DoXML(context.Background(), "appout", "API_AddReplaceDBPage", []byte("<pagename>x.js</pagename>"))
	wantScopeError(t, "API_AddReplaceDBPage outside the scope", err)
	if !strings.Contains(err.Error(), "API_AddReplaceDBPage") {
		t.Errorf("error doesn't name the action: %v", err)
	}
}

// Either kind of refused write matches ErrWriteBlocked.
func TestErrWriteBlockedMatchesBoth(t *testing.T) {
	s := newScopeServer(t)
	ctx := context.Background()
	body := quickbase.CreateFieldJSONRequestBody{Label: "A", FieldType: "text"}

	_, err := scopedClient(t, s, quickbase.WithReadOnly()).CreateField(ctx, "tin1", body)
	if !errors.Is(err, quickbase.ErrWriteBlocked) {
		t.Errorf("read-only: errors.Is(%v, ErrWriteBlocked) = false", err)
	}
	_, err = scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{})).CreateField(ctx, "tin1", body)
	if !errors.Is(err, quickbase.ErrWriteBlocked) {
		t.Errorf("write scope: errors.Is(%v, ErrWriteBlocked) = false", err)
	}
	_, err = scopedClient(t, s).GetFields(ctx, "tin1", nil)
	if errors.Is(err, quickbase.ErrWriteBlocked) {
		t.Errorf("a read matched ErrWriteBlocked: %v", err)
	}
}

// Solutions, groups, users and user tokens are refused unless the scope
// names them, and allowed when it does.
func TestWriteScopeBeyondApps(t *testing.T) {
	ctx := context.Background()

	s := newScopeServer(t)
	none := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	_, err := none.CreateSolution(ctx, []byte("qbl"), nil)
	wantScopeError(t, "createSolution", err)
	_, err = none.API().CreateSolutionFromRecord(ctx, &quickbase.CreateSolutionFromRecordParams{TableId: "tin1", FieldId: 6, RecordId: 1})
	wantScopeError(t, "createSolutionFromRecord", err)
	_, err = none.UpdateSolution(ctx, "sol1", []byte("qbl"), nil)
	wantScopeError(t, "updateSolution", err)
	_, err = none.UpdateSolutionToRecord(ctx, "sol1", "tin1", 6, 1, nil)
	wantScopeError(t, "updateSolutionToRecord", err)
	_, err = none.ExportSolutionToRecord(ctx, "sol1", "tout1", 6, nil)
	wantScopeError(t, "exportSolutionToRecord into a table outside the scope", err)
	_, err = none.GenerateDocument(ctx, 1, "tout1", nil)
	wantScopeError(t, "generateDocument outside the scope", err)
	_, err = none.AddMembersToGroup(ctx, 42, quickbase.AddMembersToGroupJSONRequestBody{"u1"})
	wantScopeError(t, "addMembersToGroup", err)
	_, err = none.UndenyUsers(ctx, quickbase.UndenyUsersJSONRequestBody{"u1"}, nil)
	wantScopeError(t, "undenyUsers", err)
	_, err = none.DeleteUserToken(ctx)
	wantScopeError(t, "deleteUserToken", err)
	if got := s.reached(); len(got) != 0 {
		t.Errorf("writes reached the server: %v", got)
	}

	s = newScopeServer(t)
	all := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{
		AppIDs:          []string{"appin"},
		SolutionIDs:     []string{"sol1"},
		CreateSolutions: true,
		GroupIDs:        []string{"42"},
		ManageUsers:     true,
		ManageTokens:    true,
	}))
	_, err = all.CreateSolution(ctx, []byte("qbl"), nil)
	wantAllowed(t, "createSolution", err)
	_, err = all.UpdateSolution(ctx, "sol1", []byte("qbl"), nil)
	wantAllowed(t, "updateSolution", err)
	_, err = all.UpdateSolution(ctx, "sol2", []byte("qbl"), nil)
	wantScopeError(t, "updateSolution of another solution", err)
	_, err = all.ExportSolutionToRecord(ctx, "sol2", "tin1", 6, nil)
	wantAllowed(t, "exportSolutionToRecord into a table in the scope", err)
	_, err = all.GenerateDocument(ctx, 1, "tin1", nil)
	wantAllowed(t, "generateDocument in the scope", err)
	_, err = all.AddMembersToGroup(ctx, 42, quickbase.AddMembersToGroupJSONRequestBody{"u1"})
	wantAllowed(t, "addMembersToGroup", err)
	_, err = all.RemoveSubgroupsFromGroup(ctx, 7, quickbase.RemoveSubgroupsFromGroupJSONRequestBody{"42"})
	wantScopeError(t, "removeSubgroupsFromGroup of another group", err)
	_, err = all.DenyUsers(ctx, quickbase.DenyUsersJSONRequestBody{"u1"}, nil)
	wantAllowed(t, "denyUsers", err)
	_, err = all.DenyUsersAndGroups(ctx, true, quickbase.DenyUsersAndGroupsJSONRequestBody{"u1"}, nil)
	wantAllowed(t, "denyUsersAndGroups", err)
	_, err = all.CloneUserToken(ctx, quickbase.CloneUserTokenJSONRequestBody{})
	wantAllowed(t, "cloneUserToken", err)
	_, err = all.DeleteUserToken(ctx)
	wantAllowed(t, "deleteUserToken", err)
	wantReached(t, s, 9)
}

// changesetSolution only lists what an update would change: read-only and a
// write scope let it through.
func TestChangesetSolutionIsARead(t *testing.T) {
	s := newScopeServer(t)
	for name, qb := range map[string]*quickbase.Client{
		"read-only":   scopedClient(t, s, quickbase.WithReadOnly()),
		"write scope": scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{})),
	} {
		_, err := qb.ChangesetSolution(context.Background(), "sol1", []byte("qbl"), nil)
		if errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: changesetSolution refused: %v", name, err)
		}
	}
	wantReached(t, s, 2)
}

// Findings from the adversarial review: each request here got past
// read-only or the write scope before the fix.

// An encoded slash in an ID: "sol1/changeset" was judged as changesetSolution
// (a read) but sent as PUT /solutions/sol1%2Fchangeset.
func TestAmbiguousPathsAreRefused(t *testing.T) {
	s := newScopeServer(t)
	ctx := context.Background()
	for name, qb := range map[string]*quickbase.Client{
		"read-only":   scopedClient(t, s, quickbase.WithReadOnly()),
		"write scope": scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{SolutionIDs: []string{"sol1"}})),
	} {
		_, err := qb.UpdateSolution(ctx, "sol1/changeset", []byte("qbl"), nil)
		if !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: updateSolution(\"sol1/changeset\"): err = %v, want refused", name, err)
		}
		_, err = qb.GetTable(ctx, "../apps/appout", "appin")
		if !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: a dot segment in a path: err = %v, want refused", name, err)
		}
	}
	wantReached(t, s, 0)
}

// XML actions not known to read are writes, in any letter case; a dbid that
// isn't a plain ID is refused before it can change the URL.
func TestXMLUnknownActionsAreWrites(t *testing.T) {
	s := newScopeServer(t)
	ctx := context.Background()
	for name, qb := range map[string]*quickbase.Client{
		"read-only":   scopedClient(t, s, quickbase.WithReadOnly()),
		"write scope": scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}})),
	} {
		for _, action := range []string{"API_DeleteDatabase", "api_deletedatabase", "API_CloneDatabase", "API_AddField", "API_SetFieldProperties", "API_UploadFile"} {
			if _, err := qb.DoXML(ctx, "appout", action, nil); !errors.Is(err, quickbase.ErrWriteBlocked) {
				t.Errorf("%s: %s: err = %v, want refused", name, action, err)
			}
		}
		if _, err := qb.DoXML(ctx, "appin?a=API_DeleteDatabase", "API_GetSchema", nil); !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: a dbid with a query: err = %v, want refused", name, err)
		}
	}
	wantReached(t, s, 0)
}

// A relationship's parent: required on create, looked up on update and delete.
func TestRelationshipParentIsChecked(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	ctx := context.Background()

	_, err := qb.CreateRelationship(ctx, "tin1", quickbase.CreateRelationshipJSONRequestBody{})
	wantScopeError(t, "createRelationship without a parent", err)
	_, err = qb.UpdateRelationship(ctx, "tin2", 10, quickbase.UpdateRelationshipJSONRequestBody{})
	wantScopeError(t, "updateRelationship whose parent is outside the scope", err)
	_, err = qb.DeleteRelationship(ctx, "tin2", 10)
	wantScopeError(t, "deleteRelationship whose parent is outside the scope", err)
	_, err = qb.DeleteRelationship(ctx, "tin2", 99)
	wantScopeError(t, "deleteRelationship that can't be found", err)
	wantReached(t, s, 0)

	_, err = qb.UpdateRelationship(ctx, "tin2", 11, quickbase.UpdateRelationshipJSONRequestBody{})
	wantAllowed(t, "updateRelationship whose parent is in the scope", err)
	wantReached(t, s, 1)
}

// recordsModifiedSince is a read sent as POST.
func TestRecordsModifiedSinceIsARead(t *testing.T) {
	s := newScopeServer(t)
	for name, qb := range map[string]*quickbase.Client{
		"read-only":   scopedClient(t, s, quickbase.WithReadOnly()),
		"write scope": scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{})),
	} {
		_, err := qb.RecordsModifiedSince(context.Background(), quickbase.RecordsModifiedSinceJSONRequestBody{From: "tout1"})
		if errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: recordsModifiedSince refused: %v", name, err)
		}
	}
	wantReached(t, s, 2)
}

// Second adversarial review.

// A QUICKBASE-ACTION header on a JSON request doesn't make it an XML read.
func TestReadOnlyIgnoresXMLHeaderOnJSON(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithReadOnly())
	editor := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("QUICKBASE-ACTION", "API_DoQuery")
		return nil
	}
	_, err := qb.API().DeleteApp(context.Background(), "appout", quickbase.DeleteAppJSONRequestBody{Name: "x"}, editor)
	if !errors.Is(err, quickbase.ErrWriteBlocked) {
		t.Errorf("deleteApp with an XML action header: err = %v, want refused", err)
	}
	wantReached(t, s, 0)
}

// Method-override headers, trailing slashes and other letter cases can't
// change what a request is judged as.
func TestGuardsAreNotFooledByRouting(t *testing.T) {
	s := newScopeServer(t)
	ctx := context.Background()
	override := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("X-HTTP-Method-Override", "DELETE")
		return nil
	}
	upper := func(ctx context.Context, req *http.Request) error {
		req.URL.Path = strings.Replace(req.URL.Path, "/solutions/", "/Solutions/", 1)
		return nil
	}
	trailing := func(ctx context.Context, req *http.Request) error {
		req.URL.Path += "/"
		return nil
	}
	for name, qb := range map[string]*quickbase.Client{
		"read-only":   scopedClient(t, s, quickbase.WithReadOnly()),
		"write scope": scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}})),
	} {
		_, err := qb.API().GetApp(ctx, "appout", override)
		if !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: a method-override header: err = %v, want refused", name, err)
		}
		params := &quickbase.CreateSolutionFromRecordParams{TableId: "tin1", FieldId: 6, RecordId: 1}
		_, err = qb.API().UpdateSolutionToRecord(ctx, "sol1", &quickbase.UpdateSolutionToRecordParams{TableId: "tin1", FieldId: 6, RecordId: 1}, upper)
		if !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: /Solutions/{id}/fromrecord: err = %v, want refused", name, err)
		}
		_, err = qb.API().CreateSolutionFromRecord(ctx, params, trailing)
		if !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: /solutions/fromrecord/: err = %v, want refused", name, err)
		}
	}
	wantReached(t, s, 0)
}

// Trustees and a table update's appId are part of the scope check.
func TestWriteScopeChecksTrusteesAndTableAppID(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	ctx := context.Background()
	_, err := qb.AddTrustees(ctx, "appout", quickbase.AddTrusteesJSONRequestBody{})
	wantScopeError(t, "addTrustees outside the scope", err)
	_, err = qb.UpdateTable(ctx, "tin1", "appout", quickbase.UpdateTableJSONRequestBody{})
	wantScopeError(t, "updateTable with an appId outside the scope", err)
	wantReached(t, s, 0)
	_, err = qb.AddTrustees(ctx, "appin", quickbase.AddTrusteesJSONRequestBody{})
	wantAllowed(t, "addTrustees in the scope", err)
	wantReached(t, s, 1)
}

// A relationship lookup ends even when the server ignores skip and leaves
// out the total.
func TestRelationshipLookupEnds(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/tables":
			w.Write([]byte(`[{"id":"tin1"}]`))
		case strings.HasSuffix(r.URL.Path, "/relationships"):
			calls++
			w.Write([]byte(`{"relationships":[{"id":5,"parentTableId":"tin1"}],"metadata":{}}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	qb, _ := quickbase.New("testrealm", quickbase.WithUserToken("token"), quickbase.WithBaseURL(srv.URL), quickbase.WithMaxRetries(0),
		quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}}))
	_, err := qb.DeleteRelationship(context.Background(), "tin1", 99)
	wantScopeError(t, "deleteRelationship that can't be found", err)
	if calls != 1 {
		t.Errorf("listed relationships %d times, want 1 (the page has no total, so it's the last)", calls)
	}
}

// Third adversarial review.

// A request editor can't send a path other than the one judged, an unusual
// method, or a query Go would read differently.
func TestGuardsJudgeWhatIsSent(t *testing.T) {
	s := newScopeServer(t)
	ctx := context.Background()
	opaque := func(ctx context.Context, req *http.Request) error {
		req.URL.Opaque = "/records"
		return nil
	}
	method := func(ctx context.Context, req *http.Request) error {
		req.Method = "PROPPATCH"
		return nil
	}
	emptySegment := func(ctx context.Context, req *http.Request) error {
		req.URL.Path = "/apps//appout"
		return nil
	}
	semicolon := func(ctx context.Context, req *http.Request) error {
		req.URL.RawQuery = "tableId=tout1;&tableId=tin1"
		return nil
	}
	for name, qb := range map[string]*quickbase.Client{
		"read-only":   scopedClient(t, s, quickbase.WithReadOnly()),
		"write scope": scopedClient(t, s, quickbase.WithWriteScope(quickbase.WriteScope{AppIDs: []string{"appin"}})),
	} {
		// In scope, so only the trick itself can get these refused
		for what, editor := range map[string]quickbase.RequestEditorFn{"opaque URL": opaque, "empty segment": emptySegment} {
			_, err := qb.API().GetApp(ctx, "appin", editor)
			if !errors.Is(err, quickbase.ErrWriteBlocked) {
				t.Errorf("%s: %s: err = %v, want refused", name, what, err)
			}
		}
		// An unusual method is a write: read-only refuses it, and the scope
		// refuses it outside the scope
		_, err := qb.API().GetApp(ctx, "appout", method)
		if !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: PROPPATCH: err = %v, want refused", name, err)
		}
		_, err = qb.API().CreateFieldWithBody(ctx, &quickbase.CreateFieldParams{TableId: "tin1"}, "application/json", strings.NewReader(`{"label":"x","fieldType":"text"}`), semicolon)
		if !errors.Is(err, quickbase.ErrWriteBlocked) {
			t.Errorf("%s: a query with \";\": err = %v, want refused", name, err)
		}
	}
	wantReached(t, s, 0)
}

// changesetSolution is a read in any letter case.
func TestChangesetSolutionAnyCase(t *testing.T) {
	s := newScopeServer(t)
	qb := scopedClient(t, s, quickbase.WithReadOnly())
	upper := func(ctx context.Context, req *http.Request) error {
		req.URL.Path = strings.Replace(strings.Replace(req.URL.Path, "/solutions/", "/Solutions/", 1), "/changeset", "/Changeset", 1)
		return nil
	}
	_, err := qb.API().ChangesetSolutionWithBody(context.Background(), "sol1", nil, "application/x-yaml", strings.NewReader("qbl"), upper)
	if errors.Is(err, quickbase.ErrWriteBlocked) {
		t.Errorf("/Solutions/sol1/Changeset refused: %v", err)
	}
	wantReached(t, s, 1)
}
