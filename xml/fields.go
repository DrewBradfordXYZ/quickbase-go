package xml

import (
	"context"
	"encoding/xml"
	"fmt"
)

// FieldAddChoicesResult contains the response from API_FieldAddChoices.
type FieldAddChoicesResult struct {
	// FieldID is the field ID
	FieldID int

	// FieldName is the field label
	FieldName string

	// NumAdded is the number of choices successfully added
	NumAdded int
}

// fieldAddChoicesResponse is the XML response structure for API_FieldAddChoices.
type fieldAddChoicesResponse struct {
	BaseResponse
	FieldID   int    `xml:"fid"`
	FieldName string `xml:"fname"`
	NumAdded  int    `xml:"numadded"`
}

// FieldAddChoices adds new choices to a multiple-choice or multi-select text field.
//
// Constraints:
//   - Choices are limited to 60 characters each
//   - Maximum 100 choices per field
//   - Duplicate choices are not added
//
// Permissions:
//   - Full Administration: Can add to any field
//   - Other users: Can only add if field has "Allow users to create new choices" enabled
//
// Example:
//
//	result, err := xmlClient.FieldAddChoices(ctx, tableId, 11, []string{"Red", "Green", "Blue"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Added %d choices to field %s\n", result.NumAdded, result.FieldName)
//
// See: https://help.quickbase.com/docs/api-fieldaddchoices
func (c *Client) FieldAddChoices(ctx context.Context, tableId string, fieldId int, choices []string) (*FieldAddChoicesResult, error) {
	inner := fmt.Sprintf("<fid>%d</fid>", fieldId)
	for _, choice := range choices {
		inner += "<choice>" + xmlEscape(choice) + "</choice>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, tableId, "API_FieldAddChoices", body)
	if err != nil {
		return nil, fmt.Errorf("API_FieldAddChoices: %w", err)
	}

	var resp fieldAddChoicesResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_FieldAddChoices response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &FieldAddChoicesResult{
		FieldID:   resp.FieldID,
		FieldName: resp.FieldName,
		NumAdded:  resp.NumAdded,
	}, nil
}

// FieldRemoveChoicesResult contains the response from API_FieldRemoveChoices.
type FieldRemoveChoicesResult struct {
	// FieldID is the field ID
	FieldID int

	// FieldName is the field label
	FieldName string

	// NumRemoved is the number of choices successfully removed
	NumRemoved int
}

// fieldRemoveChoicesResponse is the XML response structure for API_FieldRemoveChoices.
type fieldRemoveChoicesResponse struct {
	BaseResponse
	FieldID    int    `xml:"fid"`
	FieldName  string `xml:"fname"`
	NumRemoved int    `xml:"numremoved"`
}

// FieldRemoveChoices removes choices from a multiple-choice or multi-select text field.
//
// You can remove any choices you created. You need Full Administration rights
// to remove choices created by others.
//
// If some choices cannot be removed (don't exist or lack permission), other valid
// choices will still be removed. Check NumRemoved in the result to verify.
//
// Example:
//
//	result, err := xmlClient.FieldRemoveChoices(ctx, tableId, 11, []string{"Red", "Blue"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Removed %d choices from field %s\n", result.NumRemoved, result.FieldName)
//
// See: https://help.quickbase.com/docs/api-fieldremovechoices
func (c *Client) FieldRemoveChoices(ctx context.Context, tableId string, fieldId int, choices []string) (*FieldRemoveChoicesResult, error) {
	inner := fmt.Sprintf("<fid>%d</fid>", fieldId)
	for _, choice := range choices {
		inner += "<choice>" + xmlEscape(choice) + "</choice>"
	}
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, tableId, "API_FieldRemoveChoices", body)
	if err != nil {
		return nil, fmt.Errorf("API_FieldRemoveChoices: %w", err)
	}

	var resp fieldRemoveChoicesResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_FieldRemoveChoices response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &FieldRemoveChoicesResult{
		FieldID:    resp.FieldID,
		FieldName:  resp.FieldName,
		NumRemoved: resp.NumRemoved,
	}, nil
}

// SetKeyField sets a field as the key field for a table.
//
// Requirements for a key field:
//   - Must be a unique field (Unique checkbox checked)
//   - All values must be unique and non-blank
//   - Cannot be a List-user, Multi-select text, or formula field
//
// If you don't set a key field, QuickBase uses the built-in Record ID field.
//
// You must have Full Administration rights on the application.
//
// Example:
//
//	// Set field 6 as the key field
//	err := xmlClient.SetKeyField(ctx, tableId, 6)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// See: https://help.quickbase.com/docs/api-setkeyfield
func (c *Client) SetKeyField(ctx context.Context, tableId string, fieldId int) error {
	inner := fmt.Sprintf("<fid>%d</fid>", fieldId)
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, tableId, "API_SetKeyField", body)
	if err != nil {
		return fmt.Errorf("API_SetKeyField: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_SetKeyField response: %w", err)
	}

	return checkError(&resp)
}
