package main

// -f structs: a record struct, a changes struct, field IDs and a
// quickbase.Table per table of an app, from the field types QuickBase
// reports. What a field type reads and writes as was checked against a live
// realm (2026-11-12):
//
//   - blank values: text "", numbers null, multi-select [], checkbox false,
//     user null, dates "": so numbers read as *float64 (nil when blank) and
//     the rest as their zero values;
//   - a user field takes an email address or {"id"}; a bare user ID stores a
//     bogus user without an error, so changes take the email;
//   - the built-in fields 1, 2, 4 and 5 can't be written, nor can formula,
//     lookup and summary fields, so changes leave them out.

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
)

type appTable struct {
	id, name, single string
	fields           []appField
}

type appField struct {
	id                     int
	label, fieldType, mode string
}

// fetchApp reads an app's tables and their fields.
func fetchApp(realm, appID, token string) ([]appTable, error) {
	client, err := quickbase.New(realm, quickbase.WithUserToken(token))
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tables, err := client.GetAppTables(ctx, quickbase.GetAppTablesParams{AppId: appID})
	if err != nil {
		return nil, fmt.Errorf("fetching tables: %w", err)
	}
	var out []appTable
	for _, t := range tables {
		if t.Id == "" {
			continue
		}
		fields, err := client.GetFields(ctx, quickbase.GetFieldsParams{TableId: t.Id})
		if err != nil {
			return nil, fmt.Errorf("fetching fields for table %s: %w", t.Id, err)
		}
		table := appTable{id: t.Id, name: t.Name, single: t.SingleRecordName}
		for _, f := range fields {
			table.fields = append(table.fields, appField{id: int(f.Id), label: f.Label, fieldType: f.FieldType, mode: f.Mode})
		}
		sort.Slice(table.fields, func(i, j int) bool { return table.fields[i].id < table.fields[j].id })
		out = append(out, table)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

// goTypes returns the Go types a field reads as (in the record struct) and
// writes as (in the changes struct; "" when it can't be written).
func goTypes(f appField) (read, write string) {
	switch f.fieldType {
	case "text", "text-multi-line", "text-multiple-choice", "rich-text", "phone", "email", "url", "timeofday":
		read, write = "string", "*string"
	case "numeric", "currency", "percent", "rating", "duration":
		read, write = "*float64", "*float64"
	case "recordid":
		read, write = "int", "*int"
	case "checkbox":
		read, write = "bool", "*bool"
	case "multitext":
		read, write = "[]string", "*[]string"
	case "date":
		read, write = "quickbase.Date", "*quickbase.Date"
	case "timestamp":
		read, write = "quickbase.DateTime", "*quickbase.DateTime"
	case "user":
		read, write = "*quickbase.User", "*string"
	case "multiuser":
		read, write = "[]quickbase.User", "*[]string"
	case "dblink":
		read = "any" // a report link: QuickBase computes it
	default:
		read, write = "any", "*any" // address, file attachment, ...: as JSON
	}
	switch {
	case f.id == 1 || f.id == 2 || f.id == 4 || f.id == 5:
		write = "" // built in: date created and modified, record owner, last modified by
	case f.mode == "formula" || f.mode == "lookup" || f.mode == "summary":
		write = ""
	}
	return read, write
}

// names a previous run's file gave the tables and fields, matched by ID, so
// --merge keeps renames.
type existingNames struct {
	table  map[string][3]string      // table ID -> table var, record struct, changes struct
	fields map[string]map[int]string // record struct -> field ID -> Go field name
}

func loadExistingNames(path string) (existingNames, error) {
	names := existingNames{table: map[string][3]string{}, fields: map[string]map[int]string{}}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return names, nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return names, fmt.Errorf("reading %s: %w", path, err)
	}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				st, ok := s.Type.(*ast.StructType)
				if !ok {
					continue
				}
				byID := map[int]string{}
				for _, f := range st.Fields.List {
					if f.Tag == nil || len(f.Names) != 1 {
						continue
					}
					tag, _ := strconv.Unquote(f.Tag.Value)
					if id, err := strconv.Atoi(strings.Split(reflect.StructTag(tag).Get("qb"), ",")[0]); err == nil {
						byID[id] = f.Names[0].Name
					}
				}
				names.fields[s.Name.Name] = byID
			case *ast.ValueSpec:
				// var Projects = quickbase.NewTable[Project, ProjectChanges]("bq...")
				if len(s.Names) != 1 || len(s.Values) != 1 {
					continue
				}
				call, ok := s.Values[0].(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					continue
				}
				index, ok := call.Fun.(*ast.IndexListExpr)
				if !ok || len(index.Indices) != 2 {
					continue
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok {
					continue
				}
				id, _ := strconv.Unquote(lit.Value)
				record, _ := index.Indices[0].(*ast.Ident)
				changes, _ := index.Indices[1].(*ast.Ident)
				if record != nil && changes != nil {
					names.table[id] = [3]string{s.Names[0].Name, record.Name, changes.Name}
				}
			}
		}
	}
	return names, nil
}

// formatAsStructs writes the Go file.
func formatAsStructs(tables []appTable, pkg string, existing existingNames) (string, error) {
	var b strings.Builder
	b.WriteString("// QuickBase tables as Go types.\n")
	b.WriteString("// Generated by: go run github.com/DrewBradfordXYZ/quickbase-go/v3/cmd/schema -f structs\n")
	fmt.Fprintf(&b, "// Generated at: %s\n", time.Now().UTC().Format(time.RFC3339))
	b.WriteString("//\n// Rename types and fields freely: --merge keeps the names, matched by ID.\n\n")
	fmt.Fprintf(&b, "package %s\n\nimport \"github.com/DrewBradfordXYZ/quickbase-go/v3\"\n", pkg)

	used := map[string]bool{}
	for _, t := range tables {
		varName, record, changes := tableNames(t, existing, used)
		known := existing.fields[record]

		fieldNames := map[int]string{}
		taken := map[string]bool{}
		for _, f := range t.fields {
			name := known[f.id]
			if name == "" {
				name = unique(goName(f.label, f.id), taken)
			}
			taken[name] = true
			fieldNames[f.id] = name
		}

		fmt.Fprintf(&b, "\n// %s is a record of the %s table (%s).\ntype %s struct {\n", record, t.name, t.id, record)
		for _, f := range t.fields {
			read, _ := goTypes(f)
			fmt.Fprintf(&b, "\t%s %s `qb:\"%d\"` // %s\n", fieldNames[f.id], read, f.id, describe(f))
		}
		b.WriteString("}\n")

		fmt.Fprintf(&b, "\n// %s sets fields of a %s record in an upsert: a nil field isn't sent.\n", changes, t.name)
		b.WriteString("// Set RecordID (or the table's key field) to update a record; leave it nil to add one.\n")
		fmt.Fprintf(&b, "type %s struct {\n", changes)
		for _, f := range t.fields {
			if _, write := goTypes(f); write != "" {
				comment := describe(f)
				if f.fieldType == "user" || f.fieldType == "multiuser" {
					comment += ": email address"
				}
				fmt.Fprintf(&b, "\t%s %s `qb:\"%d\"` // %s\n", fieldNames[f.id], write, f.id, comment)
			}
		}
		b.WriteString("}\n")

		fmt.Fprintf(&b, "\n// %sFields are the %s table's field IDs, for queries:\n", record, t.name)
		fmt.Fprintf(&b, "// fmt.Sprintf(\"{%%d.EX.'value'}\", %sFields.%s)\n", record, fieldNames[t.fields[len(t.fields)-1].id])
		fmt.Fprintf(&b, "var %sFields = struct {\n", record)
		for _, f := range t.fields {
			fmt.Fprintf(&b, "\t%s int\n", fieldNames[f.id])
		}
		b.WriteString("}{\n")
		for _, f := range t.fields {
			fmt.Fprintf(&b, "\t%s: %d,\n", fieldNames[f.id], f.id)
		}
		b.WriteString("}\n")

		fmt.Fprintf(&b, "\n// %s is the %s table.\nvar %s = quickbase.NewTable[%s, %s](%q)\n", varName, t.name, varName, record, changes, t.id)
	}

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return b.String(), fmt.Errorf("formatting generated code: %w", err)
	}
	return string(formatted), nil
}

// tableNames picks a table's variable, record struct and changes struct
// names: the previous run's, or the table's plural and singular names.
func tableNames(t appTable, existing existingNames, used map[string]bool) (string, string, string) {
	if names, ok := existing.table[t.id]; ok {
		for _, n := range names {
			used[n] = true
		}
		return names[0], names[1], names[2]
	}
	varName := unique(goName(t.name, 0), used)
	used[varName] = true
	single := t.single
	if single == "" {
		single = t.name
	}
	record := goName(single, 0)
	if record == varName {
		record += "Record"
	}
	record = unique(record, used)
	used[record] = true
	changes := unique(record+"Changes", used)
	used[changes] = true
	used[record+"Fields"] = true
	return varName, record, changes
}

func describe(f appField) string {
	d := fmt.Sprintf("%s (%s", strings.TrimSpace(f.label), f.fieldType)
	if f.mode != "" {
		d += ", " + f.mode
	}
	return d + ")"
}

var nonWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// initialisms Go writes in capitals.
var initialisms = map[string]string{"id": "ID", "url": "URL", "api": "API", "html": "HTML", "json": "JSON", "uuid": "UUID", "ip": "IP", "sku": "SKU"}

// goName turns a label ("Record ID#", "2nd Contact") into an exported Go
// name (RecordID, F2ndContact). A label with no letters or digits becomes
// Field<id>.
func goName(label string, id int) string {
	var b strings.Builder
	for _, word := range strings.Fields(nonWord.ReplaceAllString(label, " ")) {
		if up, ok := initialisms[strings.ToLower(word)]; ok {
			b.WriteString(up)
			continue
		}
		r := []rune(word)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	name := b.String()
	if name == "" {
		return fmt.Sprintf("Field%d", id)
	}
	// Exported names start with an upper-case letter: "2nd" and scripts
	// without case ("日付") get a prefix
	if first := []rune(name)[0]; !unicode.IsUpper(first) {
		name = "F" + name
	}
	return name
}

func unique(name string, taken map[string]bool) string {
	if !taken[name] {
		return name
	}
	for i := 2; ; i++ {
		if candidate := fmt.Sprintf("%s%d", name, i); !taken[candidate] {
			return candidate
		}
	}
}
