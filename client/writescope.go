package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/generated"
)

// WriteScope lists what a client may write to. See [WithWriteScope].
//
// A write whose target can be checked is allowed when the target is listed:
// an app, a solution or a group. A write that creates something, or acts on
// the realm's users or your own credentials, is allowed by a switch. Nothing
// is allowed unless it is named here.
type WriteScope struct {
	// AppIDs are the apps the client may change: the apps themselves
	// (update, delete, trustees, roles, variables, code pages), and every
	// table in them, including tables created after the client (fields,
	// relationships, records, files, document generation).
	AppIDs []string

	// CreateApps allows creating apps (createApp, copyApp, and the XML
	// API_CreateDatabase and API_CloneDatabase). A new app isn't added to
	// AppIDs: build a client that names it to write to it.
	CreateApps bool

	// SolutionIDs are the solutions the client may update (updateSolution,
	// updateSolutionToRecord). Updating a solution changes its apps.
	SolutionIDs []string

	// CreateSolutions allows creating solutions (createSolution,
	// createSolutionFromRecord), which creates their apps.
	CreateSolutions bool

	// GroupIDs are the groups whose members, managers and subgroups the
	// client may change, and that it may rename or delete (XML). They are
	// matched exactly, as the request writes them: the JSON API writes a
	// group's ID as a number ("1217"), and the XML API's examples write one
	// with a suffix ("1217.dgpt"); list each form the client will use.
	GroupIDs []string

	// CreateGroups allows creating and copying groups (XML).
	CreateGroups bool

	// ManageUsers allows denying and undenying users in the account, and,
	// with the app in AppIDs, provisioning a user into an app (XML
	// API_ProvisionUser, which can create the realm user).
	ManageUsers bool

	// ManageTokens allows changing the client's own credentials: cloning,
	// transferring, deactivating and deleting user tokens, and signing out
	// (XML).
	ManageTokens bool
}

// WithWriteScope limits writes to what a [WriteScope] names. Any other write
// returns a [core.WriteScopeError] before it is sent. Reads are never
// limited.
//
// A table's write is allowed when its table is in one of AppIDs. To tell, the
// client lists those apps' tables (getAppTables) the first time it needs to,
// and again when it meets a table it hasn't seen; if that fails, the write is
// refused. A relationship's parent table must be in scope too, since
// QuickBase adds fields to it. An XML write is checked by its dbid (an app or
// a table), or, for groups, by its gid.
//
// [WithReadOnly] wins over a write scope: a client with both writes nothing.
func WithWriteScope(scope WriteScope) Option {
	return func(c *Client) {
		s := &writeScope{
			apps:            map[string]bool{},
			solutions:       map[string]bool{},
			groups:          map[string]bool{},
			createApps:      scope.CreateApps,
			createSolutions: scope.CreateSolutions,
			createGroups:    scope.CreateGroups,
			manageUsers:     scope.ManageUsers,
			manageTokens:    scope.ManageTokens,
			tableApp:        map[string]string{},
		}
		for _, id := range scope.AppIDs {
			if id = strings.TrimSpace(id); id != "" {
				s.apps[id] = true
			}
		}
		for _, id := range scope.SolutionIDs {
			if id = strings.TrimSpace(id); id != "" {
				s.solutions[id] = true
			}
		}
		for _, id := range scope.GroupIDs {
			if id = strings.TrimSpace(id); id != "" {
				s.groups[id] = true
			}
		}
		c.writeScope = s
	}
}

// writeScope is a client's WriteScope, with the tables known to be in its apps.
type writeScope struct {
	apps, solutions map[string]bool
	groups          map[string]bool
	createApps      bool
	createSolutions bool
	createGroups    bool
	manageUsers     bool
	manageTokens    bool

	mu       sync.Mutex        // guards tableApp; never held across a request
	tableApp map[string]string // table ID -> app ID, for the scope's apps
	loadMu   sync.Mutex        // one listing of the apps' tables at a time
}

// Write targets, in a path under /v1.
var (
	scopeAppPath      = regexp.MustCompile(`(?i)^/v1/apps/([^/]+)(/copy)?$`)
	scopeTrusteesPath = regexp.MustCompile(`(?i)^/v1/app/([^/]+)/trustees$`)
	scopeTablePath    = regexp.MustCompile(`(?i)^/v1/tables/([^/]+)(/relationship(/[^/]+)?)?$`)
	scopeFieldPath    = regexp.MustCompile(`(?i)^/v1/fields(/[^/]+)?$`)
	scopeFilePath     = regexp.MustCompile(`(?i)^/v1/files/([^/]+)/`)
	scopeDocumentPath = regexp.MustCompile(`(?i)^/v1/docTemplates/[^/]+/generate$`)
	scopeSolutionPath = regexp.MustCompile(`(?i)^/v1/solutions/([^/]+)(/fromrecord|/torecord)?$`)
	scopeGroupPath    = regexp.MustCompile(`(?i)^/v1/groups/([^/]+)/(members|managers|subgroups)$`)
	scopeUsersPath    = regexp.MustCompile(`(?i)^/v1/users/(deny|undeny)(/[^/]+)?$`)
	scopeTokenPath    = regexp.MustCompile(`(?i)^/v1/usertoken(/(clone|transfer|deactivate))?$`)
)

// XML write actions that aren't checked by their dbid.
var (
	xmlGroupCreateActions = map[string]bool{"api_creategroup": true, "api_copygroup": true}
	xmlGroupActions       = map[string]bool{
		"api_deletegroup": true, "api_addusertogroup": true, "api_removeuserfromgroup": true,
		"api_changegroupinfo": true, "api_addsubgroup": true, "api_removesubgroup": true,
	}
	xmlTokenActions     = map[string]bool{"api_signout": true}
	xmlAppCreateActions = map[string]bool{"api_createdatabase": true, "api_clonedatabase": true}
	xmlUserActions      = map[string]bool{"api_provisionuser": true}
	xmlGIDElement       = regexp.MustCompile(`(?i)<gid\s*>\s*([A-Za-z0-9._-]+)\s*</gid\s*>`)
	// Any gid element, namespaced (<x:gid>) or not
	xmlGIDOpen = regexp.MustCompile(`(?i)<([A-Za-z0-9._-]+:)?gid[\s/>]`)
)

// checkWriteScope refuses a JSON API write outside the client's write scope.
// It runs after aliases are resolved, so the IDs it checks are the ones sent.
func (c *Client) checkWriteScope(ctx context.Context, req *http.Request, body []byte) error {
	s := c.writeScope
	if s == nil {
		return nil
	}
	method := strings.ToUpper(req.Method)
	path, safe := c.requestPath(req)
	refuse := func(target, reason string) error {
		return core.NewWriteScopeError(method, path, "", target, reason)
	}
	if !safe {
		return refuse("", "the path can't be checked: a server could read it as another path")
	}
	if hasMethodOverride(req) {
		return refuse("", "a method-override header would make the server act as another method")
	}
	if !isJSONWrite(method, path) {
		return nil
	}
	// ParseQuery reports pairs it can't read (Go drops a pair with a ";",
	// which a server might read); URL.Query() would hide them
	values, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return refuse("", "the query can't be read unambiguously")
	}
	param := func(key string) (string, error) {
		v, ok := queryParam(values, key)
		if !ok {
			return "", refuse("", "the "+key+" parameter is ambiguous")
		}
		return v, nil
	}
	field := func(key string) (string, error) {
		v, ok := bodyField(body, key)
		if !ok {
			return "", refuse("", "the body's "+key+" is ambiguous")
		}
		return v, nil
	}

	switch {
	// Apps
	case method == http.MethodPost && strings.EqualFold(path, "/v1/apps"):
		return allow(s.createApps, "", "creating apps isn't allowed", refuse)
	case scopeAppPath.MatchString(path):
		m := scopeAppPath.FindStringSubmatch(path)
		if strings.EqualFold(m[2], "/copy") {
			return allow(s.createApps, m[1], "copying apps isn't allowed", refuse)
		}
		return c.requireApp(m[1], refuse)
	case scopeTrusteesPath.MatchString(path):
		return c.requireApp(scopeTrusteesPath.FindStringSubmatch(path)[1], refuse)

	// Tables
	case method == http.MethodPost && strings.EqualFold(path, "/v1/tables"):
		appID, err := param("appId")
		if err != nil {
			return err
		}
		return c.requireApp(appID, refuse)
	case scopeTablePath.MatchString(path):
		m := scopeTablePath.FindStringSubmatch(path)
		appID, err := param("appId")
		if err != nil {
			return err
		}
		if appID != "" {
			if err := c.requireApp(appID, refuse); err != nil {
				return err
			}
		}
		if err := c.requireTable(ctx, m[1], refuse); err != nil {
			return err
		}
		if m[2] == "" {
			return nil
		}
		// A relationship adds lookup fields to its child table and summary
		// fields to its parent, so the parent must be in scope too
		if m[3] == "" { // createRelationship names the parent in its body
			parent, err := field("parentTableId")
			if err != nil {
				return err
			}
			if parent == "" {
				return refuse("", "a relationship needs its parentTableId")
			}
			return c.requireTable(ctx, parent, refuse)
		}
		parent, err := c.relationshipParent(ctx, m[1], strings.TrimPrefix(m[3], "/"))
		if err != nil {
			return refuse(m[1], "finding the relationship's parent table failed: "+err.Error())
		}
		return c.requireTable(ctx, parent, refuse)
	case scopeFieldPath.MatchString(path):
		tableID, err := param("tableId")
		if err != nil {
			return err
		}
		return c.requireTable(ctx, tableID, refuse)
	case strings.EqualFold(path, "/v1/records"):
		key := "to" // upsert
		if method == http.MethodDelete {
			key = "from"
		}
		tableID, err := field(key)
		if err != nil {
			return err
		}
		return c.requireTable(ctx, tableID, refuse)
	case method == http.MethodDelete && scopeFilePath.MatchString(path):
		return c.requireTable(ctx, scopeFilePath.FindStringSubmatch(path)[1], refuse)
	case scopeDocumentPath.MatchString(path):
		tableID, err := param("tableId")
		if err != nil {
			return err
		}
		return c.requireTable(ctx, tableID, refuse)

	// Solutions
	case method == http.MethodPost && strings.EqualFold(path, "/v1/solutions"),
		method == http.MethodGet && strings.EqualFold(path, "/v1/solutions/fromrecord"):
		return allow(s.createSolutions, "", "creating solutions isn't allowed", refuse)
	case scopeSolutionPath.MatchString(path):
		m := scopeSolutionPath.FindStringSubmatch(path)
		if strings.EqualFold(m[2], "/torecord") { // reads the solution, writes a record
			tableID, err := param("tableId")
			if err != nil {
				return err
			}
			return c.requireTable(ctx, tableID, refuse)
		}
		if !s.solutions[m[1]] {
			return refuse(m[1], "solution is outside the write scope")
		}
		return nil

	// Groups, users, credentials
	case scopeGroupPath.MatchString(path):
		return c.requireGroup(scopeGroupPath.FindStringSubmatch(path)[1], refuse)
	case scopeUsersPath.MatchString(path):
		return allow(s.manageUsers, "", "managing users isn't allowed", refuse)
	case scopeTokenPath.MatchString(path):
		return allow(s.manageTokens, "", "managing user tokens isn't allowed", refuse)
	}
	return refuse("", "this write isn't one the write scope knows")
}

// checkXMLWriteScope refuses an XML API write action outside the write scope.
// Group actions are checked by their gid, sign-out by ManageTokens, and the
// rest by their dbid, an app or a table.
func (c *Client) checkXMLWriteScope(ctx context.Context, dbid, action string, body []byte) error {
	s := c.writeScope
	if s == nil || !isXMLWriteAction(action) {
		return nil
	}
	refuse := func(target, reason string) error {
		return core.NewWriteScopeError(http.MethodPost, "/db/"+dbid, action, target, reason)
	}
	name := strings.ToLower(strings.TrimSpace(action))
	switch {
	case xmlGroupCreateActions[name]:
		return allow(s.createGroups, "", "creating groups isn't allowed", refuse)
	case xmlGroupActions[name]:
		gid, ok := xmlGroupID(body)
		if !ok {
			return refuse("", "the request's gid is missing or ambiguous")
		}
		return c.requireGroup(gid, refuse)
	case xmlTokenActions[name]:
		return allow(s.manageTokens, "", "signing out isn't allowed", refuse)
	case xmlAppCreateActions[name]:
		return allow(s.createApps, dbid, "creating apps isn't allowed", refuse)
	case xmlUserActions[name]:
		if !s.manageUsers {
			return refuse("", "managing users isn't allowed")
		}
		return c.requireApp(dbid, refuse)
	}
	if s.apps[dbid] {
		return nil
	}
	return c.requireTable(ctx, dbid, refuse)
}

func allow(ok bool, target, reason string, refuse func(target, reason string) error) error {
	if !ok {
		return refuse(target, reason)
	}
	return nil
}

func (c *Client) requireApp(appID string, refuse func(target, reason string) error) error {
	if appID == "" {
		return refuse("", "no app ID in the request")
	}
	if !c.writeScope.apps[appID] {
		return refuse(appID, "app is outside the write scope")
	}
	return nil
}

func (c *Client) requireGroup(gid string, refuse func(target, reason string) error) error {
	if gid == "" {
		return refuse("", "no group ID in the request")
	}
	if !c.writeScope.groups[gid] {
		return refuse(gid, "group is outside the write scope")
	}
	return nil
}

// requireTable allows a table in one of the scope's apps. A table it hasn't
// seen sends it to list the apps' tables again, once.
func (c *Client) requireTable(ctx context.Context, tableID string, refuse func(target, reason string) error) error {
	if tableID == "" {
		return refuse("", "no table ID in the request")
	}
	s := c.writeScope
	if s.hasTable(tableID) {
		return nil
	}
	if len(s.apps) == 0 {
		return refuse(tableID, "table is outside the write scope")
	}
	// One listing at a time; a caller that waited may find the table listed
	s.loadMu.Lock()
	defer s.loadMu.Unlock()
	if s.hasTable(tableID) {
		return nil
	}
	if err := c.loadScopeTables(ctx); err != nil {
		return refuse(tableID, "listing the write scope's tables failed: "+err.Error())
	}
	if !s.hasTable(tableID) {
		return refuse(tableID, "table is outside the write scope")
	}
	return nil
}

func (s *writeScope) hasTable(tableID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tableApp[tableID]
	return ok
}

// loadScopeTables lists the tables of the scope's apps. The caller holds s.loadMu.
func (c *Client) loadScopeTables(ctx context.Context) error {
	s := c.writeScope
	tableApp := map[string]string{}
	for appID := range s.apps {
		tables, err := c.GetAppTables(ctx, appID)
		if err != nil {
			return fmt.Errorf("app %s: %w", appID, err)
		}
		for _, t := range tables {
			if t.Id != "" {
				tableApp[t.Id] = appID
			}
		}
	}
	s.mu.Lock()
	s.tableApp = tableApp
	s.mu.Unlock()
	return nil
}

// relationshipParent returns the parent table of a table's relationship.
func (c *Client) relationshipParent(ctx context.Context, childTableID, relationshipID string) (string, error) {
	id, err := strconv.Atoi(relationshipID)
	if err != nil {
		return "", fmt.Errorf("relationship ID %q isn't a number", relationshipID)
	}
	// At most 100 pages, in case a server ignores skip or leaves out the total
	for skip, pages := 0, 0; pages < 100; pages++ {
		page, err := c.GetRelationships(ctx, childTableID, &generated.GetRelationshipsOptions{Skip: &skip})
		if err != nil {
			return "", err
		}
		for _, r := range page.Relationships {
			if r.Id == id {
				return r.ParentTableId, nil
			}
		}
		if len(page.Relationships) == 0 {
			return "", fmt.Errorf("table %s has no relationship %d", childTableID, id)
		}
		skip += len(page.Relationships)
		if page.Metadata.TotalRelationships == 0 || skip >= page.Metadata.TotalRelationships {
			return "", fmt.Errorf("table %s has no relationship %d", childTableID, id)
		}
	}
	return "", fmt.Errorf("table %s: relationship %d not found in 100 pages", childTableID, id)
}

// bodyField returns a top-level string or number in a JSON body ("" when
// absent), and false when it is ambiguous: the key appears twice, or in
// another letter case ({"to": ..., "To": ...}), or the body isn't a JSON
// object. A server might read the other one.
func bodyField(body []byte, key string) (string, bool) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", true
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if t, err := decoder.Token(); err != nil || t != json.Delim('{') {
		return "", false
	}
	value, found := "", false
	for decoder.More() {
		t, err := decoder.Token()
		if err != nil {
			return "", false
		}
		k, _ := t.(string)
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return "", false
		}
		if !strings.EqualFold(k, key) {
			continue
		}
		if found || k != key {
			return "", false
		}
		found = true
		var v any
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return "", false
		}
		switch v := v.(type) {
		case string:
			value = v
		case json.Number:
			value = v.String()
		case nil:
		default:
			return "", false
		}
	}
	return value, true
}

// queryParam returns a query parameter ("" when absent), and false when it
// is ambiguous: given twice, or also in another letter case.
func queryParam(values url.Values, key string) (string, bool) {
	value, found := "", false
	for k, vs := range values {
		if !strings.EqualFold(k, key) {
			continue
		}
		if k != key || len(vs) != 1 || found {
			return "", false
		}
		value, found = vs[0], true
	}
	return value, true
}

// xmlGroupID returns an XML body's one <gid>, and false when there isn't
// exactly one, or the body has a comment, CDATA or other markup declaration
// that could hide one from this check but not from QuickBase.
func xmlGroupID(body []byte) (string, bool) {
	if bytes.Contains(body, []byte("<!")) || bytes.Contains(body, []byte("<?")) {
		return "", false
	}
	if len(xmlGIDOpen.FindAll(body, -1)) != 1 {
		return "", false
	}
	m := xmlGIDElement.FindSubmatch(body)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}
