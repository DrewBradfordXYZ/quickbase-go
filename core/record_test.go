package core

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// Records as QuickBase returns them for each field type, filled and blank
// (from a live realm: text "", numbers null, multi-select [], checkbox
// false, user null, dates "").
const filledJSON = `{"3":{"value":12},"6":{"value":"Alpha"},"7":{"value":"2024-01-15"},
	"8":{"value":"2024-01-15T15:30:00Z"},"9":{"value":1234.5},"10":{"value":["A","B"]},
	"11":{"value":true},"12":{"value":{"email":"a@example.com","id":"1.ab","name":"Ann"}},
	"13":{"value":5400000},"14":{"value":"15:30:00"}}`
const blankJSON = `{"3":{"value":13},"6":{"value":""},"7":{"value":""},"8":{"value":""},
	"9":{"value":null},"10":{"value":[]},"11":{"value":false},"12":{"value":null},
	"13":{"value":null},"14":{"value":""}}`

type project struct {
	ID       int      `qb:"3"`
	Name     string   `qb:"6"`
	Due      Date     `qb:"7"`
	Closed   DateTime `qb:"8"`
	Budget   *float64 `qb:"9"`
	Tags     []string `qb:"10"`
	Done     bool     `qb:"11"`
	Owner    *User    `qb:"12"`
	Duration int64    `qb:"13"`
	Start    string   `qb:"14"`
	Note     string   // untagged: left alone
}

func decode(t *testing.T, raw string) project {
	t.Helper()
	var r Record
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	p := project{Note: "kept"}
	if err := r.Decode(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDecodeFilled(t *testing.T) {
	p := decode(t, filledJSON)
	if p.ID != 12 || p.Name != "Alpha" || p.Due.String() != "2024-01-15" || p.Done != true || p.Duration != 5400000 || p.Start != "15:30:00" {
		t.Errorf("decoded %+v", p)
	}
	if !p.Closed.Equal(time.Date(2024, 1, 15, 15, 30, 0, 0, time.UTC)) {
		t.Errorf("Closed = %v", p.Closed)
	}
	if p.Budget == nil || *p.Budget != 1234.5 {
		t.Errorf("Budget = %v", p.Budget)
	}
	if len(p.Tags) != 2 || p.Owner == nil || p.Owner.Email != "a@example.com" || p.Note != "kept" {
		t.Errorf("Tags %v Owner %+v Note %q", p.Tags, p.Owner, p.Note)
	}
}

func TestDecodeBlanksAreZero(t *testing.T) {
	p := decode(t, blankJSON)
	if p.Name != "" || p.Due != (Date{}) || !p.Closed.IsZero() || p.Budget != nil || len(p.Tags) != 0 || p.Done || p.Owner != nil || p.Duration != 0 {
		t.Errorf("decoded blanks as %+v", p)
	}
}

func TestDecodeErrors(t *testing.T) {
	var r Record
	_ = json.Unmarshal([]byte(`{"6":{"value":5}}`), &r)
	var wrong struct {
		Name bool `qb:"6"`
	}
	if err := r.Decode(&wrong); err == nil {
		t.Error("a number decoded into a bool")
	}
	if err := r.Decode(wrong); err == nil {
		t.Error("Decode took a struct, not a pointer")
	}
}

func TestDecodeRecordsByAlias(t *testing.T) {
	records := []Record{{"name": {Value: "Alpha"}}, {"name": {Value: "Beta"}}}
	type row struct {
		Name string `qb:"name"`
	}
	rows, err := DecodeRecords[row](records)
	if err != nil || len(rows) != 2 || rows[1].Name != "Beta" {
		t.Errorf("DecodeRecords = %+v, %v", rows, err)
	}
}

// A struct of pointers says which fields to change: nil ones aren't sent.
func TestRecordFrom(t *testing.T) {
	name := "Alpha"
	due := Date{2024, time.January, 15}
	changes := struct {
		Name   *string  `qb:"6"`
		Due    *Date    `qb:"7"`
		Budget *float64 `qb:"9"`
		Done   bool     `qb:"11"`
	}{Name: &name, Due: &due}

	rec, err := RecordFrom(changes)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(rec)
	if string(b) != `{"11":{"value":false},"6":{"value":"Alpha"},"7":{"value":"2024-01-15"}}` {
		t.Errorf("RecordFrom = %s", b)
	}

	var plain *PlainTimeError
	if _, err := RecordFrom(struct {
		At time.Time `qb:"8"`
	}{time.Now()}); !errors.As(err, &plain) {
		t.Errorf("a time.Time: err = %v, want PlainTimeError", err)
	}
}
