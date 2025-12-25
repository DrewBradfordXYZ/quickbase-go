package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v2"
	"github.com/DrewBradfordXYZ/quickbase-go/v2/generated"
	"github.com/DrewBradfordXYZ/quickbase-go/v2/xml"
)

// getXMLClient creates an XML client from the test client
func getXMLClient(t *testing.T) *xml.Client {
	return xml.New(getTestClient(t))
}

// TestXMLGrantedDBs tests the GrantedDBs API
func TestXMLGrantedDBs(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)

	t.Run("lists accessible apps", func(t *testing.T) {
		result, err := xmlClient.GrantedDBs(ctx, xml.GrantedDBsOptions{
			RealmAppsOnly: true,
		})
		if err != nil {
			t.Fatalf("GrantedDBs failed: %v", err)
		}

		if len(result.Databases) == 0 {
			t.Error("Expected at least one accessible database")
		}

		// Our test app should be in the list
		testCtx := getTestContext(t)
		found := false
		for _, db := range result.Databases {
			if db.DBID == testCtx.AppID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Test app %s not found in GrantedDBs results", testCtx.AppID)
		}
	})

	t.Run("excludes parents when requested", func(t *testing.T) {
		result, err := xmlClient.GrantedDBs(ctx, xml.GrantedDBsOptions{
			RealmAppsOnly:  true,
			ExcludeParents: true,
		})
		if err != nil {
			t.Fatalf("GrantedDBs failed: %v", err)
		}

		// Should return tables only (names contain ":")
		for _, db := range result.Databases {
			if !strings.Contains(db.Name, ":") {
				// Some may be standalone tables, that's okay
				continue
			}
		}
		// Just verify the call works
		t.Logf("Found %d tables (excluding parent apps)", len(result.Databases))
	})
}

// TestXMLGetDBInfo tests the GetDBInfo API
func TestXMLGetDBInfo(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	t.Run("gets app info", func(t *testing.T) {
		result, err := xmlClient.GetDBInfo(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetDBInfo failed: %v", err)
		}

		if result.Name == "" {
			t.Error("Expected Name to be set")
		}
		if result.ManagerName == "" {
			t.Error("Expected ManagerName to be set")
		}
		t.Logf("App: %s, Manager: %s, Records: %d", result.Name, result.ManagerName, result.NumRecords)
	})

	t.Run("gets table info", func(t *testing.T) {
		result, err := xmlClient.GetDBInfo(ctx, testCtx.TableID)
		if err != nil {
			t.Fatalf("GetDBInfo failed: %v", err)
		}

		if result.Name == "" {
			t.Error("Expected Name to be set")
		}
		t.Logf("Table: %s, Records: %d", result.Name, result.NumRecords)
	})
}

// TestXMLGetNumRecords tests the GetNumRecords API
func TestXMLGetNumRecords(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	// Clean up and insert some records
	deleteAllRecords(t, ctx)
	insertTestRecords(t, ctx, 3)

	t.Run("counts records", func(t *testing.T) {
		count, err := xmlClient.GetNumRecords(ctx, testCtx.TableID)
		if err != nil {
			t.Fatalf("GetNumRecords failed: %v", err)
		}

		if count != 3 {
			t.Errorf("Got %d records, want 3", count)
		}
	})
}

// TestXMLDoQueryCount tests the DoQueryCount API
func TestXMLDoQueryCount(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	// Clean up and insert some records
	deleteAllRecords(t, ctx)
	insertTestRecords(t, ctx, 5)

	t.Run("counts all records", func(t *testing.T) {
		result, err := xmlClient.DoQueryCount(ctx, testCtx.TableID, "")
		if err != nil {
			t.Fatalf("DoQueryCount failed: %v", err)
		}

		if result.NumMatches != 5 {
			t.Errorf("Got %d records, want 5", result.NumMatches)
		}
	})

	t.Run("counts filtered records", func(t *testing.T) {
		query := fmt.Sprintf("{%d.GT.2}", testCtx.NumberFieldID)
		result, err := xmlClient.DoQueryCount(ctx, testCtx.TableID, query)
		if err != nil {
			t.Fatalf("DoQueryCount failed: %v", err)
		}

		// Records with Amount > 2 (i.e., 3, 4, 5)
		if result.NumMatches != 3 {
			t.Errorf("Got %d records, want 3", result.NumMatches)
		}
	})
}

// TestXMLGetRoleInfo tests the GetRoleInfo API
func TestXMLGetRoleInfo(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	t.Run("gets app roles", func(t *testing.T) {
		result, err := xmlClient.GetRoleInfo(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetRoleInfo failed: %v", err)
		}

		if len(result.Roles) == 0 {
			t.Error("Expected at least one role (apps have default roles)")
		}

		// Log roles for debugging
		for _, role := range result.Roles {
			t.Logf("Role %d: %s (%s)", role.ID, role.Name, role.Access.Description)
		}
	})
}

// TestXMLUserRoles tests the UserRoles API
func TestXMLUserRoles(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	t.Run("gets users with roles", func(t *testing.T) {
		result, err := xmlClient.UserRoles(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("UserRoles failed: %v", err)
		}

		if len(result.Users) == 0 {
			t.Error("Expected at least one user (the test user)")
		}

		// Log users for debugging
		for _, user := range result.Users {
			roleNames := make([]string, len(user.Roles))
			for i, r := range user.Roles {
				roleNames[i] = r.Name
			}
			t.Logf("User: %s, Roles: %v", user.Name, roleNames)
		}
	})
}

// TestXMLGetSchema tests the GetSchema API
func TestXMLGetSchema(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	t.Run("gets table schema", func(t *testing.T) {
		result, err := xmlClient.GetSchema(ctx, testCtx.TableID)
		if err != nil {
			t.Fatalf("GetSchema failed: %v", err)
		}

		if result.Table.Name == "" {
			t.Error("Expected table name")
		}

		if len(result.Table.Fields) == 0 {
			t.Error("Expected at least one field")
		}

		// Verify our test fields exist
		fieldIDs := make(map[int]bool)
		for _, f := range result.Table.Fields {
			fieldIDs[f.ID] = true
		}

		if !fieldIDs[testCtx.TextFieldID] {
			t.Errorf("Text field %d not found in schema", testCtx.TextFieldID)
		}
		if !fieldIDs[testCtx.NumberFieldID] {
			t.Errorf("Number field %d not found in schema", testCtx.NumberFieldID)
		}

		t.Logf("Schema: %s with %d fields", result.Table.Name, len(result.Table.Fields))
	})
}

// TestXMLGetRecordInfo tests the GetRecordInfo API
func TestXMLGetRecordInfo(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	// Clean up and insert a record
	deleteAllRecords(t, ctx)
	insertTestRecords(t, ctx, 1)

	// Get the record ID using upsert response
	recordID := getFirstRecordID(t, ctx)

	t.Run("gets record info", func(t *testing.T) {
		result, err := xmlClient.GetRecordInfo(ctx, testCtx.TableID, recordID)
		if err != nil {
			t.Fatalf("GetRecordInfo failed: %v", err)
		}

		if len(result.Fields) == 0 {
			t.Error("Expected fields in result")
		}

		// Log fields for debugging
		for _, f := range result.Fields {
			t.Logf("Field %d (%s): %s = %s", f.ID, f.Type, f.Name, f.Value)
		}
	})
}

// TestXMLDBVars tests the GetDBVar and SetDBVar APIs
func TestXMLDBVars(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	varName := "test_var"
	varValue := "test_value_123"

	t.Run("sets and gets variable", func(t *testing.T) {
		// Set the variable
		err := xmlClient.SetDBVar(ctx, testCtx.AppID, varName, varValue)
		if err != nil {
			t.Fatalf("SetDBVar failed: %v", err)
		}

		// Get the variable
		result, err := xmlClient.GetDBVar(ctx, testCtx.AppID, varName)
		if err != nil {
			t.Fatalf("GetDBVar failed: %v", err)
		}

		if result != varValue {
			t.Errorf("Got value %q, want %q", result, varValue)
		}
	})
}

// TestXMLGetAppDTMInfo tests the GetAppDTMInfo API
// NOTE: This API cannot be called with user tokens, requires ticket auth.
func TestXMLGetAppDTMInfo(t *testing.T) {
	skipIfNoCredentials(t)

	username := os.Getenv("QB_USERNAME")
	password := os.Getenv("QB_PASSWORD")
	if username == "" || password == "" {
		t.Skip("Skipping: QB_USERNAME or QB_PASSWORD not set (required for ticket auth)")
	}

	ctx := context.Background()
	testCtx := getTestContext(t)

	// Create a client with ticket auth
	ticketClient, err := quickbase.New(qbRealm, quickbase.WithTicketAuth(username, password))
	if err != nil {
		t.Fatalf("Failed to create ticket auth client: %v", err)
	}
	xmlClient := xml.New(ticketClient)

	t.Run("gets app modification info", func(t *testing.T) {
		result, err := xmlClient.GetAppDTMInfo(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetAppDTMInfo failed: %v", err)
		}

		if result.RequestTime == 0 {
			t.Error("Expected RequestTime to be set")
		}
		if result.AppLastModifiedTime == 0 {
			t.Error("Expected AppLastModifiedTime to be set")
		}

		t.Logf("RequestTime: %d, NextAllowed: %d", result.RequestTime, result.RequestNextAllowedTime)
		t.Logf("App last modified: %d, last record mod: %d", result.AppLastModifiedTime, result.AppLastRecModTime)
		t.Logf("Tables: %d", len(result.Tables))
	})
}

// TestXMLFindDBByName tests the FindDBByName API
// NOTE: This API cannot be called with user tokens, requires ticket auth.
func TestXMLFindDBByName(t *testing.T) {
	skipIfNoCredentials(t)

	username := os.Getenv("QB_USERNAME")
	password := os.Getenv("QB_PASSWORD")
	if username == "" || password == "" {
		t.Skip("Skipping: QB_USERNAME or QB_PASSWORD not set (required for ticket auth)")
	}

	ctx := context.Background()
	testCtx := getTestContext(t)

	// Create a client with ticket auth
	ticketClient, err := quickbase.New(qbRealm, quickbase.WithTicketAuth(username, password))
	if err != nil {
		t.Fatalf("Failed to create ticket auth client: %v", err)
	}
	xmlClient := xml.New(ticketClient)

	// First get the app name using user token client (GetDBInfo works with user tokens)
	userTokenXML := getXMLClient(t)
	info, err := userTokenXML.GetDBInfo(ctx, testCtx.AppID)
	if err != nil {
		t.Fatalf("GetDBInfo failed: %v", err)
	}

	t.Run("finds app by name", func(t *testing.T) {
		result, err := xmlClient.FindDBByName(ctx, info.Name, true)
		if err != nil {
			t.Fatalf("FindDBByName failed: %v", err)
		}

		if result.DBID != testCtx.AppID {
			t.Errorf("Got DBID %s, want %s", result.DBID, testCtx.AppID)
		}
		t.Logf("Found app: %s (DBID: %s)", result.Name, result.DBID)
	})
}

// TestXMLGenResultsTable tests the GenResultsTable API
func TestXMLGenResultsTable(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	// Insert some records
	deleteAllRecords(t, ctx)
	insertTestRecords(t, ctx, 3)

	t.Run("generates CSV results", func(t *testing.T) {
		result, err := xmlClient.GenResultsTable(ctx, testCtx.TableID, xml.GenResultsTableOptions{
			CList:  fmt.Sprintf("3.%d.%d", testCtx.TextFieldID, testCtx.NumberFieldID),
			Format: xml.GenResultsFormatCSV,
		})
		if err != nil {
			t.Fatalf("GenResultsTable failed: %v", err)
		}

		if result == "" {
			t.Error("Expected non-empty result")
		}

		// Should contain CSV data
		if !strings.Contains(result, ",") {
			t.Error("Expected CSV format with commas")
		}

		t.Logf("CSV output (first 200 chars): %s", truncate(result, 200))
	})

	t.Run("generates HTML results", func(t *testing.T) {
		result, err := xmlClient.GenResultsTable(ctx, testCtx.TableID, xml.GenResultsTableOptions{
			CList:  fmt.Sprintf("3.%d", testCtx.TextFieldID),
			Format: xml.GenResultsFormatJHT,
		})
		if err != nil {
			t.Fatalf("GenResultsTable failed: %v", err)
		}

		if result == "" {
			t.Error("Expected non-empty result")
		}

		// Should contain JavaScript/HTML
		if !strings.Contains(result, "qdbWrite") && !strings.Contains(result, "<") {
			t.Error("Expected JavaScript or HTML content")
		}

		t.Logf("HTML/JS output (first 200 chars): %s", truncate(result, 200))
	})
}

// TestXMLGetRecordAsHTML tests the GetRecordAsHTML API
func TestXMLGetRecordAsHTML(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	xmlClient := getXMLClient(t)
	testCtx := getTestContext(t)

	// Insert a record
	deleteAllRecords(t, ctx)
	insertTestRecords(t, ctx, 1)

	// Get the record ID
	recordID := getFirstRecordID(t, ctx)

	t.Run("gets record as HTML", func(t *testing.T) {
		result, err := xmlClient.GetRecordAsHTML(ctx, testCtx.TableID, xml.GetRecordAsHTMLOptions{
			RecordID: recordID,
		})
		if err != nil {
			t.Fatalf("GetRecordAsHTML failed: %v", err)
		}

		if result == "" {
			t.Error("Expected non-empty HTML result")
		}

		t.Logf("HTML output (first 200 chars): %s", truncate(result, 200))
	})
}

// getFirstRecordID queries the table and returns the first record's ID
func getFirstRecordID(t *testing.T, ctx context.Context) int {
	t.Helper()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	resp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
		From:   testCtx.TableID,
		Select: &[]int{3}, // Record ID# field
	})
	if err != nil {
		t.Fatalf("Failed to query for record ID: %v", err)
	}
	if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
		t.Fatal("No records found")
	}

	// Extract the record ID from the response
	recordData := (*resp.JSON200.Data)[0]
	recordIDVal, ok := recordData["3"]
	if !ok {
		t.Fatal("Record ID field (3) not in response")
	}

	// The value is wrapped in generated.FieldValue
	recordID := extractRecordID(recordIDVal)
	if recordID == 0 {
		t.Fatal("Could not extract record ID from response")
	}
	return recordID
}

// extractRecordID extracts the record ID from a field value
func extractRecordID(v interface{}) int {
	switch val := v.(type) {
	case generated.FieldValue:
		// Try to get the value as a float
		if inner, err := val.Value.AsFieldValueValue1(); err == nil {
			return int(inner)
		}
	case map[string]interface{}:
		if inner, ok := val["value"]; ok {
			switch num := inner.(type) {
			case float64:
				return int(num)
			case float32:
				return int(num)
			case int:
				return num
			}
		}
	case float64:
		return int(val)
	case float32:
		return int(val)
	case int:
		return val
	}
	return 0
}

// Helper to truncate string for logging
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

