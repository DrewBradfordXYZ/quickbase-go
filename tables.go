package quickbase

import (
	"context"
	"fmt"
	"iter"
	"reflect"
	"strconv"
	"strings"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
)

// Table is a QuickBase table read into R and written from C, structs whose
// fields carry qb:"<field ID>" tags. `go run ./cmd/schema -f structs`
// generates both for every table of an app, with C's fields all pointers:
// a nil field isn't sent, so a C says exactly which fields to change.
//
//	var Projects = quickbase.NewTable[Project, ProjectChanges]("bqw3ryzab")
//
//	active, err := Projects.Query(ctx, qb, quickbase.TableQuery{Where: "{7.EX.'Active'}"})
//	_, err = Projects.Upsert(ctx, qb, ProjectChanges{Name: quickbase.Ptr("Alpha")})
//
// A query selects R's fields. R can be any struct with qb tags, so a smaller
// struct for the same table selects fewer fields:
//
//	type ProjectName struct {
//		Name string `qb:"6"`
//	}
//	names, err := quickbase.NewTable[ProjectName, ProjectChanges]("bqw3ryzab").Query(ctx, qb, quickbase.TableQuery{})
type Table[R, C any] struct {
	// ID is the table's ID.
	ID string
	// fields R selects
	fields []int
}

// NewTable returns the table with this ID. R's qb tags must be field IDs.
func NewTable[R, C any](id string) *Table[R, C] {
	return &Table[R, C]{ID: id, fields: fieldIDs[R]()}
}

// TableQuery narrows a table query. The zero value is every record.
type TableQuery struct {
	// Where is a QuickBase query: {6.EX.'Active'}AND{7.GT.100}
	Where string
	// SortBy orders the records: quickbase.SortBy{quickbase.Asc(6)}
	SortBy SortBy
	// Limit stops at this many records; 0 is all of them.
	Limit int
}

func (t *Table[R, C]) body(q TableQuery) RunQueryBody {
	fields := t.fields
	body := RunQueryBody{From: t.ID, Select: &fields, Where: q.Where, SortBy: q.SortBy}
	if q.Limit > 0 {
		// QuickBase sends at most Limit records, not a full page
		top := q.Limit
		body.Options = &QueryOptions{Top: &top}
	}
	return body
}

// Query returns the records q selects, every page.
func (t *Table[R, C]) Query(ctx context.Context, qb *Client, q TableQuery) ([]R, error) {
	var out []R
	for r, err := range t.Iterate(ctx, qb, q) {
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// Iterate yields the records q selects, a page at a time, so a large table
// never has to fit in memory. Break out of the loop to stop fetching.
func (t *Table[R, C]) Iterate(ctx context.Context, qb *Client, q TableQuery) iter.Seq2[R, error] {
	return func(yield func(R, error) bool) {
		count := 0
		for record, err := range qb.RunQueryIterator(ctx, t.body(q)) {
			var r R
			if err == nil {
				err = t.keyedByID(qb, record).Decode(&r)
			}
			if !yield(r, err) || err != nil {
				return
			}
			count++
			if q.Limit > 0 && count >= q.Limit {
				return
			}
		}
	}
}

// Get returns the record with this Record ID#. A record that doesn't exist
// is a *NotFoundError.
func (t *Table[R, C]) Get(ctx context.Context, qb *Client, recordID int) (R, error) {
	var zero R
	records, err := t.Query(ctx, qb, TableQuery{Where: fmt.Sprintf("{3.EX.%d}", recordID), Limit: 1})
	if err != nil {
		return zero, err
	}
	if len(records) == 0 {
		return zero, &NotFoundError{QuickbaseError: core.QuickbaseError{
			Message:    fmt.Sprintf("table %s has no record %d", t.ID, recordID),
			StatusCode: 404,
		}}
	}
	return records[0], nil
}

// Upsert adds the changes as records, or updates the record a change's key
// field (usually Record ID#) matches. A nil field isn't sent. Values
// QuickBase rejects are in the result's Metadata.LineErrors (and an error,
// with WithLineErrorsAsErrors).
func (t *Table[R, C]) Upsert(ctx context.Context, qb *Client, changes ...C) (*UpsertResult, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("quickbase: Upsert on table %s has no changes", t.ID)
	}
	data := make([]Record, len(changes))
	for i, c := range changes {
		record, err := core.RecordFrom(c)
		if err != nil {
			if plain, ok := err.(*core.PlainTimeError); ok {
				plain.Record = i + 1
			}
			return nil, err
		}
		data[i] = record
	}
	return qb.Upsert(ctx, UpsertBody{To: t.ID, Data: data})
}

// Delete deletes the records a QuickBase query matches and returns how many.
// An empty query is refused: QuickBase would delete every record.
func (t *Table[R, C]) Delete(ctx context.Context, qb *Client, where string) (int, error) {
	if strings.TrimSpace(where) == "" {
		return 0, fmt.Errorf("quickbase: Delete on table %s needs a query; an empty one deletes every record", t.ID)
	}
	result, err := qb.DeleteRecords(ctx, DeleteRecordsBody{From: t.ID, Where: where})
	if err != nil {
		return 0, err
	}
	return result.NumberDeleted, nil
}

// keyedByID undoes TransformResponses' alias keys for this table, so a
// record matches R's field ID tags whatever the client's schema options.
func (t *Table[R, C]) keyedByID(qb *Client, record Record) Record {
	schema := qb.Schema()
	if schema == nil || !schema.Options.TransformResponses {
		return record
	}
	keyed := make(Record, len(record))
	for key, value := range record {
		if id, err := core.ResolveFieldAlias(schema, t.ID, key); err == nil {
			key = strconv.Itoa(id)
		}
		keyed[key] = value
	}
	return keyed
}

// fieldIDs returns R's qb tags as field IDs, and panics on a tag that isn't
// one: NewTable is called with the generated types, at init.
func fieldIDs[R any]() []int {
	t := reflect.TypeFor[R]()
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("quickbase.NewTable: %s is not a struct", t))
	}
	var ids []int
	for i := 0; i < t.NumField(); i++ {
		tag, ok := t.Field(i).Tag.Lookup("qb")
		if !ok {
			continue
		}
		key, _, _ := strings.Cut(tag, ",")
		id, err := strconv.Atoi(key)
		if err != nil {
			panic(fmt.Sprintf("quickbase.NewTable: %s.%s has tag qb:%q; a table's struct tags are field IDs", t.Name(), t.Field(i).Name, tag))
		}
		ids = append(ids, id)
	}
	return ids
}
