package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// QuickBase has two kinds of date field, and the JSON API accepts neither's
// values for the other (checked against a live realm, 2026-11-12):
//
//   - A Date field holds a calendar day. It returns "2024-01-15" and accepts
//     only that form: a timestamp, US format or epoch milliseconds is rejected.
//   - A Date/Time field holds a moment. It returns UTC ("2024-01-15T15:30:00Z")
//     and accepts RFC 3339 with Z or an offset. A bare date is rejected.
//
// A rejected value doesn't fail the upsert: QuickBase saves the record with
// the field blank and lists the line in metadata.lineErrors. So a value says
// which kind it is: [Date] or [DateTime]. A time.Time could be either, and
// isn't sent ([CheckRecordData]).

// Date is a Date field's value: a calendar day, with no time or time zone. It
// marshals as "2024-01-15", the only form a Date field accepts.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns t's calendar day in t's location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// ParseDate parses a Date field's value as QuickBase sends it ("2024-01-15").
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("not a QuickBase date (YYYY-MM-DD): %q", s)
	}
	return DateOf(t), nil
}

// IsValid reports whether d is a day that exists (not 2024-02-30).
func (d Date) IsValid() bool {
	return DateOf(time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)) == d
}

// In returns midnight at the start of d in loc.
func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// String returns the day as QuickBase reads and writes it: "2024-01-15".
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// MarshalJSON writes "2024-01-15". A day that doesn't exist is an error, so
// it isn't sent.
func (d Date) MarshalJSON() ([]byte, error) {
	if !d.IsValid() {
		return nil, fmt.Errorf("no such day: %s", d)
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON reads "2024-01-15".
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// DateTime is a Date/Time field's value: a moment. It marshals as RFC 3339 in
// UTC, which a Date/Time field accepts (QuickBase keeps whole seconds).
type DateTime struct {
	time.Time
}

// DateTimeOf returns t as a Date/Time value.
func DateTimeOf(t time.Time) DateTime {
	return DateTime{Time: t}
}

// ParseDateTime parses a Date/Time field's value as QuickBase sends it
// ("2024-01-15T15:30:00Z").
func ParseDateTime(s string) (DateTime, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return DateTime{}, fmt.Errorf("not a QuickBase date/time (RFC 3339): %q", s)
	}
	return DateTime{Time: t}, nil
}

// String returns the moment in UTC, RFC 3339: "2024-01-15T15:30:00Z".
func (t DateTime) String() string {
	return t.UTC().Format(time.RFC3339Nano)
}

// MarshalJSON writes the moment in UTC, RFC 3339.
func (t DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON reads RFC 3339.
func (t *DateTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := ParseDateTime(s)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// PlainTimeError reports a time.Time in record data. Wrap it with [DateOf]
// for a Date field or [DateTimeOf] for a Date/Time field.
type PlainTimeError struct {
	Field  string // field ID or alias, as given
	Record int    // 1-based
}

func (e *PlainTimeError) Error() string {
	return fmt.Sprintf("field %q in record %d is a time.Time, which QuickBase can store only once it knows "+
		"the field type: use quickbase.DateOf(t) for a Date field or quickbase.DateTimeOf(t) for a Date/Time field",
		e.Field, e.Record)
}

var timeType = reflect.TypeOf(time.Time{})

// CheckRecordData returns a [PlainTimeError] for the first time.Time in
// upsert data: records as maps of field to value or {"value": value}. Upsert
// calls it before sending, since a time.Time sent to a Date field would leave
// the field blank on a record QuickBase still saves.
func CheckRecordData(data any) error {
	records := reflect.ValueOf(data)
	if records.Kind() != reflect.Slice && records.Kind() != reflect.Array {
		return nil
	}
	for i := 0; i < records.Len(); i++ {
		record := indirect(records.Index(i))
		if record.Kind() != reflect.Map {
			continue
		}
		iter := record.MapRange()
		for iter.Next() {
			if hasPlainTime(iter.Value(), 0) {
				return &PlainTimeError{Field: fmt.Sprint(iter.Key().Interface()), Record: i + 1}
			}
		}
	}
	return nil
}

// hasPlainTime reports whether v is a time.Time, or holds one as a field
// value ({"value": t} or a struct with a Value field).
func hasPlainTime(v reflect.Value, depth int) bool {
	v = indirect(v)
	if !v.IsValid() || depth > 2 {
		return false
	}
	if v.Type() == timeType {
		return true
	}
	switch v.Kind() {
	case reflect.Map:
		if v.Type().Key().Kind() == reflect.String {
			if inner := v.MapIndex(reflect.ValueOf("value").Convert(v.Type().Key())); inner.IsValid() {
				return hasPlainTime(inner, depth+1)
			}
		}
	case reflect.Struct:
		if f := v.FieldByName("Value"); f.IsValid() && f.CanInterface() {
			return hasPlainTime(f, depth+1)
		}
	}
	return false
}

// indirect follows interfaces and pointers.
func indirect(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}
