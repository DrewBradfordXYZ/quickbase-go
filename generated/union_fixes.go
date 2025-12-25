package generated

// MarshalJSON implements json.Marshaler for RunQueryJSONBody_Where.
// The union type can contain either a string (query) or []int (record IDs).
func (t RunQueryJSONBody_Where) MarshalJSON() ([]byte, error) {
	return t.union.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler for RunQueryJSONBody_Where.
// The union type can contain either a string (query) or []int (record IDs).
func (t *RunQueryJSONBody_Where) UnmarshalJSON(b []byte) error {
	return t.union.UnmarshalJSON(b)
}

// MarshalJSON implements json.Marshaler for DeleteRecordsJSONBody_Where.
// The union type can contain either a string (query) or []int (record IDs).
func (t DeleteRecordsJSONBody_Where) MarshalJSON() ([]byte, error) {
	return t.union.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler for DeleteRecordsJSONBody_Where.
// The union type can contain either a string (query) or []int (record IDs).
func (t *DeleteRecordsJSONBody_Where) UnmarshalJSON(b []byte) error {
	return t.union.UnmarshalJSON(b)
}
