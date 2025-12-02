package integration

import (
	"context"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

func TestNotFoundErrors(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("returns 404 for non-existent table", func(t *testing.T) {
		resp, err := client.API().GetTableWithResponse(ctx, "bzzzzzzzzz", &generated.GetTableParams{
			AppId: testCtx.AppID,
		})
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode() != 404 {
			t.Errorf("Expected 404, got %d", resp.StatusCode())
		}
	})

	t.Run("returns 404 for non-existent field", func(t *testing.T) {
		resp, err := client.API().GetFieldWithResponse(ctx, 99999, &generated.GetFieldParams{
			TableId: testCtx.TableID,
		})
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode() != 404 {
			t.Errorf("Expected 404, got %d", resp.StatusCode())
		}
	})
}

func TestValidationErrors(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("returns 400 for invalid app ID format", func(t *testing.T) {
		resp, err := client.API().GetAppWithResponse(ctx, "invalid_app_id")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode() != 400 {
			t.Errorf("Expected 400, got %d", resp.StatusCode())
		}
	})

	t.Run("returns error for invalid query syntax", func(t *testing.T) {
		where := "this is not valid query syntax"
		resp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3},
			Where:  &where,
		})
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.JSON200 != nil {
			t.Error("Expected error response, got success")
		}
	})

	t.Run("returns error for invalid field ID in select", func(t *testing.T) {
		resp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{99999}, // Non-existent field
		})
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.JSON200 != nil {
			t.Error("Expected error response, got success")
		}
	})
}

func TestEmptyResults(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	// Clean up first
	deleteAllRecords(t, ctx)

	t.Run("returns empty array for query with no matches", func(t *testing.T) {
		where := "{3.EX.999999999}" // Non-existent record ID
		resp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3},
			Where:  &where,
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		if resp.JSON200.Data != nil && len(*resp.JSON200.Data) != 0 {
			t.Errorf("Expected 0 records, got %d", len(*resp.JSON200.Data))
		}
	})

	t.Run("handles deleteRecords with no matching records gracefully", func(t *testing.T) {
		resp, err := client.API().DeleteRecordsWithResponse(ctx, generated.DeleteRecordsJSONRequestBody{
			From:  testCtx.TableID,
			Where: "{3.EX.999999999}",
		})
		if err != nil {
			t.Fatalf("DeleteRecords failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		if resp.JSON200.NumberDeleted == nil || *resp.JSON200.NumberDeleted != 0 {
			t.Errorf("Expected 0 deleted, got %v", resp.JSON200.NumberDeleted)
		}
	})
}
