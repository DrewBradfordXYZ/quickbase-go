package integration

import (
	"context"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/generated"
)

// TestRelationships tests all relationship CRUD operations
func TestRelationships(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	// Create a child table for relationship testing
	childTableID := createChildTable(t, ctx, testCtx.AppID)
	defer deleteTable(t, ctx, testCtx.AppID, childTableID)

	var relationshipID float32

	t.Run("CreateRelationship", func(t *testing.T) {
		// Create a relationship from child table to parent table
		resp, err := client.API().CreateRelationshipWithResponse(ctx, childTableID,
			generated.CreateRelationshipJSONRequestBody{
				ParentTableId: testCtx.TableID,
			})
		if err != nil {
			t.Fatalf("CreateRelationship failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d: %s", resp.StatusCode(), string(resp.Body))
		}

		// Store relationship ID for subsequent tests
		relationshipID = float32(resp.JSON200.Id)
		t.Logf("Created relationship: %v", relationshipID)

		// Verify parent/child table IDs
		if resp.JSON200.ParentTableId != testCtx.TableID {
			t.Errorf("ParentTableId = %s, want %s", resp.JSON200.ParentTableId, testCtx.TableID)
		}
		if resp.JSON200.ChildTableId != childTableID {
			t.Errorf("ChildTableId = %s, want %s", resp.JSON200.ChildTableId, childTableID)
		}
	})

	t.Run("GetRelationships", func(t *testing.T) {
		if relationshipID == 0 {
			t.Skip("No relationship created")
		}

		// Get relationships from child table
		resp, err := client.GetRelationships(childTableID).Run(ctx)
		if err != nil {
			t.Fatalf("GetRelationships failed: %v", err)
		}

		// Access response via accessor method
		rels := resp.Relationships()
		if len(rels) == 0 {
			t.Error("Expected at least one relationship")
		}

		// Find our relationship
		found := false
		for _, rel := range rels {
			if float32(rel.Id()) == relationshipID {
				found = true
				// Verify parent table ID
				if rel.ParentTableId() != testCtx.TableID {
					t.Errorf("ParentTableId = %s, want %s", rel.ParentTableId(), testCtx.TableID)
				}
				break
			}
		}
		if !found {
			t.Errorf("Relationship %v not found in GetRelationships response", relationshipID)
		}
	})

	t.Run("GetRelationships_Builder", func(t *testing.T) {
		if relationshipID == 0 {
			t.Skip("No relationship created")
		}

		// Test the builder pattern (this was the bug - it should use tableID correctly)
		resp, err := client.GetRelationships(childTableID).Run(ctx)
		if err != nil {
			t.Fatalf("GetRelationships builder failed: %v", err)
		}

		// Verify we got relationships via accessor method
		rels := resp.Relationships()
		if len(rels) == 0 {
			t.Error("Expected relationships from builder")
		}
	})

	t.Run("UpdateRelationship", func(t *testing.T) {
		if relationshipID == 0 {
			t.Skip("No relationship created")
		}

		// Get the text field ID from parent table to use as lookup
		lookupFieldID := testCtx.TextFieldID

		// Update the relationship to add a lookup field
		resp, err := client.API().UpdateRelationshipWithResponse(ctx, childTableID, relationshipID,
			generated.UpdateRelationshipJSONRequestBody{
				LookupFieldIds: &[]int{lookupFieldID},
			})
		if err != nil {
			t.Fatalf("UpdateRelationship failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d: %s", resp.StatusCode(), string(resp.Body))
		}

		// Verify lookup fields were added
		if resp.JSON200.LookupFields == nil || len(*resp.JSON200.LookupFields) == 0 {
			t.Error("Expected lookup fields in response")
		}
	})

	t.Run("DeleteRelationship", func(t *testing.T) {
		if relationshipID == 0 {
			t.Skip("No relationship created")
		}

		// Delete the relationship
		resp, err := client.API().DeleteRelationshipWithResponse(ctx, childTableID, relationshipID)
		if err != nil {
			t.Fatalf("DeleteRelationship failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d: %s", resp.StatusCode(), string(resp.Body))
		}

		// Verify GetRelationships no longer returns it
		getResp, err := client.GetRelationships(childTableID).Run(ctx)
		if err != nil {
			t.Fatalf("GetRelationships after delete failed: %v", err)
		}

		for _, rel := range getResp.Relationships() {
			if float32(rel.Id()) == relationshipID {
				t.Error("Relationship still exists after delete")
			}
		}
	})
}

// TestGetRelationships_NoRelationships verifies behavior on table with no relationships
func TestGetRelationships_NoRelationships(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	// The main test table has no relationships initially
	resp, err := client.GetRelationships(testCtx.TableID).Run(ctx)
	if err != nil {
		t.Fatalf("GetRelationships failed: %v", err)
	}

	// Should return empty relationships array, not an error
	rels := resp.Relationships()
	t.Logf("Relationships count: %d", len(rels))
}

// createChildTable creates a temporary child table for relationship testing
func createChildTable(t *testing.T, ctx context.Context, appID string) string {
	t.Helper()

	resp, err := testClient.API().CreateTableWithResponse(ctx, &generated.CreateTableParams{
		AppId: appID,
	}, generated.CreateTableJSONRequestBody{
		Name:        "ChildTable",
		Description: ptr("Child table for relationship testing"),
	})
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
	}

	tableID := *resp.JSON200.Id
	t.Logf("Created child table: %s", tableID)
	return tableID
}

// deleteTable deletes a table
func deleteTable(t *testing.T, ctx context.Context, appID, tableID string) {
	t.Helper()

	_, err := testClient.API().DeleteTableWithResponse(ctx, tableID, &generated.DeleteTableParams{
		AppId: appID,
	})
	if err != nil {
		t.Logf("Warning: failed to delete table %s: %v", tableID, err)
	} else {
		t.Logf("Deleted table: %s", tableID)
	}
}
