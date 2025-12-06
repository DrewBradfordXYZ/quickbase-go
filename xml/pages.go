package xml

import (
	"context"
	"encoding/xml"
	"fmt"
)

// PageType represents the type of a code page.
type PageType int

const (
	// PageTypeXSLOrHTML is for XSL stylesheets or HTML pages (type 1)
	PageTypeXSLOrHTML PageType = 1

	// PageTypeExactForm is for Exact Forms (type 3)
	PageTypeExactForm PageType = 3
)

// GetDBPage retrieves a stored code page from QuickBase.
//
// QuickBase allows you to store various types of pages, including user-guide
// pages, XSL stylesheets, HTML pages, and Exact Forms.
//
// The pageIdOrName can be either a numeric page ID or a page name.
//
// Example:
//
//	content, err := xmlClient.GetDBPage(ctx, appId, "3")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(content)
//
//	// Or by name
//	content, err := xmlClient.GetDBPage(ctx, appId, "mystylesheet.xsl")
//
// See: https://help.quickbase.com/docs/api-getdbpage
func (c *Client) GetDBPage(ctx context.Context, appId, pageIdOrName string) (string, error) {
	inner := "<pageID>" + xmlEscape(pageIdOrName) + "</pageID>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_GetDBPage", body)
	if err != nil {
		return "", fmt.Errorf("API_GetDBPage: %w", err)
	}

	// The response is the raw page content (HTML), not wrapped in XML
	// However, if there's an error, it will be XML
	// Try to parse as error response first
	var errResp BaseResponse
	if xml.Unmarshal(respBody, &errResp) == nil && errResp.ErrCode != 0 {
		return "", checkError(&errResp)
	}

	return string(respBody), nil
}

// AddReplaceDBPageResult contains the response from API_AddReplaceDBPage.
type AddReplaceDBPageResult struct {
	// PageID is the ID of the page that was added or replaced
	PageID int
}

// addReplaceDBPageResponse is the XML response structure for API_AddReplaceDBPage.
type addReplaceDBPageResponse struct {
	BaseResponse
	PageID int `xml:"pageID"`
}

// AddReplaceDBPage adds a new code page or replaces an existing one.
//
// QuickBase allows you to store various types of pages:
//   - XSL stylesheets or HTML pages (PageTypeXSLOrHTML = 1)
//   - Exact Forms for Word document integration (PageTypeExactForm = 3)
//
// To add a new page, use pageName. To replace an existing page, use pageId.
// One of pageName or pageId must be provided (but not both).
//
// Example (add new page):
//
//	result, err := xmlClient.AddReplaceDBPage(ctx, appId, "newpage.html", 0, PageTypeXSLOrHTML, "<html><body>Hello</body></html>")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Created page ID: %d\n", result.PageID)
//
// Example (replace existing page):
//
//	result, err := xmlClient.AddReplaceDBPage(ctx, appId, "", 6, PageTypeXSLOrHTML, "<html><body>Updated</body></html>")
//
// See: https://help.quickbase.com/docs/api-addreplacedbpage
func (c *Client) AddReplaceDBPage(ctx context.Context, appId, pageName string, pageId int, pageType PageType, pageBody string) (*AddReplaceDBPageResult, error) {
	var inner string
	if pageName != "" {
		inner = "<pagename>" + xmlEscape(pageName) + "</pagename>"
	} else if pageId > 0 {
		inner = fmt.Sprintf("<pageid>%d</pageid>", pageId)
	} else {
		return nil, fmt.Errorf("either pageName or pageId must be provided")
	}

	inner += fmt.Sprintf("<pagetype>%d</pagetype>", pageType)
	inner += "<pagebody><![CDATA[" + pageBody + "]]></pagebody>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, appId, "API_AddReplaceDBPage", body)
	if err != nil {
		return nil, fmt.Errorf("API_AddReplaceDBPage: %w", err)
	}

	var resp addReplaceDBPageResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_AddReplaceDBPage response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &AddReplaceDBPageResult{
		PageID: resp.PageID,
	}, nil
}
