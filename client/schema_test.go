package client

import (
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go/core"
)

func TestClient_Table(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Table("tasks", "bqabc456").
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"resolve alias", "projects", "bqxyz123", false},
		{"resolve another alias", "tasks", "bqabc456", false},
		{"passthrough ID", "bqxyz123", "bqxyz123", false},
		{"unknown alias", "unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.Table(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Table() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Table() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_Table_NoSchema(t *testing.T) {
	client := &Client{} // No schema

	// Without schema, should passthrough
	got, err := client.Table("bqxyz123")
	if err != nil {
		t.Errorf("Table() unexpected error: %v", err)
	}
	if got != "bqxyz123" {
		t.Errorf("Table() = %v, want bqxyz123", got)
	}
}

func TestClient_Fields(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Field("id", 3).
		Field("name", 6).
		Field("status", 7).
		Field("dueDate", 12).
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}

	tests := []struct {
		name    string
		table   string
		fields  []string
		want    []int
		wantErr bool
	}{
		{"single field", "projects", []string{"name"}, []int{6}, false},
		{"multiple fields", "projects", []string{"name", "status", "dueDate"}, []int{6, 7, 12}, false},
		{"using table ID", "bqxyz123", []string{"name"}, []int{6}, false},
		{"unknown field", "projects", []string{"unknown"}, nil, true},
		{"unknown table", "unknown", []string{"name"}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.Fields(tt.table, tt.fields...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Fields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("Fields() = %v, want %v", got, tt.want)
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("Fields()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestClient_Fields_NoSchema(t *testing.T) {
	client := &Client{} // No schema

	_, err := client.Fields("projects", "name")
	if err == nil {
		t.Fatal("Fields() expected error without schema")
	}
	if err != ErrNoSchema {
		t.Errorf("Fields() error = %v, want ErrNoSchema", err)
	}
}

func TestClient_Field(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Field("id", 3).
		Field("name", 6).
		Field("status", 7).
		Build()

	client := &Client{schema: core.ResolveSchema(schema)}

	tests := []struct {
		name    string
		table   string
		field   string
		want    int
		wantErr bool
	}{
		{"resolve field", "projects", "name", 6, false},
		{"resolve another field", "projects", "status", 7, false},
		{"using table ID", "bqxyz123", "name", 6, false},
		{"unknown field", "projects", "unknown", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.Field(tt.table, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("Field() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Field() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_Field_NoSchema(t *testing.T) {
	client := &Client{} // No schema

	_, err := client.Field("projects", "name")
	if err == nil {
		t.Fatal("Field() expected error without schema")
	}
	if err != ErrNoSchema {
		t.Errorf("Field() error = %v, want ErrNoSchema", err)
	}
}

func TestClient_HasSchema(t *testing.T) {
	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Build()

	t.Run("with schema", func(t *testing.T) {
		client := &Client{schema: core.ResolveSchema(schema)}
		if !client.HasSchema() {
			t.Error("HasSchema() = false, want true")
		}
	})

	t.Run("without schema", func(t *testing.T) {
		client := &Client{}
		if client.HasSchema() {
			t.Error("HasSchema() = true, want false")
		}
	})
}
