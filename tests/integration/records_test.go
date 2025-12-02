package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

func TestUpsertRecords(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	// Clean up before test
	deleteAllRecords(t, ctx)

	t.Run("creates new records", func(t *testing.T) {
		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
		numberFieldID := fmt.Sprintf("%d", testCtx.NumberFieldID)

		data := []generated.QuickbaseRecord{
			{
				textFieldID:   generated.FieldValue{Value: toFieldValue("Alice")},
				numberFieldID: generated.FieldValue{Value: toFieldValue(100)},
			},
			{
				textFieldID:   generated.FieldValue{Value: toFieldValue("Bob")},
				numberFieldID: generated.FieldValue{Value: toFieldValue(200)},
			},
		}

		resp, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:             testCtx.TableID,
			Data:           &data,
			FieldsToReturn: &[]int{3, testCtx.TextFieldID, testCtx.NumberFieldID},
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		if resp.JSON200.Metadata == nil || resp.JSON200.Metadata.CreatedRecordIds == nil {
			t.Fatal("Expected createdRecordIds in metadata")
		}
		if len(*resp.JSON200.Metadata.CreatedRecordIds) != 2 {
			t.Errorf("Created %d records, want 2", len(*resp.JSON200.Metadata.CreatedRecordIds))
		}
	})

	t.Run("updates existing records", func(t *testing.T) {
		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
		numberFieldID := fmt.Sprintf("%d", testCtx.NumberFieldID)

		// Create a record
		createData := []generated.QuickbaseRecord{
			{
				textFieldID:   generated.FieldValue{Value: toFieldValue("Original")},
				numberFieldID: generated.FieldValue{Value: toFieldValue(100)},
			},
		}
		createResp, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:             testCtx.TableID,
			Data:           &createData,
			FieldsToReturn: &[]int{3},
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if createResp.JSON200 == nil || createResp.JSON200.Metadata == nil {
			t.Fatal("Expected metadata in create response")
		}

		recordID := (*createResp.JSON200.Metadata.CreatedRecordIds)[0]

		// Update the record using merge field
		mergeFieldID := 3 // Record ID#
		updateData := []generated.QuickbaseRecord{
			{
				"3":           generated.FieldValue{Value: toFieldValue(recordID)},
				textFieldID:   generated.FieldValue{Value: toFieldValue("Updated")},
				numberFieldID: generated.FieldValue{Value: toFieldValue(999)},
			},
		}
		updateResp, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:             testCtx.TableID,
			Data:           &updateData,
			MergeFieldId:   &mergeFieldID,
			FieldsToReturn: &[]int{3, testCtx.TextFieldID, testCtx.NumberFieldID},
		})
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if updateResp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", updateResp.StatusCode())
		}

		// Verify the update
		if updateResp.JSON200.Metadata == nil || updateResp.JSON200.Metadata.UpdatedRecordIds == nil {
			t.Fatal("Expected updatedRecordIds in metadata")
		}

		found := false
		for _, id := range *updateResp.JSON200.Metadata.UpdatedRecordIds {
			if id == recordID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Record %d not in updatedRecordIds", recordID)
		}
	})
}

func TestRunQuery(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	// Clean up and insert test data
	deleteAllRecords(t, ctx)
	insertTestRecords(t, ctx, 5)

	t.Run("queries records with filter", func(t *testing.T) {
		where := fmt.Sprintf("{%d.GT.2}", testCtx.NumberFieldID)
		resp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID, testCtx.NumberFieldID},
			Where:  &where,
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		// Should return records with Amount > 2 (i.e., records 3, 4, 5)
		if resp.JSON200.Data == nil {
			t.Fatal("Expected data in response")
		}
		if len(*resp.JSON200.Data) != 3 {
			t.Errorf("Got %d records, want 3", len(*resp.JSON200.Data))
		}
	})

	t.Run("queries records with sorting", func(t *testing.T) {
		sortBy := generated.RunQueryJSONBody_SortBy{}
		sortFields := []generated.SortField{
			{
				FieldId: testCtx.NumberFieldID,
				Order:   generated.SortFieldOrder("DESC"),
			},
		}
		sortBy.FromRunQueryJSONBodySortBy0(sortFields)

		resp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID, testCtx.NumberFieldID},
			SortBy: &sortBy,
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}

		// Check that records are sorted descending by number field
		data := *resp.JSON200.Data
		if len(data) < 2 {
			t.Skip("Not enough records to verify sorting")
		}

		// Get the number values from first two records
		numberFieldKey := fmt.Sprintf("%d", testCtx.NumberFieldID)
		firstValue := data[0][numberFieldKey].Value
		secondValue := data[1][numberFieldKey].Value

		first, _ := firstValue.AsFieldValueValue1()
		second, _ := secondValue.AsFieldValueValue1()

		if first < second {
			t.Errorf("Records not sorted DESC: first=%v, second=%v", first, second)
		}
	})
}

func TestDeleteRecords(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	t.Run("deletes records", func(t *testing.T) {
		// Clean and insert test data
		deleteAllRecords(t, ctx)
		insertTestRecords(t, ctx, 3)

		// Verify records exist
		beforeResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3},
		})
		if err != nil {
			t.Fatalf("RunQuery before delete failed: %v", err)
		}
		if beforeResp.JSON200 == nil || len(*beforeResp.JSON200.Data) == 0 {
			t.Fatal("Expected records before delete")
		}

		// Delete all records
		deleteResp, err := client.API().DeleteRecordsWithResponse(ctx, generated.DeleteRecordsJSONRequestBody{
			From:  testCtx.TableID,
			Where: ptr("{3.GT.0}"),
		})
		if err != nil {
			t.Fatalf("DeleteRecords failed: %v", err)
		}
		if deleteResp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", deleteResp.StatusCode())
		}

		if deleteResp.JSON200.NumberDeleted == nil || *deleteResp.JSON200.NumberDeleted == 0 {
			t.Error("Expected records to be deleted")
		}

		// Verify records deleted
		afterResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3},
		})
		if err != nil {
			t.Fatalf("RunQuery after delete failed: %v", err)
		}
		if afterResp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", afterResp.StatusCode())
		}
		if afterResp.JSON200.Data != nil && len(*afterResp.JSON200.Data) > 0 {
			t.Errorf("Expected 0 records after delete, got %d", len(*afterResp.JSON200.Data))
		}
	})
}

// insertTestRecords inserts n test records
func insertTestRecords(t *testing.T, ctx context.Context, n int) {
	t.Helper()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
	numberFieldID := fmt.Sprintf("%d", testCtx.NumberFieldID)

	data := make([]generated.QuickbaseRecord, n)
	for i := 0; i < n; i++ {
		data[i] = generated.QuickbaseRecord{
			textFieldID:   generated.FieldValue{Value: toFieldValue(fmt.Sprintf("Record %d", i+1))},
			numberFieldID: generated.FieldValue{Value: toFieldValue(i + 1)},
		}
	}

	_, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
		To:   testCtx.TableID,
		Data: &data,
	})
	if err != nil {
		t.Fatalf("Failed to insert test records: %v", err)
	}
}

// toFieldValue converts a value to a FieldValue_Value union type
func toFieldValue(v any) generated.FieldValue_Value {
	var fv generated.FieldValue_Value
	switch val := v.(type) {
	case string:
		fv.FromFieldValueValue0(val)
	case int:
		fv.FromFieldValueValue1(float32(val))
	case float32:
		fv.FromFieldValueValue1(val)
	case float64:
		fv.FromFieldValueValue1(float32(val))
	}
	return fv
}
