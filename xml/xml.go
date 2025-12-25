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
//	    "github.com/DrewBradfordXYZ/quickbase-go/v2"
//	    "github.com/DrewBradfordXYZ/quickbase-go/v2/xml"
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
// Role Management:
//   - [Client.GetRoleInfo]: Get all roles defined in an application
//   - [Client.UserRoles]: Get all users and their role assignments
//   - [Client.GetUserRole]: Get roles for a specific user
//   - [Client.AddUserToRole]: Assign a user to a role
//   - [Client.RemoveUserFromRole]: Remove a user from a role
//   - [Client.ChangeUserRole]: Change a user's role or disable access
//
// Group Management:
//   - [Client.CreateGroup]: Create a new group
//   - [Client.DeleteGroup]: Delete a group
//   - [Client.GetUsersInGroup]: Get users and managers in a group
//   - [Client.AddUserToGroup]: Add a user to a group
//   - [Client.RemoveUserFromGroup]: Remove a user from a group
//   - [Client.GetGroupRole]: Get roles assigned to a group
//   - [Client.AddGroupToRole]: Assign a group to a role
//   - [Client.RemoveGroupFromRole]: Remove a group from a role
//
// User Management:
//   - [Client.GetUserInfo]: Get user info by email address
//   - [Client.ProvisionUser]: Create a new unregistered user and assign to role
//   - [Client.SendInvitation]: Send invitation email to a user
//   - [Client.ChangeManager]: Change the app manager
//   - [Client.ChangeRecordOwner]: Change the owner of a record
//
// Application Variables:
//   - [Client.GetDBVar]: Get an application variable value
//   - [Client.SetDBVar]: Set an application variable value
//
// Code Pages:
//   - [Client.GetDBPage]: Get stored code page content
//   - [Client.AddReplaceDBPage]: Create or update a code page
//
// Field Management:
//   - [Client.FieldAddChoices]: Add choices to a multiple-choice field
//   - [Client.FieldRemoveChoices]: Remove choices from a multiple-choice field
//   - [Client.SetKeyField]: Set the key field for a table
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

	"github.com/DrewBradfordXYZ/quickbase-go/v2/core"
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
	schema *core.ResolvedSchema
}

// Option configures the XML client.
type Option func(*Client)

// WithSchema configures the XML client to use schema aliases.
// When provided, table and field aliases can be used in method parameters,
// and result types gain helper methods for accessing data by alias.
//
// Example:
//
//	schema := core.NewSchema().
//	    Table("projects", "bqxyz123").
//	        Field("id", 3).
//	        Field("name", 6).
//	        Field("status", 7).
//	    Build()
//
//	xmlClient := xml.New(qb, xml.WithSchema(core.ResolveSchema(schema)))
//
//	// Use table alias
//	result, _ := xmlClient.GetRecordInfo(ctx, "projects", 123)
//
//	// Access fields by alias
//	fmt.Println(result.Field("name").Value)
func WithSchema(schema *core.ResolvedSchema) Option {
	return func(c *Client) {
		c.schema = schema
	}
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
//
//	// With schema for alias support:
//	xmlClient := xml.New(qb, xml.WithSchema(resolvedSchema))
func New(caller Caller, opts ...Option) *Client {
	c := &Client{caller: caller}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// resolveTable resolves a table alias to its ID.
// If no schema is configured or the input is already an ID, returns the input unchanged.
func (c *Client) resolveTable(tableRef string) string {
	if c.schema == nil {
		return tableRef
	}
	resolved, err := core.ResolveTableAlias(c.schema, tableRef)
	if err != nil {
		// If resolution fails, return original (might be a raw ID)
		return tableRef
	}
	return resolved
}

// resolveField resolves a field alias to its ID for a given table.
// If no schema is configured or the input is already an ID, returns the input unchanged.
func (c *Client) resolveField(tableID string, fieldRef any) int {
	if c.schema == nil {
		if id, ok := fieldRef.(int); ok {
			return id
		}
		return 0
	}
	resolved, err := core.ResolveFieldAlias(c.schema, tableID, fieldRef)
	if err != nil {
		if id, ok := fieldRef.(int); ok {
			return id
		}
		return 0
	}
	return resolved
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
