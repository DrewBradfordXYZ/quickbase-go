package client

// --- Record Transformation ---
// These functions transform records from generated types to friendly user-facing types.

// Deref returns the value of a pointer, or the zero value if it is nil: for
// the optional fields of request types, which stay pointers.
func Deref[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// DerefOr returns the value of a pointer, or def if it is nil.
func DerefOr[T any](ptr *T, def T) T {
	if ptr == nil {
		return def
	}
	return *ptr
}
