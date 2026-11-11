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

// A reference field's parent table comes back as masterTableId; the
// documented masterChoiceTableId is absent.
func TestFieldsItem_ReferenceFieldParent(t *testing.T) {
	body := `[{"id": 8, "label": "Related Project", "fieldType": "numeric", "mode": "",
		"properties": {"foreignKey": true, "masterTableId": "bqparent1", "masterTableKeyFid": 3,
		               "fieldCaching": true, "openTargetIn": "sameWindow", "primaryKey": false}}]`

	var items []generated.GetFieldsItem
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	props := (&FieldsItem{&items[0]}).Properties()
	if got := props.MasterTableId(); got != "bqparent1" {
		t.Errorf("MasterTableId() = %q, want bqparent1", got)
	}
	if got := props.MasterTableKeyFid(); got != 3 {
		t.Errorf("MasterTableKeyFid() = %d, want 3", got)
	}
	if len(props.AdditionalProperties) != 0 {
		t.Errorf("AdditionalProperties = %v, want every property typed", props.AdditionalProperties)
	}
}

// Report properties are typed for the settings the spec lists; a report
// type's other settings stay in AdditionalProperties.
func TestReportProperties_TypedAndKept(t *testing.T) {
	body := `{"id": "7", "name": "Board", "type": "kanban",
		"query": {"fields": [3, 6], "sortBy": [{"fieldId": 6, "order": "ASC"}], "groupBy": [{"fieldId": 9, "grouping": "equal-values"}]},
		"properties": {"displayOnlyNewOrChangedRecords": true, "stagesFromFieldId": 12,
		               "columnProperties": [{"fieldId": 6, "labelOverride": "Unit", "width": 120}]}}`

	var item generated.GetTableReportsItem
	if err := json.Unmarshal([]byte(body), &item); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if q := item.Query; q == nil || q.Fields == nil || len(*q.Fields) != 2 || (*q.SortBy)[0].FieldId == nil || *(*q.GroupBy)[0].FieldId != 9 {
		t.Errorf("query = %+v", item.Query)
	}
	p := item.Properties
	if p == nil || p.DisplayOnlyNewOrChangedRecords == nil || !*p.DisplayOnlyNewOrChangedRecords {
		t.Fatalf("properties = %+v", p)
	}
	if _, ok := p.AdditionalProperties["stagesFromFieldId"]; !ok {
		t.Error("kanban setting stagesFromFieldId was dropped")
	}
	col := (*p.ColumnProperties)[0]
	if *col.LabelOverride != "Unit" {
		t.Errorf("labelOverride = %q", *col.LabelOverride)
	}
	if _, ok := col.AdditionalProperties["width"]; !ok {
		t.Error("unlisted column key width was dropped")
	}
}
