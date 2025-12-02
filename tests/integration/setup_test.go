// Package integration provides integration tests for the QuickBase Go SDK.
//
// These tests run against a real QuickBase instance and require credentials.
// Set QB_REALM and QB_USER_TOKEN environment variables to run these tests.
//
// The tests create an ephemeral app for each test run and clean it up afterward.
//
// Run with: go test -v ./tests/integration/...
// Skip with: go test -short ./...
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

// Environment variables for credentials
const (
	envRealm     = "QB_REALM"
	envUserToken = "QB_USER_TOKEN"
)

// TestContext holds shared test context
type TestContext struct {
	AppID           string `json:"appId"`
	TableID         string `json:"tableId"`
	TextFieldID     int    `json:"textFieldId"`
	NumberFieldID   int    `json:"numberFieldId"`
	DateFieldID     int    `json:"dateFieldId"`
	CheckboxFieldID int    `json:"checkboxFieldId"`
}

// Path to test context file (relative to where tests run from)
const testContextPath = ".test-context.json"

// Test app prefix
const testAppPrefix = "test-go-"

var (
	qbRealm     string
	qbUserToken string
	testCtx     *TestContext
	testClient  *quickbase.Client
)

// hasCredentials returns true if credentials are available
func hasCredentials() bool {
	return qbRealm != "" && qbUserToken != ""
}

// skipIfNoCredentials skips the test if credentials are not set
func skipIfNoCredentials(t *testing.T) {
	if !hasCredentials() {
		t.Skip("Skipping integration test: QB_REALM or QB_USER_TOKEN not set")
	}
}

// TestMain handles setup and teardown for all integration tests
func TestMain(m *testing.M) {
	// Load credentials from environment
	qbRealm = os.Getenv(envRealm)
	qbUserToken = os.Getenv(envUserToken)

	if !hasCredentials() {
		fmt.Println("⚠️  No credentials - skipping integration test setup")
		fmt.Println("   Set QB_REALM and QB_USER_TOKEN to run integration tests")
		os.Exit(0)
	}

	// Create test client
	var err error
	testClient, err = quickbase.New(qbRealm, quickbase.WithUserToken(qbUserToken))
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}

	// Setup: cleanup orphaned app and create fresh test app
	if err := setup(); err != nil {
		fmt.Printf("Setup failed: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown: leave app for inspection, will be cleaned up next run
	teardown()

	os.Exit(code)
}

func setup() error {
	ctx := context.Background()

	// Auto-cleanup: Delete orphaned app from previous failed run
	fmt.Println("🧹 Checking for orphaned test apps...")
	cleanupOrphanedApp(ctx)

	// Create fresh test app
	appName := fmt.Sprintf("%s%d", testAppPrefix, time.Now().UnixMilli())
	fmt.Printf("📱 Creating test app: %s\n", appName)

	assignToken := true
	createAppResp, err := testClient.API().CreateAppWithResponse(ctx, generated.CreateAppJSONRequestBody{
		Name:        appName,
		Description: ptr("Integration test app - safe to delete"),
		AssignToken: &assignToken,
	})
	if err != nil {
		return fmt.Errorf("creating app: %w", err)
	}
	if createAppResp.JSON200 == nil {
		return fmt.Errorf("creating app: unexpected response %d", createAppResp.StatusCode())
	}

	appID := *createAppResp.JSON200.Id
	fmt.Printf("   App created: %s\n", appID)

	// Create test table
	fmt.Println("📋 Creating test table...")
	createTableResp, err := testClient.API().CreateTableWithResponse(ctx, &generated.CreateTableParams{
		AppId: appID,
	}, generated.CreateTableJSONRequestBody{
		Name:        "TestRecords",
		Description: ptr("Integration test table"),
	})
	if err != nil {
		return fmt.Errorf("creating table: %w", err)
	}
	if createTableResp.JSON200 == nil {
		return fmt.Errorf("creating table: unexpected response %d", createTableResp.StatusCode())
	}

	tableID := *createTableResp.JSON200.Id
	fmt.Printf("   Table created: %s\n", tableID)

	// Create test fields
	fmt.Println("🔧 Creating test fields...")

	textField, err := createField(ctx, tableID, "Name", "text")
	if err != nil {
		return fmt.Errorf("creating text field: %w", err)
	}

	numberField, err := createField(ctx, tableID, "Amount", "numeric")
	if err != nil {
		return fmt.Errorf("creating number field: %w", err)
	}

	dateField, err := createField(ctx, tableID, "EventDate", "date")
	if err != nil {
		return fmt.Errorf("creating date field: %w", err)
	}

	checkboxField, err := createField(ctx, tableID, "IsActive", "checkbox")
	if err != nil {
		return fmt.Errorf("creating checkbox field: %w", err)
	}

	fmt.Printf("   Fields created: text=%d, numeric=%d, date=%d, checkbox=%d\n",
		textField, numberField, dateField, checkboxField)

	// Save context
	testCtx = &TestContext{
		AppID:           appID,
		TableID:         tableID,
		TextFieldID:     textField,
		NumberFieldID:   numberField,
		DateFieldID:     dateField,
		CheckboxFieldID: checkboxField,
	}

	// Write context to file for inspection/debugging
	if err := writeTestContext(testCtx); err != nil {
		fmt.Printf("   Warning: could not write test context: %v\n", err)
	}

	fmt.Println("✅ Test environment ready")
	return nil
}

func teardown() {
	if testCtx == nil {
		return
	}
	fmt.Printf("\n📌 Test app preserved for inspection: %s\n", testCtx.AppID)
	fmt.Println("   Will be deleted at the start of the next test run")
}

func cleanupOrphanedApp(ctx context.Context) {
	oldCtx, err := readTestContext()
	if err != nil {
		fmt.Println("   No orphaned apps found")
		return
	}

	fmt.Printf("   Found orphaned app: %s, deleting...\n", oldCtx.AppID)

	// Get app name (required for deletion)
	getAppResp, err := testClient.API().GetAppWithResponse(ctx, oldCtx.AppID)
	if err != nil || getAppResp.JSON200 == nil {
		fmt.Println("   Could not fetch orphaned app (may already be deleted)")
		os.Remove(testContextPath)
		return
	}

	// Delete the app
	_, err = testClient.API().DeleteAppWithResponse(ctx, oldCtx.AppID, generated.DeleteAppJSONRequestBody{
		Name: getAppResp.JSON200.Name,
	})
	if err != nil {
		fmt.Printf("   Could not delete orphaned app: %v\n", err)
	} else {
		fmt.Println("   Orphaned app deleted")
	}

	os.Remove(testContextPath)
}

func createField(ctx context.Context, tableID string, label string, fieldType string) (int, error) {
	resp, err := testClient.API().CreateFieldWithResponse(ctx, &generated.CreateFieldParams{
		TableId: tableID,
	}, generated.CreateFieldJSONRequestBody{
		Label:     label,
		FieldType: generated.CreateFieldJSONBodyFieldType(fieldType),
	})
	if err != nil {
		return 0, err
	}
	if resp.JSON200 == nil {
		return 0, fmt.Errorf("unexpected response %d", resp.StatusCode())
	}
	return int(resp.JSON200.Id), nil
}

func writeTestContext(ctx *TestContext) error {
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(testContextPath, data, 0644)
}

func readTestContext() (*TestContext, error) {
	data, err := os.ReadFile(testContextPath)
	if err != nil {
		return nil, err
	}
	var ctx TestContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, err
	}
	return &ctx, nil
}

// ptr returns a pointer to the given value
func ptr[T any](v T) *T {
	return &v
}

// getTestClient returns the shared test client
func getTestClient(t *testing.T) *quickbase.Client {
	skipIfNoCredentials(t)
	return testClient
}

// getTestContext returns the shared test context
func getTestContext(t *testing.T) *TestContext {
	skipIfNoCredentials(t)
	if testCtx == nil {
		t.Fatal("Test context not initialized")
	}
	return testCtx
}

// deleteAllRecords deletes all records from the test table
func deleteAllRecords(t *testing.T, ctx context.Context) {
	client := getTestClient(t)
	testCtx := getTestContext(t)

	_, err := client.API().DeleteRecordsWithResponse(ctx, generated.DeleteRecordsJSONRequestBody{
		From:  testCtx.TableID,
		Where: "{3.GT.0}", // Record ID# > 0
	})
	// Ignore errors (may be no records to delete)
	_ = err
}
