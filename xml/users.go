package xml

import (
	"context"
	"encoding/xml"
	"fmt"
)

// UserInfo contains information about a QuickBase user.
type UserInfo struct {
	// ID is the unique user ID (e.g., "112149.bhsv")
	ID string

	// FirstName is the user's first name
	FirstName string

	// LastName is the user's last name
	LastName string

	// Email is the user's email address
	Email string

	// Login is the user's login name (LDAP, screen name, or email)
	Login string

	// ScreenName is the user's QuickBase screen name
	ScreenName string

	// IsVerified indicates if the user's email is verified
	IsVerified bool

	// ExternalAuth indicates if user uses external authentication
	ExternalAuth bool
}

// userXML is the XML structure for a user element.
type userXML struct {
	ID           string `xml:"id,attr"`
	FirstName    string `xml:"firstName"`
	LastName     string `xml:"lastName"`
	Email        string `xml:"email"`
	Login        string `xml:"login"`
	ScreenName   string `xml:"screenName"`
	IsVerified   int    `xml:"isVerified"`
	ExternalAuth int    `xml:"externalAuth"`
}

// getUserInfoResponse is the XML response structure for API_GetUserInfo.
type getUserInfoResponse struct {
	BaseResponse
	User userXML `xml:"user"`
}

// GetUserInfo returns information about a user by email address.
//
// If email is empty, returns info about the current authenticated user.
// The email must belong to a user registered with QuickBase.
//
// Use this to get a user ID before calling role assignment methods.
//
// Example:
//
//	// Get info about a specific user
//	user, err := xmlClient.GetUserInfo(ctx, "user@example.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("User ID: %s, Name: %s %s\n", user.ID, user.FirstName, user.LastName)
//
//	// Get info about the current user
//	me, err := xmlClient.GetUserInfo(ctx, "")
//
// See: https://help.quickbase.com/docs/api-getuserinfo
func (c *Client) GetUserInfo(ctx context.Context, email string) (*UserInfo, error) {
	inner := ""
	if email != "" {
		inner = "<email>" + xmlEscape(email) + "</email>"
	}
	body := buildRequest(inner)

	// GetUserInfo is invoked on db/main
	respBody, err := c.caller.DoXML(ctx, "main", "API_GetUserInfo", body)
	if err != nil {
		return nil, fmt.Errorf("API_GetUserInfo: %w", err)
	}

	var resp getUserInfoResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_GetUserInfo response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &UserInfo{
		ID:           resp.User.ID,
		FirstName:    resp.User.FirstName,
		LastName:     resp.User.LastName,
		Email:        resp.User.Email,
		Login:        resp.User.Login,
		ScreenName:   resp.User.ScreenName,
		IsVerified:   resp.User.IsVerified == 1,
		ExternalAuth: resp.User.ExternalAuth == 1,
	}, nil
}

// ProvisionUserResult contains the response from API_ProvisionUser.
type ProvisionUserResult struct {
	// UserID is the user ID of the newly provisioned user
	UserID string
}

// provisionUserResponse is the XML response structure for API_ProvisionUser.
type provisionUserResponse struct {
	BaseResponse
	UserID string `xml:"userid"`
}

// ProvisionUser adds a new user who is not yet registered with QuickBase.
//
// This call:
//   - Starts a new user registration using the supplied email, first name, and last name
//   - Gives application access to the user by adding them to the specified role
//
// After calling ProvisionUser, use SendInvitation to invite the new user via email.
// When the user clicks the invitation, they'll complete their registration.
//
// If the user is already registered with QuickBase, this call returns an error.
// Use GetUserInfo, AddUserToRole, and SendInvitation for existing users.
//
// Permissions required:
//   - Basic Access with Sharing: Can assign roles except Full Administration
//   - Full Administration: Can assign any role
//
// Example:
//
//	result, err := xmlClient.ProvisionUser(ctx, appId, "new@example.com", "John", "Doe", 11)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Provisioned user: %s\n", result.UserID)
//
//	// Send invitation email
//	err = xmlClient.SendInvitation(ctx, appId, result.UserID, "Welcome to our app!")
//
// See: https://help.quickbase.com/docs/api-provisionuser
func (c *Client) ProvisionUser(ctx context.Context, appId, email, firstName, lastName string, roleId int) (*ProvisionUserResult, error) {
	inner := "<email>" + xmlEscape(email) + "</email>"
	inner += "<fname>" + xmlEscape(firstName) + "</fname>"
	inner += "<lname>" + xmlEscape(lastName) + "</lname>"
	if roleId > 0 {
		inner += fmt.Sprintf("<roleid>%d</roleid>", roleId)
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_ProvisionUser", body)
	if err != nil {
		return nil, fmt.Errorf("API_ProvisionUser: %w", err)
	}

	var resp provisionUserResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_ProvisionUser response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &ProvisionUserResult{
		UserID: resp.UserID,
	}, nil
}

// SendInvitation sends an email invitation to a user for an application.
//
// You can send an invitation to:
//   - An existing QuickBase user granted access via AddUserToRole
//   - A new user created via ProvisionUser
//
// Example:
//
//	err := xmlClient.SendInvitation(ctx, appId, "112149.bhsv", "Welcome to our project tracker!")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// See: https://help.quickbase.com/docs/api-sendinvitation
func (c *Client) SendInvitation(ctx context.Context, appId, userId, userText string) error {
	inner := "<userid>" + xmlEscape(userId) + "</userid>"
	if userText != "" {
		inner += "<usertext>" + xmlEscape(userText) + "</usertext>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_SendInvitation", body)
	if err != nil {
		return fmt.Errorf("API_SendInvitation: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_SendInvitation response: %w", err)
	}

	return checkError(&resp)
}

// ChangeManager assigns a new manager for an application.
//
// You must be an account admin or realm admin to use this call.
//
// Example:
//
//	err := xmlClient.ChangeManager(ctx, appId, "newmanager@example.com")
//
// See: https://help.quickbase.com/docs/api-changemanager
func (c *Client) ChangeManager(ctx context.Context, appId, newManagerEmail string) error {
	inner := "<newmgr>" + xmlEscape(newManagerEmail) + "</newmgr>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_ChangeManager", body)
	if err != nil {
		return fmt.Errorf("API_ChangeManager: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_ChangeManager response: %w", err)
	}

	return checkError(&resp)
}

// ChangeRecordOwner changes the owner of a record.
//
// In QuickBase, the creator of a record is its owner. Some roles may restrict
// view/modify access to the record owner. Use this call to transfer ownership.
//
// You must have Full Administration rights on the application.
//
// The newOwner can be either:
//   - A QuickBase username
//   - An email address
//
// Example:
//
//	// Change owner by record ID
//	err := xmlClient.ChangeRecordOwner(ctx, tableId, 123, "newowner@example.com")
//
// See: https://help.quickbase.com/docs/api-changerecordowner
func (c *Client) ChangeRecordOwner(ctx context.Context, tableId string, recordId int, newOwner string) error {
	inner := fmt.Sprintf("<rid>%d</rid>", recordId)
	inner += "<newowner>" + xmlEscape(newOwner) + "</newowner>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, tableId, "API_ChangeRecordOwner", body)
	if err != nil {
		return fmt.Errorf("API_ChangeRecordOwner: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_ChangeRecordOwner response: %w", err)
	}

	return checkError(&resp)
}

// SignOut clears the ticket cookie for API clients using cookie-based authentication.
//
// This call is primarily for API client implementations that use the ticket cookie
// rather than the <ticket> parameter. It returns a null ticket cookie (named TICKET).
//
// Important notes:
//   - This does NOT invalidate any tickets
//   - This does NOT log off the caller from QuickBase applications
//   - Callers with a saved valid ticket can continue using it after SignOut
//   - Some local applications may be unable to access QuickBase until
//     API_Authenticate is called for a new ticket cookie
//
// For most server-side SDK usage with user tokens, this call has no practical effect.
// It may be useful in edge cases involving browser-based ticket authentication.
//
// Example:
//
//	err := xmlClient.SignOut(ctx)
//	if err != nil {
//	    log.Printf("SignOut failed: %v", err)
//	}
//
// See: https://help.quickbase.com/docs/api-signout
func (c *Client) SignOut(ctx context.Context) error {
	body := buildRequest("")

	// SignOut is invoked on db/main
	respBody, err := c.caller.DoXML(ctx, "main", "API_SignOut", body)
	if err != nil {
		return fmt.Errorf("API_SignOut: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_SignOut response: %w", err)
	}

	return checkError(&resp)
}
