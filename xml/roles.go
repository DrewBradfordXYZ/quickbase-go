package xml

import (
	"context"
	"encoding/xml"
	"fmt"
)

// Role represents a role defined in a QuickBase application.
type Role struct {
	// ID is the unique identifier for the role
	ID int `xml:"id,attr"`

	// Name is the display name of the role
	Name string `xml:"name"`

	// Access describes the access level (e.g., "Basic Access", "Administrator")
	Access RoleAccess `xml:"access"`
}

// RoleAccess represents the access level of a role.
type RoleAccess struct {
	// ID is the access level ID:
	//   1 = Administrator
	//   2 = Basic Access with Share
	//   3 = Basic Access
	ID int `xml:"id,attr"`

	// Description is the text description of the access level
	Description string `xml:",chardata"`
}

// GetRoleInfoResult contains the response from API_GetRoleInfo.
type GetRoleInfoResult struct {
	// Roles is the list of all roles defined in the application
	Roles []Role
}

// getRoleInfoResponse is the XML response structure for API_GetRoleInfo.
type getRoleInfoResponse struct {
	BaseResponse
	Roles []Role `xml:"roles>role"`
}

// GetRoleInfo returns all roles defined in a QuickBase application.
//
// This calls the legacy API_GetRoleInfo XML endpoint. The appId should be
// the application-level dbid (not a table dbid).
//
// Example:
//
//	roles, err := xmlClient.GetRoleInfo(ctx, "bqxyz123")
//	for _, role := range roles.Roles {
//	    fmt.Printf("Role %d: %s (%s)\n", role.ID, role.Name, role.Access.Description)
//	}
//
// See: https://help.quickbase.com/docs/api-getroleinfo
func (c *Client) GetRoleInfo(ctx context.Context, appId string) (*GetRoleInfoResult, error) {
	body := buildRequest("")

	respBody, err := c.caller.DoXML(ctx, appId, "API_GetRoleInfo", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetRoleInfo: %w", err)
	}

	var resp getRoleInfoResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetRoleInfo response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &GetRoleInfoResult{
		Roles: resp.Roles,
	}, nil
}

// UserWithRoles represents a user and their assigned roles.
type UserWithRoles struct {
	// ID is the user's QuickBase user ID (e.g., "112149.bhsv")
	ID string `xml:"id,attr"`

	// Type is "user" for individual users or "group" for groups
	Type string `xml:"type,attr"`

	// Name is the user's display name
	Name string `xml:"name"`

	// FirstName is the user's first name (may be empty for groups)
	FirstName string `xml:"firstName"`

	// LastName is the user's last name (may be empty for groups)
	LastName string `xml:"lastName"`

	// LastAccess is the timestamp of the user's last access (milliseconds since epoch)
	LastAccess string `xml:"lastAccess"`

	// LastAccessAppLocal is the human-readable last access time
	LastAccessAppLocal string `xml:"lastAccessAppLocal"`

	// Roles is the list of roles assigned to this user
	Roles []Role `xml:"roles>role"`
}

// UserRolesResult contains the response from API_UserRoles.
type UserRolesResult struct {
	// Users is the list of all users and their role assignments
	Users []UserWithRoles
}

// userRolesResponse is the XML response structure for API_UserRoles.
type userRolesResponse struct {
	BaseResponse
	Users []UserWithRoles `xml:"users>user"`
}

// UserRoles returns all users in an application and their role assignments.
//
// This calls the legacy API_UserRoles XML endpoint. The appId should be
// the application-level dbid. You must have Basic Access with Sharing or
// Full Administration access to use this call.
//
// Example:
//
//	result, err := xmlClient.UserRoles(ctx, "bqxyz123")
//	for _, user := range result.Users {
//	    fmt.Printf("%s (%s):\n", user.Name, user.ID)
//	    for _, role := range user.Roles {
//	        fmt.Printf("  - %s\n", role.Name)
//	    }
//	}
//
// See: https://help.quickbase.com/docs/api-userroles
func (c *Client) UserRoles(ctx context.Context, appId string) (*UserRolesResult, error) {
	body := buildRequest("")

	respBody, err := c.caller.DoXML(ctx, appId, "API_UserRoles", body)
	if err != nil {
		return nil, fmt.Errorf("API_UserRoles: %w", err)
	}

	var resp userRolesResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_UserRoles response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &UserRolesResult{
		Users: resp.Users,
	}, nil
}

// RoleMember describes how a role was assigned (directly or via group).
type RoleMember struct {
	// Type is "user", "group", or "domainGroup"
	Type string `xml:"type,attr"`

	// Name is the display name of the member
	Name string `xml:",chardata"`
}

// UserRole represents a role assigned to a user, with membership info.
type UserRole struct {
	// ID is the role ID
	ID int `xml:"id,attr"`

	// Name is the role name
	Name string `xml:"name"`

	// Access is the access level
	Access RoleAccess `xml:"access"`

	// Member describes how this role was assigned (only present if inclgrps=1)
	Member *RoleMember `xml:"member"`
}

// GetUserRoleResult contains the response from API_GetUserRole.
type GetUserRoleResult struct {
	// UserID is the user's QuickBase user ID
	UserID string

	// UserName is the user's display name
	UserName string

	// Roles is the list of roles assigned to this user
	Roles []UserRole
}

// getUserRoleResponse is the XML response structure for API_GetUserRole.
type getUserRoleResponse struct {
	BaseResponse
	User struct {
		ID    string     `xml:"id,attr"`
		Name  string     `xml:"name"`
		Roles []UserRole `xml:"roles>role"`
	} `xml:"user"`
}

// GetUserRole returns the roles assigned to a specific user.
//
// This calls the legacy API_GetUserRole XML endpoint. The appId should be
// the application-level dbid. The userId should be the user's QuickBase ID
// (e.g., "112149.bhsv").
//
// If includeGroups is true, the response will include roles assigned via
// groups, with the Member field populated to indicate how the role was assigned.
//
// Example:
//
//	result, err := xmlClient.GetUserRole(ctx, "bqxyz123", "112149.bhsv", true)
//	fmt.Printf("User: %s\n", result.UserName)
//	for _, role := range result.Roles {
//	    fmt.Printf("  Role: %s", role.Name)
//	    if role.Member != nil {
//	        fmt.Printf(" (via %s: %s)", role.Member.Type, role.Member.Name)
//	    }
//	    fmt.Println()
//	}
//
// See: https://help.quickbase.com/docs/api-getuserrole
func (c *Client) GetUserRole(ctx context.Context, appId, userId string, includeGroups bool) (*GetUserRoleResult, error) {
	inner := ""
	if userId != "" {
		inner += "<userid>" + userId + "</userid>"
	}
	if includeGroups {
		inner += "<inclgrps>1</inclgrps>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_GetUserRole", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetUserRole: %w", err)
	}

	var resp getUserRoleResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetUserRole response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &GetUserRoleResult{
		UserID:   resp.User.ID,
		UserName: resp.User.Name,
		Roles:    resp.User.Roles,
	}, nil
}

// AddUserToRole assigns a user to a role in the application.
//
// You can call this multiple times to give a user multiple roles.
// After assigning, use SendInvitation to invite the user to the app.
//
// Requires Basic Access with Sharing or Full Administration access.
// Users with Basic Access cannot add users to admin roles.
//
// Example:
//
//	// Get user ID first
//	user, _ := xmlClient.GetUserInfo(ctx, "user@example.com")
//
//	// Assign them to role 10
//	err := xmlClient.AddUserToRole(ctx, appId, user.ID, 10)
//
// See: https://help.quickbase.com/docs/api-addusertorole
func (c *Client) AddUserToRole(ctx context.Context, appId, userId string, roleId int) error {
	inner := "<userid>" + xmlEscape(userId) + "</userid>"
	inner += fmt.Sprintf("<roleid>%d</roleid>", roleId)
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_AddUserToRole", body)
	if err != nil {
		return fmt.Errorf("API_AddUserToRole: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_AddUserToRole response: %w", err)
	}

	return checkError(&resp)
}

// RemoveUserFromRole removes a user from a specific role.
//
// If this is the user's only role, they lose all access to the app
// and are removed from the app's user list.
//
// To temporarily disable access while keeping the user in the app,
// use ChangeUserRole with newRoleId=0 instead.
//
// Example:
//
//	err := xmlClient.RemoveUserFromRole(ctx, appId, "112149.bhsv", 10)
//
// See: https://help.quickbase.com/docs/api-removeuserfromrole
func (c *Client) RemoveUserFromRole(ctx context.Context, appId, userId string, roleId int) error {
	inner := "<userid>" + xmlEscape(userId) + "</userid>"
	inner += fmt.Sprintf("<roleid>%d</roleid>", roleId)
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_RemoveUserFromRole", body)
	if err != nil {
		return fmt.Errorf("API_RemoveUserFromRole: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_RemoveUserFromRole response: %w", err)
	}

	return checkError(&resp)
}

// ChangeUserRole changes a user's role or disables their access.
//
// This is preferred over RemoveUserFromRole when you want to keep the
// user in the app's user list but change/disable their access.
//
// Pass newRoleId=0 to set the role to "None" (role ID 9), which disables
// access while keeping the user on the app's user list for future reinstatement.
//
// Example:
//
//	// Change user from role 10 to role 11
//	err := xmlClient.ChangeUserRole(ctx, appId, "112149.bhsv", 10, 11)
//
//	// Disable access (set to None role)
//	err := xmlClient.ChangeUserRole(ctx, appId, "112149.bhsv", 10, 0)
//
// See: https://help.quickbase.com/docs/api-changeuserrole
func (c *Client) ChangeUserRole(ctx context.Context, appId, userId string, currentRoleId, newRoleId int) error {
	inner := "<userid>" + xmlEscape(userId) + "</userid>"
	inner += fmt.Sprintf("<roleid>%d</roleid>", currentRoleId)
	if newRoleId > 0 {
		inner += fmt.Sprintf("<newRoleid>%d</newRoleid>", newRoleId)
	}
	// If newRoleId is 0, omit it and QuickBase sets role to None (9)
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_ChangeUserRole", body)
	if err != nil {
		return fmt.Errorf("API_ChangeUserRole: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_ChangeUserRole response: %w", err)
	}

	return checkError(&resp)
}
