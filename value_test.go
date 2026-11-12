package quickbase_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v2"
)

// Value sent every number as float32, which holds 7 significant digits:
// 1234567.89 was written as 1234567.875 and record ID 20000001 as 20000000.
func TestValueKeepsNumberPrecision(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{1234567.89, `{"value":1234567.89}`},
		{20000001, `{"value":20000001}`},
		{int64(123456789012), `{"value":123456789012}`},
	}
	for _, c := range cases {
		got, err := json.Marshal(quickbase.Value(c.in))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != c.want {
			t.Errorf("Value(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

// Value sent null, which clears the field, for every type it didn't list,
// time.Time included.
func TestValueDatesAndOtherTypes(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{quickbase.DateOf(time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC)), `{"value":"2024-01-15"}`},
		{quickbase.DateTimeOf(time.Date(2024, 1, 15, 15, 30, 0, 0, time.UTC)), `{"value":"2024-01-15T15:30:00Z"}`},
		{nil, `{"value":null}`},
		{uint(7), `{"value":7}`},
		{map[string]any{"fileName": "a.txt", "data": "aGk="}, `{"value":{"data":"aGk=","fileName":"a.txt"}}`},
	}
	for _, c := range cases {
		got, err := json.Marshal(quickbase.Value(c.in))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != c.want {
			t.Errorf("Value(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestValuePanicsOnTime(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "DateOf") {
			t.Errorf("recover() = %v, want a panic naming DateOf", r)
		}
	}()
	quickbase.Value(time.Now())
}
