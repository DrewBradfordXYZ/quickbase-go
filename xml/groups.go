package xml

import (
	"context"
	"encoding/xml"
	"fmt"
)

// Group represents a QuickBase group.
type Group struct {
	// ID is the group ID (e.g., "1217.dgpt")
	ID string `xml:"id,attr"`

	// Name is the group name
	Name string `xml:"name"`

	// Description is the group description
	Description string `xml:"description"`

	// ManagedByUser indicates if the group is managed by the user
	ManagedByUser bool `xml:"managedByUser"`
}

// CreateGroupResult contains the response from API_CreateGroup.
type CreateGroupResult struct {
	// Group is the created group
	Group Group
}

// createGroupResponse is the XML response structure for API_CreateGroup.
type createGroupResponse struct {
	BaseResponse
	Group Group `xml:"group"`
}

// CreateGroup creates a new group.
//
// The group will be created with the caller as the group owner and first member.
// The caller must be the manager of the account where the group is created.
//
// The name may not contain spaces or punctuation.
//
// Example:
//
//	result, err := xmlClient.CreateGroup(ctx, "MarketingTeam", "Marketing department users", "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Created group: %s (ID: %s)\n", result.Group.Name, result.Group.ID)
//
// See: https://help.quickbase.com/docs/api-creategroup
func (c *Client) CreateGroup(ctx context.Context, name, description, accountId string) (*CreateGroupResult, error) {
	inner := "<name>" + xmlEscape(name) + "</name>"
	inner += "<description>" + xmlEscape(description) + "</description>"
	if accountId != "" {
		inner += "<accountId>" + xmlEscape(accountId) + "</accountId>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, "main", "API_CreateGroup", body)
	if err != nil {
		return nil, fmt.Errorf("API_CreateGroup: %w", err)
	}

	var resp createGroupResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_CreateGroup response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &CreateGroupResult{
		Group: resp.Group,
	}, nil
}

// DeleteGroup deletes a group.
//
// Caution: Once a group has been deleted it cannot be restored.
//
// Example:
//
//	err := xmlClient.DeleteGroup(ctx, "1217.dgpt")
//
// See: https://help.quickbase.com/docs/api-deletegroup
func (c *Client) DeleteGroup(ctx context.Context, groupId string) error {
	inner := "<gid>" + xmlEscape(groupId) + "</gid>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, "main", "API_DeleteGroup", body)
	if err != nil {
		return fmt.Errorf("API_DeleteGroup: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_DeleteGroup response: %w", err)
	}

	return checkError(&resp)
}

// GroupUser represents a user in a group.
type GroupUser struct {
	// ID is the user's QuickBase user ID
	ID string `xml:"id,attr"`

	// FirstName is the user's first name
	FirstName string `xml:"firstName"`

	// LastName is the user's last name
	LastName string `xml:"lastName"`

	// Email is the user's email address
	Email string `xml:"email"`

	// ScreenName is the user's screen name (if set)
	ScreenName string `xml:"screenName"`

	// IsAdmin indicates if the user is an admin of the group
	IsAdmin bool `xml:"isAdmin"`
}

// GroupManager represents a manager of a group.
type GroupManager struct {
	// ID is the manager's QuickBase user ID
	ID string `xml:"id,attr"`

	// FirstName is the manager's first name
	FirstName string `xml:"firstName"`

	// LastName is the manager's last name
	LastName string `xml:"lastName"`

	// Email is the manager's email address
	Email string `xml:"email"`

	// ScreenName is the manager's screen name (if set)
	ScreenName string `xml:"screenName"`

	// IsMember indicates if the manager is also a member of the group
	IsMember bool `xml:"isMember"`
}

// GroupSubgroup represents a subgroup within a group.
type GroupSubgroup struct {
	// ID is the subgroup's ID
	ID string `xml:"id,attr"`
}

// GetUsersInGroupResult contains the response from API_GetUsersInGroup.
type GetUsersInGroupResult struct {
	// GroupID is the group ID
	GroupID string

	// Name is the group name
	Name string

	// Description is the group description
	Description string

	// Users is the list of users in the group
	Users []GroupUser

	// Managers is the list of managers of the group (only if includeManagers=true)
	Managers []GroupManager

	// Subgroups is the list of subgroups in the group
	Subgroups []GroupSubgroup
}

// getUsersInGroupResponse is the XML response structure for API_GetUsersInGroup.
type getUsersInGroupResponse struct {
	BaseResponse
	Group struct {
		ID          string          `xml:"id,attr"`
		Name        string          `xml:"name"`
		Description string          `xml:"description"`
		Users       []GroupUser     `xml:"users>user"`
		Managers    []GroupManager  `xml:"managers>manager"`
		Subgroups   []GroupSubgroup `xml:"subgroups>subgroup"`
	} `xml:"group"`
}

// GetUsersInGroup returns the list of users, managers, and subgroups in a group.
//
// If includeManagers is true, both members and managers of the group are returned.
//
// Example:
//
//	result, err := xmlClient.GetUsersInGroup(ctx, "1217.dgpt", true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Group: %s\n", result.Name)
//	for _, user := range result.Users {
//	    fmt.Printf("  User: %s %s (%s)\n", user.FirstName, user.LastName, user.Email)
//	}
//
// See: https://help.quickbase.com/docs/api-getusersingroup
func (c *Client) GetUsersInGroup(ctx context.Context, groupId string, includeManagers bool) (*GetUsersInGroupResult, error) {
	inner := "<gid>" + xmlEscape(groupId) + "</gid>"
	if includeManagers {
		inner += "<includeAllMgrs>true</includeAllMgrs>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, "main", "API_GetUsersInGroup", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetUsersInGroup: %w", err)
	}

	var resp getUsersInGroupResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetUsersInGroup response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &GetUsersInGroupResult{
		GroupID:     resp.Group.ID,
		Name:        resp.Group.Name,
		Description: resp.Group.Description,
		Users:       resp.Group.Users,
		Managers:    resp.Group.Managers,
		Subgroups:   resp.Group.Subgroups,
	}, nil
}

// AddUserToGroup adds a user to a group.
//
// The user can be added as a regular member or as an admin (manager) of the group.
//
// Example:
//
//	// Add user as regular member
//	err := xmlClient.AddUserToGroup(ctx, "1217.dgpt", "112149.bhsv", false)
//
//	// Add user as admin
//	err := xmlClient.AddUserToGroup(ctx, "1217.dgpt", "112149.bhsv", true)
//
// See: https://help.quickbase.com/docs/api-addusertogroup
func (c *Client) AddUserToGroup(ctx context.Context, groupId, userId string, allowAdminAccess bool) error {
	inner := "<gid>" + xmlEscape(groupId) + "</gid>"
	inner += "<uid>" + xmlEscape(userId) + "</uid>"
	if allowAdminAccess {
		inner += "<allowAdminAccess>true</allowAdminAccess>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, "main", "API_AddUserToGroup", body)
	if err != nil {
		return fmt.Errorf("API_AddUserToGroup: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_AddUserToGroup response: %w", err)
	}

	return checkError(&resp)
}

// RemoveUserFromGroup removes a user from a group.
//
// Note: You cannot remove the last manager from a group.
//
// Example:
//
//	err := xmlClient.RemoveUserFromGroup(ctx, "1217.dgpt", "112149.bhsv")
//
// See: https://help.quickbase.com/docs/api-removeuserfromgroup
func (c *Client) RemoveUserFromGroup(ctx context.Context, groupId, userId string) error {
	inner := "<gid>" + xmlEscape(groupId) + "</gid>"
	inner += "<uid>" + xmlEscape(userId) + "</uid>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, "main", "API_RemoveUserFromGroup", body)
	if err != nil {
		return fmt.Errorf("API_RemoveUserFromGroup: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_RemoveUserFromGroup response: %w", err)
	}

	return checkError(&resp)
}

// GetGroupRoleResult contains the response from API_GetGroupRole.
type GetGroupRoleResult struct {
	// Roles is the list of roles assigned to the group in the app
	Roles []Role
}

// getGroupRoleResponse is the XML response structure for API_GetGroupRole.
type getGroupRoleResponse struct {
	BaseResponse
	Roles []Role `xml:"roles>role"`
}

// GetGroupRole returns the roles assigned to a group in an application.
//
// Example:
//
//	result, err := xmlClient.GetGroupRole(ctx, appId, "1217.dgpt")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, role := range result.Roles {
//	    fmt.Printf("Role: %s (ID: %d)\n", role.Name, role.ID)
//	}
//
// See: https://help.quickbase.com/docs/api-getgrouprole
func (c *Client) GetGroupRole(ctx context.Context, appId, groupId string) (*GetGroupRoleResult, error) {
	inner := "<gid>" + xmlEscape(groupId) + "</gid>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_GetGroupRole", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetGroupRole: %w", err)
	}

	var resp getGroupRoleResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetGroupRole response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &GetGroupRoleResult{
		Roles: resp.Roles,
	}, nil
}

// AddGroupToRole assigns a group to a role in an application.
//
// Example:
//
//	err := xmlClient.AddGroupToRole(ctx, appId, "1217.dgpt", 12)
//
// See: https://help.quickbase.com/docs/api-addgrouptorole
func (c *Client) AddGroupToRole(ctx context.Context, appId, groupId string, roleId int) error {
	inner := "<gid>" + xmlEscape(groupId) + "</gid>"
	inner += fmt.Sprintf("<roleid>%d</roleid>", roleId)
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_AddGroupToRole", body)
	if err != nil {
		return fmt.Errorf("API_AddGroupToRole: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_AddGroupToRole response: %w", err)
	}

	return checkError(&resp)
}

// RemoveGroupFromRole removes a group from a role in an application.
//
// If allRoles is true, the group is removed from all roles in the app.
//
// Example:
//
//	// Remove from specific role
//	err := xmlClient.RemoveGroupFromRole(ctx, appId, "1217.dgpt", 12, false)
//
//	// Remove from all roles
//	err := xmlClient.RemoveGroupFromRole(ctx, appId, "1217.dgpt", 0, true)
//
// See: https://help.quickbase.com/docs/api-removegroupfromrole
func (c *Client) RemoveGroupFromRole(ctx context.Context, appId, groupId string, roleId int, allRoles bool) error {
	inner := "<gid>" + xmlEscape(groupId) + "</gid>"
	if !allRoles {
		inner += fmt.Sprintf("<roleid>%d</roleid>", roleId)
	}
	if allRoles {
		inner += "<allRoles>true</allRoles>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_RemoveGroupFromRole", body)
	if err != nil {
		return fmt.Errorf("API_RemoveGroupFromRole: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_RemoveGroupFromRole response: %w", err)
	}

	return checkError(&resp)
}
