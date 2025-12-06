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
