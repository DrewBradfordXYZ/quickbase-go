package xml

import (
	"context"
	"encoding/xml"
	"fmt"
)

// getDBVarResponse is the XML response structure for API_GetDBVar.
type getDBVarResponse struct {
	BaseResponse
	Value string `xml:"value"`
}

// GetDBVar returns the value of an application variable (DBVar).
//
// DBVars are application-level variables that can be used in formulas
// and accessed via the API. You must have at least viewer access to the app.
//
// Example:
//
//	value, err := xmlClient.GetDBVar(ctx, appId, "myVariable")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Variable value: %s\n", value)
//
// See: https://help.quickbase.com/docs/api-getdbvar
func (c *Client) GetDBVar(ctx context.Context, appId, varName string) (string, error) {
	resolvedID := c.resolveTable(appId)
	inner := "<varname>" + xmlEscape(varName) + "</varname>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, resolvedID, "API_GetDBVar", body)
	if err != nil {
		return "", fmt.Errorf("API_GetDBVar: %w", err)
	}

	var resp getDBVarResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parsing API_GetDBVar response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return "", err
	}

	return resp.Value, nil
}

// SetDBVar sets the value of an application variable (DBVar).
//
// If the variable doesn't exist, it will be created.
// If it exists, the value will be overwritten.
// You must have full admin rights on the application.
//
// Example:
//
//	err := xmlClient.SetDBVar(ctx, appId, "myVariable", "new value")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// See: https://help.quickbase.com/docs/api-setdbvar
func (c *Client) SetDBVar(ctx context.Context, appId, varName, value string) error {
	resolvedID := c.resolveTable(appId)
	inner := "<varname>" + xmlEscape(varName) + "</varname>"
	inner += "<value>" + xmlEscape(value) + "</value>"
	body := buildRequest(inner)

	respBody, err := c.caller.DoXML(ctx, resolvedID, "API_SetDBVar", body)
	if err != nil {
		return fmt.Errorf("API_SetDBVar: %w", err)
	}

	var resp BaseResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_SetDBVar response: %w", err)
	}

	if err := checkError(&resp); err != nil {
		return err
	}

	return nil
}
