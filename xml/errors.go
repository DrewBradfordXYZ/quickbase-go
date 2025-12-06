package xml

import (
	"encoding/xml"
	"fmt"
)

// Error represents an error returned by the QuickBase XML API.
type Error struct {
	// Code is the QuickBase error code (e.g., 4 for "unauthorized")
	Code int

	// Text is the error text (e.g., "No error", "User not authorized")
	Text string

	// Detail provides additional context when available
	Detail string

	// Action is the API action that was called (e.g., "API_GetRoleInfo")
	Action string
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("XML API error %d: %s", e.Code, e.Text)
	if e.Detail != "" {
		msg += " (" + e.Detail + ")"
	}
	return msg
}

// BaseResponse contains the common fields in all XML API responses.
type BaseResponse struct {
	XMLName   xml.Name `xml:"qdbapi"`
	Action    string   `xml:"action"`
	ErrCode   int      `xml:"errcode"`
	ErrText   string   `xml:"errtext"`
	ErrDetail string   `xml:"errdetail"`
}

// checkError checks if the response contains an error and returns it.
// Returns nil if errcode is 0 (success).
func checkError(resp *BaseResponse) error {
	if resp.ErrCode == 0 {
		return nil
	}
	return &Error{
		Code:   resp.ErrCode,
		Text:   resp.ErrText,
		Detail: resp.ErrDetail,
		Action: resp.Action,
	}
}

// Common QuickBase XML API error codes
const (
	// ErrCodeSuccess indicates no error
	ErrCodeSuccess = 0

	// ErrCodeUnauthorized indicates the user is not authorized for this action
	ErrCodeUnauthorized = 4

	// ErrCodeInvalidInput indicates invalid input parameters
	ErrCodeInvalidInput = 5

	// ErrCodeNoSuchDatabase indicates the database/table does not exist
	ErrCodeNoSuchDatabase = 6

	// ErrCodeAccessDenied indicates access to the resource is denied
	ErrCodeAccessDenied = 7

	// ErrCodeInvalidTicket indicates the authentication ticket is invalid or expired
	ErrCodeInvalidTicket = 8

	// ErrCodeNoSuchRecord indicates the record does not exist
	ErrCodeNoSuchRecord = 30

	// ErrCodeNoSuchField indicates the field does not exist
	ErrCodeNoSuchField = 31

	// ErrCodeNoSuchUser indicates the user does not exist
	ErrCodeNoSuchUser = 33
)

// IsUnauthorized returns true if the error is an authorization error.
func IsUnauthorized(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == ErrCodeUnauthorized || e.Code == ErrCodeAccessDenied
	}
	return false
}

// IsNotFound returns true if the error indicates a resource was not found.
func IsNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == ErrCodeNoSuchDatabase ||
			e.Code == ErrCodeNoSuchRecord ||
			e.Code == ErrCodeNoSuchField ||
			e.Code == ErrCodeNoSuchUser
	}
	return false
}

// IsInvalidTicket returns true if the error indicates an invalid or expired ticket.
func IsInvalidTicket(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == ErrCodeInvalidTicket
	}
	return false
}
