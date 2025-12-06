// Package xml provides access to legacy QuickBase XML API endpoints.
//
// The XML API contains endpoints that have no JSON API equivalent, primarily
// for retrieving role and schema information. This package wraps those endpoints
// while reusing the main client's authentication, retry, and throttling infrastructure.
//
// # Rate Limits
//
// The XML API has different rate limits than the JSON API:
//   - XML API: 10 requests per second per table (dynamically enforced)
//   - JSON API: 100 requests per 10 seconds per user token
//
// The SDK uses retry logic with exponential backoff for 429 responses.
// Note that the proactive throttle (WithProactiveThrottle) is designed for
// the JSON API and doesn't account for the XML API's per-table limits.
//
// # Deprecation Notice
//
// The QuickBase XML API is legacy and may be discontinued in the future.
// Use JSON API methods (via the main quickbase package) where possible.
// This package will be removed when QuickBase discontinues the XML API.
//
// To find all XML API-related code for removal, search for: grep -r "XML-API"
//
// # Usage
//
// Create an XML client from an existing quickbase.Client:
//
//	import (
//	    "github.com/DrewBradfordXYZ/quickbase-go"
//	    "github.com/DrewBradfordXYZ/quickbase-go/xml"
//	)
//
//	// Main client for JSON API
//	qb, _ := quickbase.New("myrealm", quickbase.WithUserToken("..."))
//
//	// XML client for legacy endpoints
//	xmlClient := xml.New(qb)
//
//	// Get all roles defined in an app
//	roles, err := xmlClient.GetRoleInfo(ctx, appId)
//
//	// Get comprehensive schema information
//	schema, err := xmlClient.GetSchema(ctx, tableId)
//
// # Available Endpoints
//
// App Discovery:
//   - [Client.GrantedDBs]: List all apps/tables the user can access
//   - [Client.FindDBByName]: Find an app by its name
//   - [Client.GetDBInfo]: Get app/table metadata (record count, manager, timestamps)
//   - [Client.GetNumRecords]: Get total record count for a table
//
// Role Information:
//   - [Client.GetRoleInfo]: Get all roles defined in an application
//   - [Client.UserRoles]: Get all users and their role assignments
//   - [Client.GetUserRole]: Get roles for a specific user
//   - [Client.AddUserToRole]: Assign a user to a role
//   - [Client.RemoveUserFromRole]: Remove a user from a role
//   - [Client.ChangeUserRole]: Change a user's role or disable access
//
// User Information:
//   - [Client.GetUserInfo]: Get user info by email address
//
// Application Variables:
//   - [Client.GetDBVar]: Get an application variable value
//   - [Client.SetDBVar]: Set an application variable value
//
// Schema Information:
//   - [Client.GetSchema]: Get comprehensive app/table metadata including fields, reports, and variables
//
// Record Information:
//   - [Client.DoQueryCount]: Get count of matching records without fetching data
//   - [Client.GetRecordInfo]: Get a single record with full field metadata (name, type, value, printable)
//   - [Client.GetRecordInfoByKey]: Get a record by key field value instead of record ID
package xml

import (
	"bytes"
	"context"
	"encoding/xml"
)

// Caller defines the minimal interface required to make XML API calls.
// This is implemented by *client.Client (and transitively by *quickbase.Client).
//
// By depending on this interface rather than concrete types, the xml package:
//   - Avoids import cycles with the main package
//   - Remains easily testable with mock implementations
//   - Can be cleanly removed when XML API is deprecated
type Caller interface {
	// Realm returns the QuickBase realm name (e.g., "mycompany").
	Realm() string

	// DoXML makes an XML API request and returns the raw response body.
	// The action parameter specifies the QUICKBASE-ACTION header value.
	DoXML(ctx context.Context, dbid, action string, body []byte) ([]byte, error)
}

// Client provides methods for calling legacy QuickBase XML API endpoints.
//
// Create a Client using [New] with an existing quickbase.Client or client.Client.
type Client struct {
	caller Caller
}

// New creates an XML API client from an existing QuickBase client.
//
// The caller parameter should be a *quickbase.Client or *client.Client,
// both of which implement the [Caller] interface.
//
// Example:
//
//	qb, _ := quickbase.New("myrealm", quickbase.WithUserToken("..."))
//	xmlClient := xml.New(qb)
func New(caller Caller) *Client {
	return &Client{caller: caller}
}

// buildRequest creates an XML request body with the given inner content.
// It wraps the content in <qdbapi> tags.
func buildRequest(inner string) []byte {
	return []byte("<qdbapi>" + inner + "</qdbapi>")
}

// xmlEscape escapes special XML characters in a string.
// Returns empty string if escaping fails (invalid characters).
func xmlEscape(s string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return ""
	}
	return buf.String()
}
