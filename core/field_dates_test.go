package core

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestDate(t *testing.T) {
	d, err := ParseDate("2024-01-15")
	if err != nil {
		t.Fatal(err)
	}
	if d != (Date{2024, time.January, 15}) || d.String() != "2024-01-15" {
		t.Errorf("ParseDate = %+v (%s)", d, d)
	}
	b, _ := json.Marshal(map[string]any{"value": d})
	if string(b) != `{"value":"2024-01-15"}` {
		t.Errorf("marshal = %s", b)
	}
	var back Date
	if err := json.Unmarshal([]byte(`"2024-02-29"`), &back); err != nil || back.String() != "2024-02-29" {
		t.Errorf("unmarshal = %v, %v", back, err)
	}
	for _, bad := range []string{"2024-01-15T00:00:00Z", "01-15-2024", "2024-02-30"} {
		if _, err := ParseDate(bad); err == nil {
			t.Errorf("ParseDate(%q) accepted", bad)
		}
	}
	if _, err := json.Marshal(Date{2024, time.February, 30}); err == nil {
		t.Error("marshalled a day that doesn't exist")
	}
}

// DateOf takes the day in t's own location: 23:30 on the 15th in New York
// is the 16th in UTC, and stays the 15th.
func TestDateOfUsesTheTimesLocation(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	late := time.Date(2024, time.January, 15, 23, 30, 0, 0, ny)
	if got := DateOf(late).String(); got != "2024-01-15" {
		t.Errorf("DateOf = %s", got)
	}
	if got := DateOf(late.UTC()).String(); got != "2024-01-16" {
		t.Errorf("DateOf(UTC) = %s", got)
	}
}

func TestDateTime(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	dt := DateTimeOf(time.Date(2024, time.January, 15, 10, 30, 0, 0, ny))
	b, _ := json.Marshal(dt)
	if string(b) != `"2024-01-15T15:30:00Z"` {
		t.Errorf("marshal = %s, want UTC", b)
	}
	parsed, err := ParseDateTime("2024-01-15T15:30:00Z")
	if err != nil || !parsed.Equal(dt.Time) {
		t.Errorf("ParseDateTime = %v, %v", parsed, err)
	}
	if _, err := ParseDateTime("2024-01-15"); err == nil {
		t.Error("ParseDateTime accepted a bare date")
	}
}

func TestCheckRecordData(t *testing.T) {
	now := time.Now()
	ok := []any{
		map[string]any{"6": "x", "7": DateOf(now), "8": DateTimeOf(now)},
		map[string]any{"7": map[string]any{"value": DateOf(now)}},
	}
	if err := CheckRecordData(ok); err != nil {
		t.Errorf("wrapped dates refused: %v", err)
	}

	for name, data := range map[string][]any{
		"bare":    {map[string]any{"6": "x"}, map[string]any{"due": now}},
		"wrapped": {map[string]any{"6": "x"}, map[string]any{"due": map[string]any{"value": now}}},
		"pointer": {map[string]any{"6": "x"}, map[string]any{"due": &now}},
	} {
		var pt *PlainTimeError
		if err := CheckRecordData(data); !errors.As(err, &pt) || pt.Field != "due" || pt.Record != 2 {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// QuickBase sends a blank date as "" (and null); inside a result type that
// failed the whole response.
func TestBlankDatesDecodeAsZero(t *testing.T) {
	var v struct {
		D  Date     `json:"d"`
		DT DateTime `json:"dt"`
		N  Date     `json:"n"`
	}
	if err := json.Unmarshal([]byte(`{"d":"","dt":"","n":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.D != (Date{}) || !v.DT.IsZero() || v.N != (Date{}) {
		t.Errorf("blanks decoded as %+v", v)
	}
}
