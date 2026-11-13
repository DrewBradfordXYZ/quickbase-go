package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
)

// XML writes are checked by gid (groups), by ManageTokens (sign-out), and by
// dbid (everything else). Checked directly: an allowed XML call would go to
// the realm's real host.
func TestXMLWriteScope(t *testing.T) {
	// Table lookups go to a fake server, never to QuickBase
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/tables" && r.URL.Query().Get("appId") == "appin" {
			w.Write([]byte(`[{"id":"tin1"}]`))
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c, err := New("testrealm", nil, WithUserToken("token"), WithBaseURL(srv.URL), WithWriteScope(WriteScope{
		AppIDs:   []string{"appin"},
		GroupIDs: []string{"42", "1217.dgpt"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	cases := []struct {
		dbid, action, body string
		allowed            bool
	}{
		{"appin", "API_AddReplaceDBPage", "<pagename>x.js</pagename>", true},
		{"appin", "API_SetDBVar", "<varname>v</varname>", true},
		{"appin", "API_AddUserToRole", "<userid>u1</userid>", true},
		{"tin1", "API_AddRecord", "", true},
		{"tin1", "API_FieldAddChoices", "", true},
		{"main", "API_AddUserToGroup", "<gid>42</gid><userid>u1</userid>", true},
		{"main", "API_AddSubGroup", "<gid> 42 </gid><subgroupid>7</subgroupid>", true},
		{"main", "API_AddUserToGroup", "<gid>7</gid>", false},
		{"main", "API_DeleteGroup", "", false},
		{"main", "API_CreateGroup", "<name>g</name>", false},
		{"main", "API_CopyGroup", "<gid>42</gid>", false},
		{"main", "API_SignOut", "", false},
		{"appout", "API_AddReplaceDBPage", "", false},
		{"appin", "API_GetSchema", "", true}, // a read
		{"main", "api_addusertogroup", "<GID>42</GID>", true},
		{"main", "API_AddUserToGroup", "<!-- <gid>42</gid> --><gid>99</gid>", false},
		{"main", "API_AddUserToGroup", "<gid>42</gid><gid>99</gid>", false},
		{"main", "API_AddUserToGroup", "<gid><![CDATA[42]]></gid>", false},
		{"main", "API_AddUserToGroup", "<gid/><gid>42</gid>", false},
		{"main", "API_SIGNOUT", "", false},
		{"appout", "API_DeleteDatabase", "", false},
		{"main", "API_AddUserToGroup", "<gid>1217.dgpt</gid>", true},
		{"main", "API_AddUserToGroup", "<gid>1217</gid>", false},
		{"main", "API_AddUserToGroup", "<x:gid>99</x:gid><gid>42</gid>", false},
		{"main", "API_AddUserToGroup", `<!DOCTYPE q [<!ENTITY e "&#60;gid>99&#60;/gid>">]><gid>42</gid>&e;`, false},
		{"appin", "API_CloneDatabase", "<newdbname>x</newdbname>", false},
		{"main", "API_AddUserToGroup", `<?xml-stylesheet x?><gid>42</gid>`, false},
		{"appin", "API_CreateDatabase", "", false},
		{"appin", "API_ProvisionUser", "<email>a@b.c</email>", false},
		{"appout", "api_adddfield", "", false},
	}
	for _, tc := range cases {
		err := c.checkXMLWriteScope(ctx, tc.dbid, tc.action, []byte(tc.body))
		var scopeErr *core.WriteScopeError
		if refused := errors.As(err, &scopeErr); refused == tc.allowed {
			t.Errorf("%s %s %q: err = %v, want allowed = %v", tc.dbid, tc.action, tc.body, err, tc.allowed)
		}
	}

	c, _ = New("testrealm", nil, WithUserToken("token"), WithBaseURL(srv.URL), WithWriteScope(WriteScope{
		AppIDs: []string{"appin"}, CreateApps: true, CreateGroups: true, ManageUsers: true, ManageTokens: true,
	}))
	if err := c.checkXMLWriteScope(ctx, "appin", "API_ProvisionUser", nil); err != nil {
		t.Errorf("API_ProvisionUser with ManageUsers and the app in scope: %v", err)
	}
	if err := c.checkXMLWriteScope(ctx, "appout", "API_ProvisionUser", nil); err == nil {
		t.Error("API_ProvisionUser on an app outside the scope was allowed")
	}
	for _, action := range []string{"API_CreateGroup", "API_CopyGroup", "API_SignOut", "API_CloneDatabase", "API_CreateDatabase"} {
		if err := c.checkXMLWriteScope(ctx, "main", action, nil); err != nil {
			t.Errorf("%s with the switch on: %v", action, err)
		}
	}
}

// A body or query that names a checked key twice, or in another letter case,
// is ambiguous: a server might act on the other one.
func TestAmbiguousBodiesAndQueries(t *testing.T) {
	for _, tc := range []struct {
		body  string
		value string
		ok    bool
	}{
		{`{"to":"tin1"}`, "tin1", true},
		{`{"other":1}`, "", true},
		{`{"to":"tin1","To":"tout1"}`, "", false},
		{`{"To":"tout1"}`, "", false},
		{`{"to":"tin1","to":"tout1"}`, "", false},
		{`{"to":["tin1"]}`, "", false},
		{`[]`, "", false},
	} {
		value, ok := bodyField([]byte(tc.body), "to")
		if value != tc.value || ok != tc.ok {
			t.Errorf("bodyField(%s) = %q, %v; want %q, %v", tc.body, value, ok, tc.value, tc.ok)
		}
	}
	for _, tc := range []struct {
		query string
		ok    bool
	}{
		{"tableId=tin1", true},
		{"tableId=tin1&tableId=tout1", false},
		{"tableId=tin1&TableId=tout1", false},
	} {
		values, _ := url.ParseQuery(tc.query)
		if _, ok := queryParam(values, "tableId"); ok != tc.ok {
			t.Errorf("queryParam(%s) ok = %v, want %v", tc.query, ok, tc.ok)
		}
	}
}

func TestXMLActionNamesAreTrimmed(t *testing.T) {
	if isXMLWriteAction(" API_GetSchema ") {
		t.Error(`isXMLWriteAction(" API_GetSchema ") = true, want a read`)
	}
}
