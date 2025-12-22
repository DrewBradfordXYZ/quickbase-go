package client

import (
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
	"github.com/DrewBradfordXYZ/quickbase-go/internal/generated"
)

func TestQueryBuilder_Select(t *testing.T) {
	// Without schema - using field IDs
	client := &Client{}
	qb := client.Query("bqxyz123").Select(3, 6, 7)

	if qb.err != nil {
		t.Fatalf("unexpected error: %v", qb.err)
	}
	if qb.tableID != "bqxyz123" {
		t.Errorf("expected tableID 'bqxyz123', got '%s'", qb.tableID)
	}
	if len(qb.fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(qb.fields))
	}
}

func TestQueryBuilder_SelectWithSchema(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Field("id", 3).
		Field("name", 6).
		Field("status", 7).
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}
	qb := client.Query("projects").Select("name", "status")

	if qb.err != nil {
		t.Fatalf("unexpected error: %v", qb.err)
	}
	if qb.tableID != "bqxyz123" {
		t.Errorf("expected tableID 'bqxyz123', got '%s'", qb.tableID)
	}

	// Build body and verify field IDs were resolved
	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if body.Select == nil {
		t.Fatal("expected Select to be set")
	}
	if len(*body.Select) != 2 {
		t.Errorf("expected 2 fields, got %d", len(*body.Select))
	}
	if (*body.Select)[0] != 6 {
		t.Errorf("expected field ID 6, got %d", (*body.Select)[0])
	}
	if (*body.Select)[1] != 7 {
		t.Errorf("expected field ID 7, got %d", (*body.Select)[1])
	}
}

func TestQueryBuilder_Where(t *testing.T) {
	client := &Client{}
	qb := client.Query("bqxyz123").
		Select(3, 6).
		Where("{6.EX.'Active'}")

	if qb.err != nil {
		t.Fatalf("unexpected error: %v", qb.err)
	}
	if qb.where != "{6.EX.'Active'}" {
		t.Errorf("expected where clause '{6.EX.'Active'}', got '%s'", qb.where)
	}

	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if body.Where == nil || *body.Where != "{6.EX.'Active'}" {
		t.Error("expected Where to be set in body")
	}
}

func TestQueryBuilder_SortBy(t *testing.T) {
	client := &Client{}
	qb := client.Query("bqxyz123").
		Select(3, 6).
		SortBy(
			SortSpec{Field: 6, Order: generated.SortFieldOrderASC},
			SortSpec{Field: 7, Order: generated.SortFieldOrderDESC},
		)

	if qb.err != nil {
		t.Fatalf("unexpected error: %v", qb.err)
	}
	if len(qb.sortBy) != 2 {
		t.Errorf("expected 2 sort specs, got %d", len(qb.sortBy))
	}

	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if body.SortBy == nil {
		t.Fatal("expected SortBy to be set")
	}
}

func TestQueryBuilder_SortByWithSchema(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Field("name", 6).
		Field("status", 7).
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}
	qb := client.Query("projects").
		Select("name", "status").
		SortBy(
			SortSpec{Field: "name", Order: generated.SortFieldOrderASC},
			SortSpec{Field: "status", Order: generated.SortFieldOrderDESC},
		)

	if qb.err != nil {
		t.Fatalf("unexpected error: %v", qb.err)
	}

	// Build should resolve aliases
	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if body.SortBy == nil {
		t.Fatal("expected SortBy to be set")
	}

	// Extract sort fields from union
	sortFields, err := body.SortBy.AsSortByUnion0()
	if err != nil {
		t.Fatalf("failed to extract sort fields: %v", err)
	}
	if len(sortFields) != 2 {
		t.Errorf("expected 2 sort fields, got %d", len(sortFields))
	}
	if sortFields[0].FieldId != 6 {
		t.Errorf("expected first sort field ID 6, got %d", sortFields[0].FieldId)
	}
	if sortFields[1].FieldId != 7 {
		t.Errorf("expected second sort field ID 7, got %d", sortFields[1].FieldId)
	}
}

func TestQueryBuilder_GroupBy(t *testing.T) {
	client := &Client{}
	qb := client.Query("bqxyz123").
		Select(3, 6, 7).
		GroupBy(7)

	if qb.err != nil {
		t.Fatalf("unexpected error: %v", qb.err)
	}
	if len(qb.groupBy) != 1 {
		t.Errorf("expected 1 groupBy field, got %d", len(qb.groupBy))
	}

	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if body.GroupBy == nil {
		t.Fatal("expected GroupBy to be set")
	}
	if len(*body.GroupBy) != 1 {
		t.Errorf("expected 1 groupBy field in body, got %d", len(*body.GroupBy))
	}
	if *(*body.GroupBy)[0].FieldId != 7 {
		t.Errorf("expected groupBy field ID 7, got %d", *(*body.GroupBy)[0].FieldId)
	}
}

func TestQueryBuilder_Options(t *testing.T) {
	client := &Client{}
	qb := client.Query("bqxyz123").
		Select(3, 6).
		Options(100, 50)

	if qb.err != nil {
		t.Fatalf("unexpected error: %v", qb.err)
	}
	if qb.top == nil || *qb.top != 100 {
		t.Error("expected top=100")
	}
	if qb.skip == nil || *qb.skip != 50 {
		t.Error("expected skip=50")
	}

	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if body.Options == nil {
		t.Fatal("expected Options to be set")
	}
	if body.Options.Top == nil || *body.Options.Top != 100 {
		t.Error("expected body.Options.Top=100")
	}
	if body.Options.Skip == nil || *body.Options.Skip != 50 {
		t.Error("expected body.Options.Skip=50")
	}
}

func TestQueryBuilder_TopSkip(t *testing.T) {
	client := &Client{}
	qb := client.Query("bqxyz123").
		Select(3).
		Top(25).
		Skip(10)

	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if body.Options == nil {
		t.Fatal("expected Options to be set")
	}
	if *body.Options.Top != 25 {
		t.Errorf("expected Top=25, got %d", *body.Options.Top)
	}
	if *body.Options.Skip != 10 {
		t.Errorf("expected Skip=10, got %d", *body.Options.Skip)
	}
}

func TestQueryBuilder_AliasWithoutSchema(t *testing.T) {
	client := &Client{} // No schema
	qb := client.Query("bqxyz123").Select("name") // String alias without schema

	// Build should fail because we can't resolve string aliases without schema
	_, err := qb.buildBody()
	if err == nil {
		t.Fatal("expected error when using string alias without schema")
	}

	var schemaErr *core.SchemaError
	if ok := isSchemaError(err, &schemaErr); !ok {
		t.Errorf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestQueryBuilder_InvalidTableAlias(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}
	qb := client.Query("nonexistent") // Invalid table alias

	if qb.err == nil {
		t.Fatal("expected error for invalid table alias")
	}
}

func TestQueryBuilder_InvalidFieldAlias(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Field("name", 6).
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}
	qb := client.Query("projects").Select("nonexistent")

	// Error happens at build time
	_, err := qb.buildBody()
	if err == nil {
		t.Fatal("expected error for invalid field alias")
	}
}

func TestQueryBuilder_ChainedCalls(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Field("name", 6).
		Field("status", 7).
		Field("dueDate", 12).
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}

	// Full fluent chain
	qb := client.Query("projects").
		Select("name", "status", "dueDate").
		Where("{'status'.EX.'Active'}").
		SortBy(SortSpec{Field: "name", Order: generated.SortFieldOrderASC}).
		GroupBy("status").
		Options(100, 0)

	if qb.err != nil {
		t.Fatalf("unexpected error in chain: %v", qb.err)
	}

	body, err := qb.buildBody()
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	// Verify everything is set
	if body.From != "bqxyz123" {
		t.Errorf("expected From='bqxyz123', got '%s'", body.From)
	}
	if body.Select == nil || len(*body.Select) != 3 {
		t.Error("expected 3 select fields")
	}
	if body.Where == nil {
		t.Error("expected Where to be set")
	}
	if body.SortBy == nil {
		t.Error("expected SortBy to be set")
	}
	if body.GroupBy == nil || len(*body.GroupBy) != 1 {
		t.Error("expected 1 groupBy field")
	}
	if body.Options == nil {
		t.Error("expected Options to be set")
	}
}

// Helper to check if error is SchemaError
func isSchemaError(err error, target **core.SchemaError) bool {
	if se, ok := err.(*core.SchemaError); ok {
		*target = se
		return true
	}
	return false
}
