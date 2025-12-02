package integration

import (
	"context"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

func TestGetApp(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("gets app details", func(t *testing.T) {
		resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetApp failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		if resp.JSON200.Id == nil || *resp.JSON200.Id != testCtx.AppID {
			t.Errorf("App ID = %v, want %s", resp.JSON200.Id, testCtx.AppID)
		}
		if resp.JSON200.Name == "" {
			t.Error("Expected app name")
		}
	})
}

func TestGetTable(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("gets table details", func(t *testing.T) {
		resp, err := client.API().GetTableWithResponse(ctx, testCtx.TableID, &generated.GetTableParams{
			AppId: testCtx.AppID,
		})
		if err != nil {
			t.Fatalf("GetTable failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		if *resp.JSON200.Id != testCtx.TableID {
			t.Errorf("Table ID = %s, want %s", *resp.JSON200.Id, testCtx.TableID)
		}
		if resp.JSON200.Name == nil || *resp.JSON200.Name == "" {
			t.Error("Expected table name")
		}
	})
}

func TestGetAppTables(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("gets all tables in app", func(t *testing.T) {
		resp, err := client.API().GetAppTablesWithResponse(ctx, &generated.GetAppTablesParams{
			AppId: testCtx.AppID,
		})
		if err != nil {
			t.Fatalf("GetAppTables failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		// Should have at least our test table
		if len(*resp.JSON200) == 0 {
			t.Error("Expected at least one table")
		}

		// Find our test table
		found := false
		for _, table := range *resp.JSON200 {
			if *table.Id == testCtx.TableID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Test table %s not found in app tables", testCtx.TableID)
		}
	})
}

func TestGetFields(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("gets all fields in table", func(t *testing.T) {
		resp, err := client.API().GetFieldsWithResponse(ctx, &generated.GetFieldsParams{
			TableId: testCtx.TableID,
		})
		if err != nil {
			t.Fatalf("GetFields failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		// Should have our test fields plus built-in fields
		if len(*resp.JSON200) < 4 {
			t.Errorf("Expected at least 4 fields, got %d", len(*resp.JSON200))
		}

		// Check for our test fields
		fieldIDs := make(map[int64]bool)
		for _, field := range *resp.JSON200 {
			fieldIDs[field.Id] = true
		}

		if !fieldIDs[int64(testCtx.TextFieldID)] {
			t.Errorf("Text field %d not found", testCtx.TextFieldID)
		}
		if !fieldIDs[int64(testCtx.NumberFieldID)] {
			t.Errorf("Number field %d not found", testCtx.NumberFieldID)
		}
		if !fieldIDs[int64(testCtx.DateFieldID)] {
			t.Errorf("Date field %d not found", testCtx.DateFieldID)
		}
		if !fieldIDs[int64(testCtx.CheckboxFieldID)] {
			t.Errorf("Checkbox field %d not found", testCtx.CheckboxFieldID)
		}
	})
}

func TestGetField(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("gets field details", func(t *testing.T) {
		resp, err := client.API().GetFieldWithResponse(ctx, testCtx.TextFieldID, &generated.GetFieldParams{
			TableId: testCtx.TableID,
		})
		if err != nil {
			t.Fatalf("GetField failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		if resp.JSON200.Id != int64(testCtx.TextFieldID) {
			t.Errorf("Field ID = %d, want %d", resp.JSON200.Id, testCtx.TextFieldID)
		}
		if resp.JSON200.Label == nil || *resp.JSON200.Label != "Name" {
			t.Errorf("Field label = %v, want 'Name'", resp.JSON200.Label)
		}
		if resp.JSON200.FieldType == nil || *resp.JSON200.FieldType != "text" {
			t.Errorf("Field type = %v, want 'text'", resp.JSON200.FieldType)
		}
	})
}
