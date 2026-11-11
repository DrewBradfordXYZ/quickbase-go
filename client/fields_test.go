package client

import (
	"encoding/json"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/v2/generated"
)

// getFields returns summaryQuery on a summary field that has a filter, though
// the official spec documents it only as an updateField input.
func TestFieldsItem_SummaryQuery(t *testing.T) {
	body := `[
		{"id": 12, "label": "Open Tasks", "fieldType": "numeric", "mode": "summary",
		 "properties": {"summaryFunction": "Count", "summaryReferenceFieldId": 8,
		                "summaryQuery": "{'7'.EX.'Open'}AND{'9'.EX.'1'}"}},
		{"id": 13, "label": "All Tasks", "fieldType": "numeric", "mode": "summary",
		 "properties": {"summaryFunction": "Count", "summaryReferenceFieldId": 8}}
	]`

	var items []generated.GetFieldsItem
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	filtered := (&FieldsItem{&items[0]}).Properties()
	if got, want := filtered.SummaryQuery(), "{'7'.EX.'Open'}AND{'9'.EX.'1'}"; got != want {
		t.Errorf("SummaryQuery() = %q, want %q", got, want)
	}
	if _, leaked := filtered.AdditionalProperties["summaryQuery"]; leaked {
		t.Error("summaryQuery also landed in AdditionalProperties")
	}

	if got := (&FieldsItem{&items[1]}).Properties().SummaryQuery(); got != "" {
		t.Errorf("unfiltered SummaryQuery() = %q, want empty", got)
	}
}
