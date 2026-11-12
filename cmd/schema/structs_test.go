package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// squash collapses runs of spaces and tabs, which gofmt uses to align fields.
func squash(s string) string {
	return regexp.MustCompile(`[ \t]+`).ReplaceAllString(s, " ")
}

func TestGoName(t *testing.T) {
	for label, want := range map[string]string{
		"Record ID#":        "RecordID",
		"Payment Date":      "PaymentDate",
		"2nd Contact":       "F2ndContact",
		"# of Unit Records": "OfUnitRecords",
		"Website URL":       "WebsiteURL",
		"State/Region":      "StateRegion",
		"   ":               "Field9",
	} {
		if got := goName(label, 9); got != want {
			t.Errorf("goName(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestGoTypes(t *testing.T) {
	cases := []struct {
		f           appField
		read, write string
	}{
		{appField{id: 6, fieldType: "text"}, "string", "*string"},
		{appField{id: 7, fieldType: "currency"}, "*float64", "*float64"},
		{appField{id: 8, fieldType: "date"}, "quickbase.Date", "*quickbase.Date"},
		{appField{id: 9, fieldType: "timestamp"}, "quickbase.DateTime", "*quickbase.DateTime"},
		{appField{id: 10, fieldType: "user"}, "*quickbase.User", "*string"}, // written by email
		{appField{id: 11, fieldType: "multitext"}, "[]string", "*[]string"},
		{appField{id: 12, fieldType: "text", mode: "lookup"}, "string", ""}, // computed
		{appField{id: 13, fieldType: "numeric", mode: "summary"}, "*float64", ""},
		{appField{id: 4, fieldType: "user"}, "*quickbase.User", ""}, // built in
		{appField{id: 14, fieldType: "address"}, "any", "*any"},
		{appField{id: 15, fieldType: "dblink"}, "any", ""},
	}
	for _, c := range cases {
		read, write := goTypes(c.f)
		if read != c.read || write != c.write {
			t.Errorf("goTypes(%s %s) = %q, %q; want %q, %q", c.f.fieldType, c.f.mode, read, write, c.read, c.write)
		}
	}
}

var sampleTables = []appTable{{
	id: "bqprojects", name: "Projects", single: "Project",
	fields: []appField{
		{id: 1, label: "Date Created", fieldType: "timestamp"},
		{id: 3, label: "Record ID#", fieldType: "recordid"},
		{id: 6, label: "Name", fieldType: "text"},
		{id: 7, label: "Due", fieldType: "date"},
		{id: 8, label: "Open Tasks", fieldType: "numeric", mode: "summary"},
	},
}, {
	id: "bqstatus", name: "Status", single: "Status",
	fields: []appField{{id: 3, label: "Record ID#", fieldType: "recordid"}},
}}

func TestFormatAsStructs(t *testing.T) {
	out, err := formatAsStructs(sampleTables, "app", existingNames{})
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	out = squash(out)
	for _, want := range []string{
		"package app",
		"type Project struct {",
		"Due quickbase.Date `qb:\"7\"`",
		"type ProjectChanges struct {",
		`var Projects = quickbase.NewTable[Project, ProjectChanges]("bqprojects")`,
		// a table whose singular and plural names are the same
		"type StatusRecord struct {",
		`var Status = quickbase.NewTable[StatusRecord, StatusRecordChanges]("bqstatus")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	changes := out[strings.Index(out, "type ProjectChanges struct {"):]
	changes = changes[:strings.Index(changes, "}")]
	if strings.Contains(changes, "OpenTasks") || strings.Contains(changes, "DateCreated") {
		t.Errorf("changes include computed or built-in fields:\n%s", changes)
	}
}

// --merge keeps renamed types and fields, matched by table and field ID.
func TestMergeKeepsNames(t *testing.T) {
	out, _ := formatAsStructs(sampleTables, "app", existingNames{})
	out = squash(out)
	renamed := strings.NewReplacer(
		"type Project struct", "type Work struct",
		"NewTable[Project, ProjectChanges]", "NewTable[Work, ProjectChanges]",
		"Due quickbase.Date", "Deadline quickbase.Date",
	).Replace(out)
	path := filepath.Join(t.TempDir(), "quickbase_gen.go")
	if err := os.WriteFile(path, []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := loadExistingNames(path)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := formatAsStructs(sampleTables, "app", names)
	if err != nil {
		t.Fatal(err)
	}
	merged = squash(merged)
	for _, want := range []string{"type Work struct {", "Deadline quickbase.Date `qb:\"7\"`", `NewTable[Work, ProjectChanges]("bqprojects")`} {
		if !strings.Contains(merged, want) {
			t.Errorf("merged output lacks %q", want)
		}
	}
}
