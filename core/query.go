package core

import (
	"encoding/json"
	"strconv"
	"strings"
)

// SortOrder is a sort direction.
type SortOrder string

const (
	ASC         SortOrder = "ASC"
	DESC        SortOrder = "DESC"
	EqualValues SortOrder = "equal-values" // group equal values without ordering them
)

// SortField is a field to sort by.
type SortField struct {
	FieldId int       `json:"fieldId"`
	Order   SortOrder `json:"order"`
}

// Asc sorts by a field, ascending.
func Asc(fieldID int) SortField { return SortField{FieldId: fieldID, Order: ASC} }

// Desc sorts by a field, descending.
func Desc(fieldID int) SortField { return SortField{FieldId: fieldID, Order: DESC} }

// SortBy is a query's sort order: SortBy{Asc(6), Desc(3)}. Left unset (nil),
// QuickBase uses the table's default sort; NoSort turns sorting off, which
// is faster for large queries.
type SortBy []SortField

// NoSort turns sorting off: QuickBase returns records unsorted.
var NoSort = SortBy{}

// MarshalJSON writes the fields, or false for NoSort.
func (s SortBy) MarshalJSON() ([]byte, error) {
	if s != nil && len(s) == 0 {
		return []byte("false"), nil
	}
	return json.Marshal([]SortField(s))
}

// UnmarshalJSON reads the fields, or false as NoSort.
func (s *SortBy) UnmarshalJSON(b []byte) error {
	if string(b) == "false" {
		*s = SortBy{}
		return nil
	}
	var fields []SortField
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	*s = fields
	return nil
}

// RecordIDs is a query for these records by Record ID#: {3.EX.12}OR{3.EX.15}.
// QuickBase also takes a where clause as a list of record IDs; this is the
// same query as text, so Where is always a string.
func RecordIDs(ids ...int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "{3.EX." + strconv.Itoa(id) + "}"
	}
	return strings.Join(parts, "OR")
}
