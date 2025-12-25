package client

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v2/core"
)

func TestExtractDBID(t *testing.T) {
	tests := []struct {
		name     string
		req      *http.Request
		expected string
	}{
		// Query params - tableId
		{
			name: "tableId from query params",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/fields?tableId=bqtable123"),
			},
			expected: "bqtable123",
		},
		// Query params - appId
		{
			name: "appId from query params",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/apps?appId=bqapp123"),
			},
			expected: "bqapp123",
		},
		// Path - tableId
		{
			name: "tableId from path /tables/{tableId}",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/tables/bqpath456"),
			},
			expected: "bqpath456",
		},
		{
			name: "tableId from nested path /tables/{tableId}/records",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/tables/bqnested789/records"),
			},
			expected: "bqnested789",
		},
		// Path - appId
		{
			name: "appId from path /apps/{appId}",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/apps/bqapp456"),
			},
			expected: "bqapp456",
		},
		{
			name: "appId from nested path /apps/{appId}/tables",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/apps/bqapp789/tables"),
			},
			expected: "bqapp789",
		},
		// Priority: query params over path
		{
			name: "query params preferred over path",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/tables/path123?tableId=query456"),
			},
			expected: "query456",
		},
		// Priority: tableId over appId
		{
			name: "tableId preferred over appId in query",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/fields?tableId=table123&appId=app456"),
			},
			expected: "table123",
		},
		// No dbid found
		{
			name: "no dbid in request",
			req: &http.Request{
				URL: mustParseURL("https://api.quickbase.com/v1/users"),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDBID(tt.req)
			if result != tt.expected {
				t.Errorf("extractDBID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractDBIDFromBody(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "extracts from body.from (runQuery)",
			body:     []byte(`{"from": "bqfrom123", "select": [3, 6, 7]}`),
			expected: "bqfrom123",
		},
		{
			name:     "extracts from body.from (deleteRecords)",
			body:     []byte(`{"from": "bqdelete456", "where": "{3.GT.0}"}`),
			expected: "bqdelete456",
		},
		{
			name:     "extracts from body.to (upsert)",
			body:     []byte(`{"to": "bqto789", "data": []}`),
			expected: "bqto789",
		},
		{
			name:     "prefers from over to",
			body:     []byte(`{"from": "bqfrom111", "to": "bqto222"}`),
			expected: "bqfrom111",
		},
		{
			name:     "empty body",
			body:     []byte(``),
			expected: "",
		},
		{
			name:     "empty JSON object",
			body:     []byte(`{}`),
			expected: "",
		},
		{
			name:     "invalid JSON",
			body:     []byte(`not json`),
			expected: "",
		},
		{
			name:     "from is not a string",
			body:     []byte(`{"from": 12345}`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDBIDFromBody(tt.body)
			if result != tt.expected {
				t.Errorf("extractDBIDFromBody() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateRealm(t *testing.T) {
	tests := []struct {
		name      string
		realm     string
		expectErr bool
	}{
		{
			name:      "valid realm",
			realm:     "mycompany",
			expectErr: false,
		},
		{
			name:      "empty realm",
			realm:     "",
			expectErr: true,
		},
		{
			name:      "realm with dot",
			realm:     "mycompany.quickbase.com",
			expectErr: true,
		},
		{
			name:      "realm with subdomain dot",
			realm:     "my.company",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRealm(tt.realm)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateRealm(%q) expected error, got nil", tt.realm)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateRealm(%q) unexpected error: %v", tt.realm, err)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	// Create a mock authHTTPClient to test backoff calculation
	client := &Client{
		initialDelay: 100 * time.Millisecond,
		maxDelay:     1000 * time.Millisecond,
		backoffMult:  2,
	}
	h := &authHTTPClient{client: client}

	t.Run("first attempt delay around initialDelay", func(t *testing.T) {
		delay := h.calculateBackoff(1)
		// 100ms ± 10% jitter = 90-110ms
		if delay < 90*time.Millisecond || delay > 110*time.Millisecond {
			t.Errorf("calculateBackoff(1) = %v, want 90-110ms", delay)
		}
	})

	t.Run("second attempt doubles delay", func(t *testing.T) {
		delay := h.calculateBackoff(2)
		// 100 * 2^1 = 200ms ± 10% = 180-220ms
		if delay < 180*time.Millisecond || delay > 220*time.Millisecond {
			t.Errorf("calculateBackoff(2) = %v, want 180-220ms", delay)
		}
	})

	t.Run("third attempt quadruples delay", func(t *testing.T) {
		delay := h.calculateBackoff(3)
		// 100 * 2^2 = 400ms ± 10% = 360-440ms
		if delay < 360*time.Millisecond || delay > 440*time.Millisecond {
			t.Errorf("calculateBackoff(3) = %v, want 360-440ms", delay)
		}
	})

	t.Run("caps at maxDelay", func(t *testing.T) {
		delay := h.calculateBackoff(10)
		// Should be capped at 1000ms + jitter max
		if delay > 1100*time.Millisecond {
			t.Errorf("calculateBackoff(10) = %v, should be capped at ~1100ms", delay)
		}
	})

	t.Run("respects different multiplier", func(t *testing.T) {
		client := &Client{
			initialDelay: 100 * time.Millisecond,
			maxDelay:     10 * time.Second,
			backoffMult:  3,
		}
		h := &authHTTPClient{client: client}

		delay := h.calculateBackoff(2)
		// 100 * 3^1 = 300ms ± 10% = 270-330ms
		if delay < 270*time.Millisecond || delay > 330*time.Millisecond {
			t.Errorf("calculateBackoff(2) with mult=3 = %v, want 270-330ms", delay)
		}
	})
}

func TestBackoffJitter(t *testing.T) {
	// Run multiple times to verify jitter introduces variation
	client := &Client{
		initialDelay: 1 * time.Second,
		maxDelay:     30 * time.Second,
		backoffMult:  2,
	}
	h := &authHTTPClient{client: client}

	// Expected base delay for attempt 1: 1s
	baseDelay := 1 * time.Second
	minDelay := time.Duration(float64(baseDelay) * 0.9) // -10%
	maxDelay := time.Duration(float64(baseDelay) * 1.1) // +10%

	seenValues := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		delay := h.calculateBackoff(1)

		if delay < minDelay || delay > maxDelay {
			t.Errorf("calculateBackoff(1) = %v, want between %v and %v", delay, minDelay, maxDelay)
		}

		seenValues[delay] = true
	}

	// With jitter, we should see multiple different values
	if len(seenValues) < 5 {
		t.Errorf("Expected jitter to produce variation, got only %d unique values", len(seenValues))
	}
}

// Helper function
func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

// Test exponential growth
func TestExponentialBackoff(t *testing.T) {
	client := &Client{
		initialDelay: 100 * time.Millisecond,
		maxDelay:     10 * time.Second,
		backoffMult:  2,
	}
	h := &authHTTPClient{client: client}

	expectedDelays := []time.Duration{
		100 * time.Millisecond,  // 100 * 2^0
		200 * time.Millisecond,  // 100 * 2^1
		400 * time.Millisecond,  // 100 * 2^2
		800 * time.Millisecond,  // 100 * 2^3
		1600 * time.Millisecond, // 100 * 2^4
	}

	for attempt, expected := range expectedDelays {
		delay := h.calculateBackoff(attempt + 1)
		// Allow ±10% for jitter
		minExpected := time.Duration(float64(expected) * 0.9)
		maxExpected := time.Duration(float64(expected) * 1.1)

		if delay < minExpected || delay > maxExpected {
			t.Errorf("calculateBackoff(%d) = %v, want between %v and %v",
				attempt+1, delay, minExpected, maxExpected)
		}
	}
}

// Verify the exponential formula
func TestExponentialFormula(t *testing.T) {
	// delay = initialDelay * multiplier^(attempt-1)
	initialDelay := 100.0
	multiplier := 2.0

	for attempt := 1; attempt <= 5; attempt++ {
		expected := initialDelay * math.Pow(multiplier, float64(attempt-1))
		t.Logf("Attempt %d: expected base delay = %.0fms", attempt, expected)
	}
}

// mockSignOuter is a mock auth strategy that implements SignOuter
type mockSignOuter struct {
	signOutCalled bool
}

func (m *mockSignOuter) GetToken(_ context.Context, _ string) (string, error) {
	return "mock-token", nil
}

func (m *mockSignOuter) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-TICKET "+token)
}

func (m *mockSignOuter) HandleAuthError(_ context.Context, _ int, _ string, _ int, _ int) (string, error) {
	return "", nil
}

func (m *mockSignOuter) SignOut() {
	m.signOutCalled = true
}

// mockNoSignOut is a mock auth strategy that does NOT implement SignOuter
type mockNoSignOut struct{}

func (m *mockNoSignOut) GetToken(_ context.Context, _ string) (string, error) {
	return "mock-token", nil
}

func (m *mockNoSignOut) ApplyAuth(req *http.Request, token string) {
	req.Header.Set("Authorization", "QB-USER-TOKEN "+token)
}

func (m *mockNoSignOut) HandleAuthError(_ context.Context, _ int, _ string, _ int, _ int) (string, error) {
	return "", nil
}

func TestClient_SignOut_WithSignOuter(t *testing.T) {
	mock := &mockSignOuter{}
	client, err := New("testrealm", mock)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result := client.SignOut()

	if !result {
		t.Error("SignOut() returned false, expected true for SignOuter strategy")
	}
	if !mock.signOutCalled {
		t.Error("SignOut() did not call strategy.SignOut()")
	}
}

func TestClient_SignOut_WithoutSignOuter(t *testing.T) {
	mock := &mockNoSignOut{}
	client, err := New("testrealm", mock)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result := client.SignOut()

	if result {
		t.Error("SignOut() returned true, expected false for non-SignOuter strategy")
	}
}

func TestIsWriteMethod(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"PATCH", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := isWriteMethod(tt.method)
			if result != tt.expected {
				t.Errorf("isWriteMethod(%q) = %v, want %v", tt.method, result, tt.expected)
			}
		})
	}
}

func TestIsXMLWriteAction(t *testing.T) {
	// Write actions should return true
	writeActions := []string{
		"API_AddUserToRole",
		"API_RemoveUserFromRole",
		"API_ChangeUserRole",
		"API_ProvisionUser",
		"API_SendInvitation",
		"API_ChangeManager",
		"API_ChangeRecordOwner",
		"API_CreateGroup",
		"API_DeleteGroup",
		"API_AddUserToGroup",
		"API_RemoveUserFromGroup",
		"API_AddGroupToRole",
		"API_RemoveGroupFromRole",
		"API_CopyGroup",
		"API_ChangeGroupInfo",
		"API_AddSubGroup",
		"API_RemoveSubGroup",
		"API_SetDBVar",
		"API_AddReplaceDBPage",
		"API_FieldAddChoices",
		"API_FieldRemoveChoices",
		"API_SetKeyField",
		"API_Webhooks_Create",
		"API_Webhooks_Edit",
		"API_Webhooks_Delete",
		"API_Webhooks_Activate",
		"API_Webhooks_Deactivate",
		"API_Webhooks_Copy",
		"API_ImportFromCSV",
		"API_RunImport",
		"API_CopyMasterDetail",
		"API_PurgeRecords",
		"API_AddRecord",
		"API_EditRecord",
		"API_DeleteRecord",
		"API_SignOut",
	}

	for _, action := range writeActions {
		t.Run(action+" is write", func(t *testing.T) {
			if !isXMLWriteAction(action) {
				t.Errorf("isXMLWriteAction(%q) = false, want true", action)
			}
		})
	}

	// Read actions should return false
	readActions := []string{
		"API_GetRoleInfo",
		"API_GetUserRole",
		"API_GetSchema",
		"API_GrantedDBs",
		"API_GetDBInfo",
		"API_GetNumRecords",
		"API_DoQueryCount",
		"API_DoQuery",
		"API_GenResultsTable",
		"API_GetRecordInfo",
		"API_GetRecordAsHTML",
		"API_GetUserInfo",
		"API_GetDBVar",
		"API_GetAppDTMInfo",
		"API_FindDBByName",
		"API_Authenticate",
	}

	for _, action := range readActions {
		t.Run(action+" is read", func(t *testing.T) {
			if isXMLWriteAction(action) {
				t.Errorf("isXMLWriteAction(%q) = true, want false", action)
			}
		})
	}
}

func TestCheckReadOnly_Disabled(t *testing.T) {
	client := &Client{readOnly: false}

	// All methods should be allowed when read-only is disabled
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		req := &http.Request{
			Method: method,
			URL:    mustParseURL("https://api.quickbase.com/v1/records"),
		}
		if err := client.checkReadOnly(req); err != nil {
			t.Errorf("checkReadOnly() with readOnly=false returned error for %s: %v", method, err)
		}
	}
}

func TestCheckReadOnly_BlocksJSONWrites(t *testing.T) {
	client := &Client{readOnly: true}

	// Write methods should be blocked
	writeTests := []struct {
		method string
		path   string
	}{
		{"POST", "/v1/records"},
		{"PUT", "/v1/apps/bqxyz123"},
		{"DELETE", "/v1/tables/bqtable/records"},
		{"PATCH", "/v1/fields/6"},
	}

	for _, tt := range writeTests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := &http.Request{
				Method: tt.method,
				URL:    mustParseURL("https://api.quickbase.com" + tt.path),
				Header: make(http.Header),
			}
			err := client.checkReadOnly(req)
			if err == nil {
				t.Errorf("checkReadOnly() returned nil, expected ReadOnlyError for %s %s", tt.method, tt.path)
				return
			}

			// Verify it's a ReadOnlyError with correct fields
			roErr, ok := err.(*core.ReadOnlyError)
			if !ok {
				t.Errorf("checkReadOnly() returned %T, expected *core.ReadOnlyError", err)
				return
			}
			if roErr.Method != tt.method {
				t.Errorf("ReadOnlyError.Method = %q, want %q", roErr.Method, tt.method)
			}
			if roErr.Path != tt.path {
				t.Errorf("ReadOnlyError.Path = %q, want %q", roErr.Path, tt.path)
			}
			if roErr.Action != "" {
				t.Errorf("ReadOnlyError.Action = %q, want empty for JSON API", roErr.Action)
			}
		})
	}
}

func TestCheckReadOnly_AllowsJSONReads(t *testing.T) {
	client := &Client{readOnly: true}

	// Read methods should be allowed
	readTests := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/apps/bqxyz123"},
		{"GET", "/v1/tables/bqtable/fields"},
		{"GET", "/v1/reports/123"},
		{"HEAD", "/v1/files/bqtable/1/6/0"},
	}

	for _, tt := range readTests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := &http.Request{
				Method: tt.method,
				URL:    mustParseURL("https://api.quickbase.com" + tt.path),
			}
			if err := client.checkReadOnly(req); err != nil {
				t.Errorf("checkReadOnly() returned error for read %s %s: %v", tt.method, tt.path, err)
			}
		})
	}
}

func TestCheckReadOnly_XMLWriteActions(t *testing.T) {
	client := &Client{readOnly: true}

	// XML API write actions should be blocked
	writeActions := []string{
		"API_AddUserToRole",
		"API_SetDBVar",
		"API_ImportFromCSV",
		"API_SignOut",
	}

	for _, action := range writeActions {
		t.Run(action, func(t *testing.T) {
			req := &http.Request{
				Method: http.MethodPost,
				URL:    mustParseURL("https://myrealm.quickbase.com/db/bqxyz123"),
				Header: make(http.Header),
			}
			req.Header.Set("QUICKBASE-ACTION", action)

			err := client.checkReadOnly(req)
			if err == nil {
				t.Errorf("checkReadOnly() returned nil, expected ReadOnlyError for XML action %s", action)
				return
			}

			roErr, ok := err.(*core.ReadOnlyError)
			if !ok {
				t.Errorf("checkReadOnly() returned %T, expected *core.ReadOnlyError", err)
				return
			}
			if roErr.Action != action {
				t.Errorf("ReadOnlyError.Action = %q, want %q", roErr.Action, action)
			}
		})
	}
}

func TestCheckReadOnly_XMLReadActions(t *testing.T) {
	client := &Client{readOnly: true}

	// XML API read actions should be allowed
	readActions := []string{
		"API_GetRoleInfo",
		"API_GetSchema",
		"API_DoQueryCount",
		"API_GetRecordInfo",
		"API_GetUserInfo",
		"API_GrantedDBs",
		"API_GetDBInfo",
		"API_GetNumRecords",
	}

	for _, action := range readActions {
		t.Run(action, func(t *testing.T) {
			req := &http.Request{
				Method: http.MethodPost,
				URL:    mustParseURL("https://myrealm.quickbase.com/db/bqxyz123"),
				Header: make(http.Header),
			}
			req.Header.Set("QUICKBASE-ACTION", action)

			if err := client.checkReadOnly(req); err != nil {
				t.Errorf("checkReadOnly() returned error for read XML action %s: %v", action, err)
			}
		})
	}
}

func TestReadOnlyError_Format(t *testing.T) {
	t.Run("JSON API error message", func(t *testing.T) {
		err := core.NewReadOnlyError("POST", "/v1/records", "")
		msg := err.Error()
		expected := "read-only mode: write operation blocked (POST /v1/records)"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("XML API error message", func(t *testing.T) {
		err := core.NewReadOnlyError("POST", "/db/bqxyz123", "API_AddUserToRole")
		msg := err.Error()
		expected := "read-only mode: write operation blocked (XML action: API_AddUserToRole)"
		if msg != expected {
			t.Errorf("Error() = %q, want %q", msg, expected)
		}
	})
}

func TestWithReadOnly_Option(t *testing.T) {
	mock := &mockNoSignOut{}

	// Without WithReadOnly
	client1, err := New("testrealm", mock)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if client1.readOnly {
		t.Error("Client without WithReadOnly() should have readOnly=false")
	}

	// With WithReadOnly
	client2, err := New("testrealm", mock, WithReadOnly())
	if err != nil {
		t.Fatalf("New() with WithReadOnly() error: %v", err)
	}
	if !client2.readOnly {
		t.Error("Client with WithReadOnly() should have readOnly=true")
	}
}

func TestIsJSONWriteEndpoint(t *testing.T) {
	// Write endpoints that should be blocked
	writeEndpoints := []struct {
		method string
		path   string
	}{
		// Exact matches
		{"POST", "/v1/records"},
		{"DELETE", "/v1/records"},
		{"POST", "/v1/apps"},
		{"POST", "/v1/tables"},
		{"POST", "/v1/fields"},
		{"DELETE", "/v1/fields"},
		{"POST", "/v1/usertoken"},
		{"DELETE", "/v1/usertoken"},
		{"POST", "/v1/solutions"},

		// Prefix matches (with IDs)
		{"POST", "/v1/apps/abc123"},
		{"POST", "/v1/apps/abc123/copy"},
		{"DELETE", "/v1/apps/abc123"},
		{"POST", "/v1/tables/abc123"},
		{"DELETE", "/v1/tables/abc123"},
		{"POST", "/v1/tables/abc123/relationship"},
		{"POST", "/v1/fields/6"},
		{"DELETE", "/v1/files/abc/1/6/0"},
		{"PUT", "/v1/users/deny"},
		{"PUT", "/v1/users/deny/true"},
		{"POST", "/v1/groups/123/members"},
		{"DELETE", "/v1/groups/123/managers"},
		{"PUT", "/v1/solutions/abc123"},

		// GET endpoints that write (non-RESTful)
		{"GET", "/v1/docTemplates/123/generate"},
		{"GET", "/v1/solutions/fromrecord"},
	}

	for _, tt := range writeEndpoints {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			if !isJSONWriteEndpoint(tt.method, tt.path) {
				t.Errorf("isJSONWriteEndpoint(%q, %q) = false, want true", tt.method, tt.path)
			}
		})
	}

	// Read endpoints that should be allowed
	readEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/apps/abc123"},
		{"GET", "/v1/tables/abc123"},
		{"GET", "/v1/fields"},
		{"GET", "/v1/reports/123"},
		{"GET", "/v1/solutions/abc123"}, // Export solution (read-only)
		{"GET", "/v1/auth/temporary/abc123"},
		{"GET", "/v1/files/abc/1/6/0"}, // Download file
		{"GET", "/v1/fields/usage/6"},
	}

	for _, tt := range readEndpoints {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			if isJSONWriteEndpoint(tt.method, tt.path) {
				t.Errorf("isJSONWriteEndpoint(%q, %q) = true, want false", tt.method, tt.path)
			}
		})
	}
}

func TestIsJSONReadOnlyPOSTEndpoint(t *testing.T) {
	// POST endpoints that are actually read-only
	readOnlyPOSTs := []string{
		"/v1/records/query",
		"/v1/reports/123/run",
		"/v1/formula/run",
		"/v1/audit",
		"/v1/users",
		"/v1/analytics/events/summaries",
	}

	for _, path := range readOnlyPOSTs {
		t.Run("read-only: "+path, func(t *testing.T) {
			if !isJSONReadOnlyPOSTEndpoint(path) {
				t.Errorf("isJSONReadOnlyPOSTEndpoint(%q) = false, want true", path)
			}
		})
	}

	// POST endpoints that are writes
	writePOSTs := []string{
		"/v1/records",
		"/v1/apps",
		"/v1/tables",
		"/v1/fields",
	}

	for _, path := range writePOSTs {
		t.Run("write: "+path, func(t *testing.T) {
			if isJSONReadOnlyPOSTEndpoint(path) {
				t.Errorf("isJSONReadOnlyPOSTEndpoint(%q) = true, want false", path)
			}
		})
	}
}

func TestCheckReadOnly_BlocksWriteGETs(t *testing.T) {
	client := &Client{readOnly: true}

	// GET requests that perform write operations should be blocked
	writeGETs := []string{
		"/v1/docTemplates/123/generate",
		"/v1/solutions/fromrecord",
	}

	for _, path := range writeGETs {
		t.Run(path, func(t *testing.T) {
			req := &http.Request{
				Method: http.MethodGet,
				URL:    mustParseURL("https://api.quickbase.com" + path),
				Header: make(http.Header),
			}
			err := client.checkReadOnly(req)
			if err == nil {
				t.Errorf("checkReadOnly() returned nil, expected ReadOnlyError for GET %s", path)
				return
			}

			roErr, ok := err.(*core.ReadOnlyError)
			if !ok {
				t.Errorf("checkReadOnly() returned %T, expected *core.ReadOnlyError", err)
				return
			}
			if roErr.Method != http.MethodGet {
				t.Errorf("ReadOnlyError.Method = %q, want GET", roErr.Method)
			}
		})
	}
}

func TestCheckReadOnly_AllowsReadGETs(t *testing.T) {
	client := &Client{readOnly: true}

	// Regular GET requests should be allowed
	readGETs := []string{
		"/v1/apps/abc123",
		"/v1/solutions/abc123", // Export solution is read-only
		"/v1/fields",
		"/v1/reports/123",
	}

	for _, path := range readGETs {
		t.Run(path, func(t *testing.T) {
			req := &http.Request{
				Method: http.MethodGet,
				URL:    mustParseURL("https://api.quickbase.com" + path),
				Header: make(http.Header),
			}
			if err := client.checkReadOnly(req); err != nil {
				t.Errorf("checkReadOnly() returned error for GET %s: %v", path, err)
			}
		})
	}
}

func TestCheckReadOnly_AllowsReadOnlyPOSTs(t *testing.T) {
	client := &Client{readOnly: true}

	// POST endpoints that are read-only should be allowed
	readOnlyPOSTs := []string{
		"/v1/records/query",
		"/v1/reports/123/run",
		"/v1/formula/run",
		"/v1/audit",
		"/v1/users",
	}

	for _, path := range readOnlyPOSTs {
		t.Run(path, func(t *testing.T) {
			req := &http.Request{
				Method: http.MethodPost,
				URL:    mustParseURL("https://api.quickbase.com" + path),
				Header: make(http.Header),
			}
			if err := client.checkReadOnly(req); err != nil {
				t.Errorf("checkReadOnly() returned error for read-only POST %s: %v", path, err)
			}
		})
	}
}

func TestCheckReadOnly_BlocksWritePOSTs(t *testing.T) {
	client := &Client{readOnly: true}

	// POST endpoints that write should be blocked
	writePOSTs := []string{
		"/v1/records",
		"/v1/apps",
		"/v1/apps/abc123",
		"/v1/tables",
		"/v1/fields",
	}

	for _, path := range writePOSTs {
		t.Run(path, func(t *testing.T) {
			req := &http.Request{
				Method: http.MethodPost,
				URL:    mustParseURL("https://api.quickbase.com" + path),
				Header: make(http.Header),
			}
			err := client.checkReadOnly(req)
			if err == nil {
				t.Errorf("checkReadOnly() returned nil, expected ReadOnlyError for POST %s", path)
			}
		})
	}
}

func TestInjectAppToken(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		token    string
		expected string
	}{
		{
			name:     "injects token after opening tag",
			body:     "<qdbapi><query>{}</query></qdbapi>",
			token:    "abc123",
			expected: "<qdbapi><apptoken>abc123</apptoken><query>{}</query></qdbapi>",
		},
		{
			name:     "handles empty body",
			body:     "<qdbapi></qdbapi>",
			token:    "token",
			expected: "<qdbapi><apptoken>token</apptoken></qdbapi>",
		},
		{
			name:     "returns unchanged if no qdbapi tag",
			body:     "<other>content</other>",
			token:    "token",
			expected: "<other>content</other>",
		},
		{
			name:     "handles complex body",
			body:     "<qdbapi><udata>test</udata><rid>123</rid></qdbapi>",
			token:    "mytoken",
			expected: "<qdbapi><apptoken>mytoken</apptoken><udata>test</udata><rid>123</rid></qdbapi>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectAppToken([]byte(tt.body), tt.token)
			if string(result) != tt.expected {
				t.Errorf("injectAppToken() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestWithAppToken_Option(t *testing.T) {
	mock := &mockNoSignOut{}

	// Without WithAppToken
	client1, err := New("testrealm", mock)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if client1.appToken != "" {
		t.Error("Client without WithAppToken() should have empty appToken")
	}

	// With WithAppToken
	client2, err := New("testrealm", mock, WithAppToken("test-token"))
	if err != nil {
		t.Fatalf("New() with WithAppToken() error: %v", err)
	}
	if client2.appToken != "test-token" {
		t.Errorf("Client with WithAppToken() should have appToken=%q, got %q", "test-token", client2.appToken)
	}
}
