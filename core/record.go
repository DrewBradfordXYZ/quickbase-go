package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FieldValue is one field's value in a record, as QuickBase sends and takes
// it: {"value": ...}. Value is a plain Go value: string, float64, bool,
// []any, map[string]any, or nil; and, to send, Date, DateTime or anything
// that marshals to the field's form.
type FieldValue struct {
	Value any `json:"value"`
}

// Record is a record as QuickBase sends and takes it: field ID ("6"), or an
// alias with a schema, to its value.
//
//	rec["6"].Value            // one value
//	rec.Values()              // map[string]any
//	rec.Decode(&project)      // into a struct with qb:"<id>" tags
type Record map[string]FieldValue

// Values returns the record as field ID (or alias) to plain value.
func (r Record) Values() map[string]any {
	values := make(map[string]any, len(r))
	for k, v := range r {
		values[k] = v.Value
	}
	return values
}

// User is a user field's value.
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	UserName string `json:"userName,omitempty"`
}

// Decode sets the fields of the struct v points to from the record. A
// struct field is matched by its qb tag, the field ID (`qb:"6"`) or alias;
// untagged fields are left alone. A blank value ("" or null) leaves the
// field its zero value. Otherwise the value converts as JSON would: text to
// string, numbers to int or float64 (or a pointer to one, nil when blank),
// checkbox to bool, multi-select text to []string, a date to Date, a
// date/time to DateTime, a user to User or *User.
func (r Record) Decode(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("quickbase: Decode needs a pointer to a struct, got %T", v)
	}
	target := rv.Elem()
	for _, f := range fieldsOf(target.Type()) {
		value, ok := r[f.key]
		if !ok || blank(value.Value) {
			continue
		}
		raw, err := json.Marshal(value.Value)
		if err != nil {
			return fmt.Errorf("quickbase: field %s: %w", f.key, err)
		}
		dest := target.FieldByIndex(f.index)
		if err := json.Unmarshal(raw, dest.Addr().Interface()); err != nil {
			return fmt.Errorf("quickbase: field %s (%s %s): %w", f.key, target.Type().Name(), f.name, err)
		}
	}
	return nil
}

// DecodeRecords decodes each record into a new T.
func DecodeRecords[T any](records []Record) ([]T, error) {
	out := make([]T, len(records))
	for i, r := range records {
		if err := r.Decode(&out[i]); err != nil {
			return nil, fmt.Errorf("record %d: %w", i+1, err)
		}
	}
	return out, nil
}

// RecordFrom builds a record from a struct with qb tags, to send. A nil
// pointer field is left out, so a struct of pointers says which fields to
// change; other fields are always sent. A time.Time is refused (see
// PlainTimeError): use Date or DateTime.
func RecordFrom(v any) (Record, error) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, fmt.Errorf("quickbase: RecordFrom got a nil %T", v)
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("quickbase: RecordFrom needs a struct, got %T", v)
	}
	record := Record{}
	for _, f := range fieldsOf(rv.Type()) {
		field := rv.FieldByIndex(f.index)
		if field.Kind() == reflect.Pointer {
			if field.IsNil() {
				continue
			}
			field = field.Elem()
		}
		value := field.Interface()
		if _, isTime := value.(time.Time); isTime {
			return nil, &PlainTimeError{Field: f.key, Record: 1}
		}
		record[f.key] = FieldValue{Value: value}
	}
	return record, nil
}

// blank reports whether a value is QuickBase's blank: null, or "" (text,
// dates, times of day).
func blank(v any) bool {
	return v == nil || v == ""
}

type taggedField struct {
	key   string // field ID or alias
	name  string // Go field name
	index []int
}

var fieldPlans sync.Map // reflect.Type -> []taggedField

// fieldsOf returns a struct type's qb-tagged fields, cached.
func fieldsOf(t reflect.Type) []taggedField {
	if plan, ok := fieldPlans.Load(t); ok {
		return plan.([]taggedField)
	}
	var fields []taggedField
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag, ok := sf.Tag.Lookup("qb")
		if !ok || !sf.IsExported() {
			continue
		}
		key, _, _ := strings.Cut(tag, ",")
		if key == "" || key == "-" {
			continue
		}
		if _, err := strconv.Atoi(key); err != nil && strings.ContainsAny(key, " \t") {
			continue
		}
		fields = append(fields, taggedField{key: key, name: sf.Name, index: sf.Index})
	}
	fieldPlans.Store(t, fields)
	return fields
}
