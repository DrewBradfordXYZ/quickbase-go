package quickbase_test

import (
	"encoding/json"
	"testing"

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
