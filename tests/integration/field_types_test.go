package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

func TestDateFields(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	deleteAllRecords(t, ctx)

	t.Run("handles date fields", func(t *testing.T) {
		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
		dateFieldID := fmt.Sprintf("%d", testCtx.DateFieldID)
		testDate := "2024-06-15"

		data := []generated.QuickbaseRecord{
			{
				textFieldID: generated.FieldValue{Value: toFieldValue("DateTest")},
				dateFieldID: generated.FieldValue{Value: toFieldValue(testDate)},
			},
		}

		resp, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:             testCtx.TableID,
			Data:           &data,
			FieldsToReturn: &[]int{3, testCtx.TextFieldID, testCtx.DateFieldID},
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}
		if resp.JSON200.Metadata == nil || resp.JSON200.Metadata.CreatedRecordIds == nil {
			t.Fatal("Expected createdRecordIds")
		}
		if len(*resp.JSON200.Metadata.CreatedRecordIds) != 1 {
			t.Errorf("Expected 1 created record, got %d", len(*resp.JSON200.Metadata.CreatedRecordIds))
		}

		// Query back and verify
		queryResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID, testCtx.DateFieldID},
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if queryResp.JSON200 == nil || queryResp.JSON200.Data == nil {
			t.Fatal("Expected data in response")
		}
		if len(*queryResp.JSON200.Data) != 1 {
			t.Fatalf("Expected 1 record, got %d", len(*queryResp.JSON200.Data))
		}

		record := (*queryResp.JSON200.Data)[0]
		dateValue, err := record[dateFieldID].Value.AsFieldValueValue0()
		if err != nil {
			t.Fatalf("Failed to get date value: %v", err)
		}
		if !strings.Contains(dateValue, "2024-06-15") {
			t.Errorf("Expected date containing 2024-06-15, got %s", dateValue)
		}
	})

	t.Run("filters by date range", func(t *testing.T) {
		deleteAllRecords(t, ctx)

		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
		dateFieldID := fmt.Sprintf("%d", testCtx.DateFieldID)

		data := []generated.QuickbaseRecord{
			{
				textFieldID: generated.FieldValue{Value: toFieldValue("Early")},
				dateFieldID: generated.FieldValue{Value: toFieldValue("2024-01-15")},
			},
			{
				textFieldID: generated.FieldValue{Value: toFieldValue("Mid")},
				dateFieldID: generated.FieldValue{Value: toFieldValue("2024-06-15")},
			},
			{
				textFieldID: generated.FieldValue{Value: toFieldValue("Late")},
				dateFieldID: generated.FieldValue{Value: toFieldValue("2024-12-15")},
			},
		}

		_, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:   testCtx.TableID,
			Data: &data,
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}

		// Query dates after June 1st
		where := fmt.Sprintf("{%d.AF.2024-06-01}", testCtx.DateFieldID)
		queryResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID, testCtx.DateFieldID},
			Where:  &where,
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if queryResp.JSON200 == nil || queryResp.JSON200.Data == nil {
			t.Fatal("Expected data in response")
		}
		if len(*queryResp.JSON200.Data) != 2 {
			t.Errorf("Expected 2 records after June 1st, got %d", len(*queryResp.JSON200.Data))
		}
	})
}

func TestCheckboxFields(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	deleteAllRecords(t, ctx)

	t.Run("handles checkbox fields", func(t *testing.T) {
		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
		checkboxFieldID := fmt.Sprintf("%d", testCtx.CheckboxFieldID)

		data := []generated.QuickbaseRecord{
			{
				textFieldID:     generated.FieldValue{Value: toFieldValue("CheckedItem")},
				checkboxFieldID: generated.FieldValue{Value: toBoolFieldValue(true)},
			},
			{
				textFieldID:     generated.FieldValue{Value: toFieldValue("UncheckedItem")},
				checkboxFieldID: generated.FieldValue{Value: toBoolFieldValue(false)},
			},
		}

		resp, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:             testCtx.TableID,
			Data:           &data,
			FieldsToReturn: &[]int{3, testCtx.TextFieldID, testCtx.CheckboxFieldID},
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Metadata == nil {
			t.Fatal("Expected metadata in response")
		}
		if len(*resp.JSON200.Metadata.CreatedRecordIds) != 2 {
			t.Errorf("Expected 2 created records, got %d", len(*resp.JSON200.Metadata.CreatedRecordIds))
		}

		// Query back and verify
		queryResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID, testCtx.CheckboxFieldID},
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if queryResp.JSON200 == nil || queryResp.JSON200.Data == nil {
			t.Fatal("Expected data in response")
		}
		if len(*queryResp.JSON200.Data) != 2 {
			t.Errorf("Expected 2 records, got %d", len(*queryResp.JSON200.Data))
		}
	})

	t.Run("filters by checkbox value", func(t *testing.T) {
		deleteAllRecords(t, ctx)

		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
		checkboxFieldID := fmt.Sprintf("%d", testCtx.CheckboxFieldID)

		data := []generated.QuickbaseRecord{
			{
				textFieldID:     generated.FieldValue{Value: toFieldValue("Active1")},
				checkboxFieldID: generated.FieldValue{Value: toBoolFieldValue(true)},
			},
			{
				textFieldID:     generated.FieldValue{Value: toFieldValue("Active2")},
				checkboxFieldID: generated.FieldValue{Value: toBoolFieldValue(true)},
			},
			{
				textFieldID:     generated.FieldValue{Value: toFieldValue("Inactive")},
				checkboxFieldID: generated.FieldValue{Value: toBoolFieldValue(false)},
			},
		}

		_, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:   testCtx.TableID,
			Data: &data,
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}

		// Query only checked items
		where := fmt.Sprintf("{%d.EX.true}", testCtx.CheckboxFieldID)
		queryResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID, testCtx.CheckboxFieldID},
			Where:  &where,
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if queryResp.JSON200 == nil || queryResp.JSON200.Data == nil {
			t.Fatal("Expected data in response")
		}
		if len(*queryResp.JSON200.Data) != 2 {
			t.Errorf("Expected 2 active records, got %d", len(*queryResp.JSON200.Data))
		}
	})
}

func TestSpecialCharacters(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	deleteAllRecords(t, ctx)

	t.Run("handles special characters in text fields", func(t *testing.T) {
		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)
		numberFieldID := fmt.Sprintf("%d", testCtx.NumberFieldID)
		specialText := "Test with 'quotes', \"double quotes\", & ampersands, <brackets>"

		data := []generated.QuickbaseRecord{
			{
				textFieldID:   generated.FieldValue{Value: toFieldValue(specialText)},
				numberFieldID: generated.FieldValue{Value: toFieldValue(1)},
			},
		}

		resp, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:             testCtx.TableID,
			Data:           &data,
			FieldsToReturn: &[]int{3, testCtx.TextFieldID},
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Metadata == nil {
			t.Fatal("Expected metadata in response")
		}
		if len(*resp.JSON200.Metadata.CreatedRecordIds) != 1 {
			t.Errorf("Expected 1 created record, got %d", len(*resp.JSON200.Metadata.CreatedRecordIds))
		}

		// Query back and verify
		queryResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID},
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if queryResp.JSON200 == nil || queryResp.JSON200.Data == nil {
			t.Fatal("Expected data in response")
		}

		record := (*queryResp.JSON200.Data)[0]
		textValue, err := record[textFieldID].Value.AsFieldValueValue0()
		if err != nil {
			t.Fatalf("Failed to get text value: %v", err)
		}
		if textValue != specialText {
			t.Errorf("Expected %q, got %q", specialText, textValue)
		}
	})
}

func TestNullValues(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	client := getTestClient(t)
	testCtx := getTestContext(t)

	deleteAllRecords(t, ctx)

	t.Run("handles null/empty values", func(t *testing.T) {
		textFieldID := fmt.Sprintf("%d", testCtx.TextFieldID)

		// Insert record with only required field
		data := []generated.QuickbaseRecord{
			{
				textFieldID: generated.FieldValue{Value: toFieldValue("OnlyName")},
			},
		}

		resp, err := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
			To:             testCtx.TableID,
			Data:           &data,
			FieldsToReturn: &[]int{3, testCtx.TextFieldID, testCtx.NumberFieldID, testCtx.DateFieldID, testCtx.CheckboxFieldID},
		})
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		if resp.JSON200 == nil || resp.JSON200.Metadata == nil {
			t.Fatal("Expected metadata in response")
		}
		if len(*resp.JSON200.Metadata.CreatedRecordIds) != 1 {
			t.Errorf("Expected 1 created record, got %d", len(*resp.JSON200.Metadata.CreatedRecordIds))
		}

		// Query back and verify defaults
		queryResp, err := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
			From:   testCtx.TableID,
			Select: &[]int{3, testCtx.TextFieldID, testCtx.NumberFieldID, testCtx.DateFieldID, testCtx.CheckboxFieldID},
		})
		if err != nil {
			t.Fatalf("RunQuery failed: %v", err)
		}
		if queryResp.JSON200 == nil || queryResp.JSON200.Data == nil {
			t.Fatal("Expected data in response")
		}
		if len(*queryResp.JSON200.Data) != 1 {
			t.Errorf("Expected 1 record, got %d", len(*queryResp.JSON200.Data))
		}

		record := (*queryResp.JSON200.Data)[0]
		textValue, _ := record[textFieldID].Value.AsFieldValueValue0()
		if textValue != "OnlyName" {
			t.Errorf("Expected 'OnlyName', got %q", textValue)
		}

		// Checkbox defaults to false
		checkboxFieldID := fmt.Sprintf("%d", testCtx.CheckboxFieldID)
		checkboxValue, _ := record[checkboxFieldID].Value.AsFieldValueValue2()
		if checkboxValue != false {
			t.Errorf("Expected checkbox to default to false, got %v", checkboxValue)
		}
	})
}

// toBoolFieldValue converts a bool to a FieldValue_Value union type
func toBoolFieldValue(v bool) generated.FieldValue_Value {
	var fv generated.FieldValue_Value
	fv.FromFieldValueValue2(v)
	return fv
}
