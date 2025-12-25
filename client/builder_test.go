package client

import (
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/generated"
)

// Helper to create a resolved schema for tests
func testSchema(tables map[string]core.TableSchema) *core.ResolvedSchema {
	schema := &core.Schema{Tables: tables}
	return core.ResolveSchema(schema)
}

func TestSortSpec(t *testing.T) {
	// Test with int field
	specInt := SortSpec{Field: 6, Order: generated.SortFieldOrderASC}
	if specInt.Field != 6 {
		t.Errorf("Field = %v, want 6", specInt.Field)
	}
	if specInt.Order != generated.SortFieldOrderASC {
		t.Errorf("Order = %v, want ASC", specInt.Order)
	}

	// Test with string field
	specStr := SortSpec{Field: "name", Order: generated.SortFieldOrderDESC}
	if specStr.Field != "name" {
		t.Errorf("Field = %v, want 'name'", specStr.Field)
	}
	if specStr.Order != generated.SortFieldOrderDESC {
		t.Errorf("Order = %v, want DESC", specStr.Order)
	}
}

func TestCreateAppBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.CreateApp()

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.client != c {
		t.Error("client not set correctly")
	}
}

func TestCreateAppBuilder_Chaining(t *testing.T) {
	c := &Client{}
	b := c.CreateApp().
		Name("Test App").
		Description("A test application").
		AssignToken(true)

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.params["name"] != "Test App" {
		t.Errorf("name = %v, want 'Test App'", b.params["name"])
	}

	if b.params["description"] != "A test application" {
		t.Errorf("description = %v, want 'A test application'", b.params["description"])
	}

	if b.params["assignToken"] != true {
		t.Errorf("assignToken = %v, want true", b.params["assignToken"])
	}
}

func TestUpdateAppBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.UpdateApp("bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.appId != "bqxyz123" {
		t.Errorf("appId = %q, want %q", b.appId, "bqxyz123")
	}
}

func TestUpdateAppBuilder_Chaining(t *testing.T) {
	c := &Client{}
	b := c.UpdateApp("bqxyz123").
		Name("Updated App").
		Description("Updated description")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.params["name"] != "Updated App" {
		t.Errorf("name = %v, want 'Updated App'", b.params["name"])
	}

	if b.params["description"] != "Updated description" {
		t.Errorf("description = %v, want 'Updated description'", b.params["description"])
	}
}

func TestCopyAppBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.CopyApp("bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.appId != "bqxyz123" {
		t.Errorf("appId = %q, want %q", b.appId, "bqxyz123")
	}
}

func TestCopyAppBuilder_Chaining(t *testing.T) {
	c := &Client{}
	b := c.CopyApp("bqxyz123").
		Name("Copied App").
		Description("Copy of the original").
		KeepData(true)

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.params["name"] != "Copied App" {
		t.Errorf("name = %v, want 'Copied App'", b.params["name"])
	}

	if b.params["description"] != "Copy of the original" {
		t.Errorf("description = %v, want 'Copy of the original'", b.params["description"])
	}

	// keepData is a nested property under "properties"
	props, ok := b.params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties not set correctly")
	}
	if props["keepData"] != true {
		t.Errorf("keepData = %v, want true", props["keepData"])
	}
}

// Note: CreateTable and UpdateTable builders are skipped due to complex enum types
// They can be accessed via RawCreateTable and RawUpdateTable methods

// Note: CreateField builder is skipped due to int64 type for Id field
// It can be accessed via RawCreateField method

func TestRunReportBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.RunReport("1", "bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.reportId != "1" {
		t.Errorf("reportId = %q, want %q", b.reportId, "1")
	}
}

func TestDeleteAppBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.DeleteApp("bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.appId != "bqxyz123" {
		t.Errorf("appId = %q, want %q", b.appId, "bqxyz123")
	}
}

func TestDeleteAppBuilder_Chaining(t *testing.T) {
	c := &Client{}
	b := c.DeleteApp("bqxyz123").
		Name("App to Delete")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.params["name"] != "App to Delete" {
		t.Errorf("name = %v, want 'App to Delete'", b.params["name"])
	}
}

func TestCreateRelationshipBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - unknown alias errors",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:   "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.CreateRelationship(tt.table)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestGetRelationshipsBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - unknown alias errors",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:   "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.GetRelationships(tt.table)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestDeleteRelationshipBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			wantTable: "bqxyz123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.DeleteRelationship(tt.table, 1)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestUpdateRelationshipBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			wantTable: "bqxyz123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.UpdateRelationship(tt.table, 1)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestRunReportBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		reportId  string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			reportId:  "1",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			reportId:  "1",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - unknown alias errors",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:    "unknown",
			reportId: "1",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.RunReport(tt.reportId, tt.table)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestRunReportBuilder_Chaining(t *testing.T) {
	c := &Client{}
	skip := 10
	top := 50

	b := c.RunReport("1", "bqxyz123").
		Skip(skip).
		Top(top)

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.reportId != "1" {
		t.Errorf("reportId = %q, want %q", b.reportId, "1")
	}

	if *b.qpSkip != skip {
		t.Errorf("skip = %v, want %v", *b.qpSkip, skip)
	}

	if *b.qpTop != top {
		t.Errorf("top = %v, want %v", *b.qpTop, top)
	}
}

func TestUpsertBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - unknown alias errors",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:   "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.Upsert(tt.table)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestUpsertBuilder_Chaining(t *testing.T) {
	c := &Client{}
	data := []any{
		map[string]any{"6": map[string]any{"value": "test"}},
	}

	b := c.Upsert("bqxyz123").
		Data(data...).
		MergeFieldId(6).
		FieldsToReturn(3, 6, 7)

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.params["data"] == nil {
		t.Error("data not set")
	}

	if b.params["mergeFieldId"] != 6 {
		t.Errorf("mergeFieldId = %v, want 6", b.params["mergeFieldId"])
	}

	fieldsToReturn := b.params["fieldsToReturn"].([]int)
	if len(fieldsToReturn) != 3 || fieldsToReturn[0] != 3 {
		t.Errorf("fieldsToReturn = %v, want [3 6 7]", fieldsToReturn)
	}
}

func TestDeleteRecordsBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - unknown alias errors",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:   "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.DeleteRecords(tt.table)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestDeleteRecordsBuilder_Chaining(t *testing.T) {
	c := &Client{}
	b := c.DeleteRecords("bqxyz123").
		Where("{3.EX.'123'}")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.params["where"] != "{3.EX.'123'}" {
		t.Errorf("where = %v, want {3.EX.'123'}", b.params["where"])
	}
}

func TestGetFieldsBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.GetFields("bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.tableID != "bqxyz123" {
		t.Errorf("tableID = %q, want %q", b.tableID, "bqxyz123")
	}
}

func TestGetFieldsBuilder_TableResolution(t *testing.T) {
	tests := []struct {
		name      string
		schema    *core.ResolvedSchema
		table     string
		wantErr   bool
		wantTable string
	}{
		{
			name:      "no schema - table ID passthrough",
			schema:    nil,
			table:     "bqxyz123",
			wantTable: "bqxyz123",
		},
		{
			name: "with schema - alias resolved",
			schema: testSchema(map[string]core.TableSchema{
				"projects": {ID: "bqxyz123"},
			}),
			table:     "projects",
			wantTable: "bqxyz123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{schema: tt.schema}
			b := c.GetFields(tt.table)

			if tt.wantErr {
				if b.err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if b.err != nil {
				t.Errorf("unexpected error: %v", b.err)
				return
			}

			if b.tableID != tt.wantTable {
				t.Errorf("tableID = %q, want %q", b.tableID, tt.wantTable)
			}
		})
	}
}

func TestGetFieldBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.GetField(6, "bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.tableID != "bqxyz123" {
		t.Errorf("tableID = %q, want %q", b.tableID, "bqxyz123")
	}

	if b.fieldId != 6 {
		t.Errorf("fieldId = %d, want 6", b.fieldId)
	}
}

func TestGetTableBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.GetTable("bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.tableID != "bqxyz123" {
		t.Errorf("tableID = %q, want %q", b.tableID, "bqxyz123")
	}
}

func TestGetReportBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.GetReport("1", "bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.tableID != "bqxyz123" {
		t.Errorf("tableID = %q, want %q", b.tableID, "bqxyz123")
	}

	if b.reportId != "1" {
		t.Errorf("reportId = %q, want %q", b.reportId, "1")
	}
}

func TestGetAppBuilder_Basic(t *testing.T) {
	c := &Client{}
	b := c.GetApp("bqxyz123")

	if b.err != nil {
		t.Errorf("unexpected error: %v", b.err)
	}

	if b.appId != "bqxyz123" {
		t.Errorf("appId = %q, want %q", b.appId, "bqxyz123")
	}
}

// NOTE: Result type structure tests were removed in v2.0.
// In the raw-first architecture, all builders return generated types directly.
// Users can use opt-in helpers like UnwrapRecords() and Deref() for convenience.
