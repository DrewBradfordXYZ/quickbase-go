package xml

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
)

// WebhookHeader represents a key-value pair for webhook headers.
type WebhookHeader struct {
	Key   string
	Value string
}

// WebhookTrigger specifies when a webhook should fire.
// Valid values are "a" (add), "d" (delete), "m" (modify), or any combination.
type WebhookTrigger string

const (
	// WebhookTriggerAdd fires on record creation
	WebhookTriggerAdd WebhookTrigger = "a"
	// WebhookTriggerDelete fires on record deletion
	WebhookTriggerDelete WebhookTrigger = "d"
	// WebhookTriggerModify fires on record modification
	WebhookTriggerModify WebhookTrigger = "m"
	// WebhookTriggerAll fires on any change
	WebhookTriggerAll WebhookTrigger = "adm"
)

// WebhookMessageFormat specifies the format of the webhook payload.
type WebhookMessageFormat string

const (
	WebhookFormatXML  WebhookMessageFormat = "XML"
	WebhookFormatJSON WebhookMessageFormat = "JSON"
	WebhookFormatRAW  WebhookMessageFormat = "RAW"
)

// WebhookHTTPVerb specifies the HTTP method for the webhook.
type WebhookHTTPVerb string

const (
	WebhookVerbPOST   WebhookHTTPVerb = "POST"
	WebhookVerbGET    WebhookHTTPVerb = "GET"
	WebhookVerbPUT    WebhookHTTPVerb = "PUT"
	WebhookVerbPATCH  WebhookHTTPVerb = "PATCH"
	WebhookVerbDELETE WebhookHTTPVerb = "DELETE"
)

// WebhooksCreateOptions configures the webhook creation.
type WebhooksCreateOptions struct {
	// Label is a unique name for the webhook (required)
	Label string

	// WebhookURL is the endpoint URL (must start with https://) (required)
	WebhookURL string

	// Description of the webhook (optional)
	Description string

	// Query is filter criteria to trigger the webhook (optional)
	Query string

	// WorkflowWhen specifies when to trigger: "a" (add), "d" (delete), "m" (modify)
	// Can be combined, e.g., "adm" for all triggers. Default is "a".
	WorkflowWhen WebhookTrigger

	// Headers are key-value pairs for the webhook request headers
	Headers []WebhookHeader

	// Message is the payload of the webhook (empty by default)
	Message string

	// MessageFormat is the format of the payload: XML (default), JSON, or RAW
	MessageFormat WebhookMessageFormat

	// HTTPVerb specifies the HTTP method: POST (default), GET, PUT, PATCH, DELETE
	HTTPVerb WebhookHTTPVerb

	// TriggerFields limits the webhook to fire only when these field IDs change.
	// If empty, webhook fires on any field change.
	TriggerFields []int
}

// webhooksCreateResponse is the XML response for API_Webhooks_Create.
type webhooksCreateResponse struct {
	BaseResponse
	Changed bool `xml:"changed"`
	Success bool `xml:"success"`
}

// WebhooksCreate creates a new webhook for the table.
//
// The webhook will fire based on the specified trigger conditions and send
// an HTTP request to the configured URL.
//
// Example:
//
//	err := xmlClient.WebhooksCreate(ctx, tableId, xml.WebhooksCreateOptions{
//	    Label:        "Notify on new records",
//	    WebhookURL:   "https://myapp.example.com/webhook",
//	    WorkflowWhen: xml.WebhookTriggerAdd,
//	    MessageFormat: xml.WebhookFormatJSON,
//	    Headers: []xml.WebhookHeader{
//	        {Key: "Content-Type", Value: "application/json"},
//	    },
//	})
//
// See: https://help.quickbase.com/docs/api-webhooks-create
func (c *Client) WebhooksCreate(ctx context.Context, tableId string, opts WebhooksCreateOptions) error {
	inner := fmt.Sprintf("<label>%s</label>", xmlEscape(opts.Label))
	inner += fmt.Sprintf("<WebhookURL>%s</WebhookURL>", xmlEscape(opts.WebhookURL))

	if opts.Description != "" {
		inner += fmt.Sprintf("<Description>%s</Description>", xmlEscape(opts.Description))
	}
	if opts.Query != "" {
		inner += fmt.Sprintf("<Query>%s</Query>", xmlEscape(opts.Query))
	}
	if opts.WorkflowWhen != "" {
		inner += fmt.Sprintf("<WorkflowWhen>%s</WorkflowWhen>", xmlEscape(string(opts.WorkflowWhen)))
	}
	if len(opts.Headers) > 0 {
		inner += fmt.Sprintf("<WebhookHeaderCount>%d</WebhookHeaderCount>", len(opts.Headers))
		for i, h := range opts.Headers {
			inner += fmt.Sprintf("<WebhookHeaderKey%d>%s</WebhookHeaderKey%d>", i+1, xmlEscape(h.Key), i+1)
			inner += fmt.Sprintf("<WebhookHeaderValue%d>%s</WebhookHeaderValue%d>", i+1, xmlEscape(h.Value), i+1)
		}
	}
	if opts.Message != "" {
		inner += fmt.Sprintf("<WebhookMessage>%s</WebhookMessage>", xmlEscape(opts.Message))
	}
	if opts.MessageFormat != "" {
		inner += fmt.Sprintf("<WebhookMessageFormat>%s</WebhookMessageFormat>", string(opts.MessageFormat))
	}
	if opts.HTTPVerb != "" {
		inner += fmt.Sprintf("<WebhookHTTPVerb>%s</WebhookHTTPVerb>", string(opts.HTTPVerb))
	}
	if len(opts.TriggerFields) > 0 {
		inner += "<tfidsWhich>TRUE</tfidsWhich>"
		for _, fid := range opts.TriggerFields {
			inner += fmt.Sprintf("<tfids>%d</tfids>", fid)
		}
	}

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_Webhooks_Create", body)
	if err != nil {
		return fmt.Errorf("API_Webhooks_Create: %w", err)
	}

	var resp webhooksCreateResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_Webhooks_Create response: %w", err)
	}

	return checkError(&resp.BaseResponse)
}

// WebhooksEditOptions configures webhook editing.
type WebhooksEditOptions struct {
	// ActionID is the ID of the webhook to edit (required)
	ActionID string

	// Label is a unique name for the webhook (required)
	Label string

	// WebhookURL is the endpoint URL (must start with https://) (required)
	WebhookURL string

	// Description of the webhook (optional)
	Description string

	// Query is filter criteria to trigger the webhook (optional)
	Query string

	// WorkflowWhen specifies when to trigger: "a" (add), "d" (delete), "m" (modify)
	WorkflowWhen WebhookTrigger

	// Headers are key-value pairs for the webhook request headers
	Headers []WebhookHeader

	// Message is the payload of the webhook
	Message string

	// MessageFormat is the format of the payload: XML, JSON, or RAW
	MessageFormat WebhookMessageFormat

	// HTTPVerb specifies the HTTP method: POST, GET, PUT, PATCH, DELETE
	HTTPVerb WebhookHTTPVerb

	// TriggerFields limits the webhook to fire only when these field IDs change.
	// Set to nil to keep existing, set to empty slice with ClearTriggerFields=true to clear.
	TriggerFields []int

	// ClearTriggerFields when true clears the field trigger criteria (fires on any field)
	ClearTriggerFields bool
}

// webhooksEditResponse is the XML response for API_Webhooks_Edit.
type webhooksEditResponse struct {
	BaseResponse
	Changed bool `xml:"changed"`
	Success bool `xml:"success"`
}

// WebhooksEdit modifies an existing webhook.
//
// Example:
//
//	err := xmlClient.WebhooksEdit(ctx, tableId, xml.WebhooksEditOptions{
//	    ActionID:      "15",
//	    Label:         "Updated webhook name",
//	    WebhookURL:    "https://myapp.example.com/new-endpoint",
//	    MessageFormat: xml.WebhookFormatJSON,
//	})
//
// See: https://help.quickbase.com/docs/api-webhooks-edit
func (c *Client) WebhooksEdit(ctx context.Context, tableId string, opts WebhooksEditOptions) error {
	inner := fmt.Sprintf("<actionId>%s</actionId>", xmlEscape(opts.ActionID))
	inner += fmt.Sprintf("<label>%s</label>", xmlEscape(opts.Label))
	inner += fmt.Sprintf("<WebhookURL>%s</WebhookURL>", xmlEscape(opts.WebhookURL))

	if opts.Description != "" {
		inner += fmt.Sprintf("<Description>%s</Description>", xmlEscape(opts.Description))
	}
	if opts.Query != "" {
		inner += fmt.Sprintf("<Query>%s</Query>", xmlEscape(opts.Query))
	}
	if opts.WorkflowWhen != "" {
		inner += fmt.Sprintf("<WorkflowWhen>%s</WorkflowWhen>", xmlEscape(string(opts.WorkflowWhen)))
	}
	if len(opts.Headers) > 0 {
		inner += fmt.Sprintf("<WebhookHeaderCount>%d</WebhookHeaderCount>", len(opts.Headers))
		for i, h := range opts.Headers {
			inner += fmt.Sprintf("<WebhookHeaderKey%d>%s</WebhookHeaderKey%d>", i+1, xmlEscape(h.Key), i+1)
			inner += fmt.Sprintf("<WebhookHeaderValue%d>%s</WebhookHeaderValue%d>", i+1, xmlEscape(h.Value), i+1)
		}
	}
	if opts.Message != "" {
		inner += fmt.Sprintf("<WebhookMessage>%s</WebhookMessage>", xmlEscape(opts.Message))
	}
	if opts.MessageFormat != "" {
		inner += fmt.Sprintf("<WebhookMessageFormat>%s</WebhookMessageFormat>", string(opts.MessageFormat))
	}
	if opts.HTTPVerb != "" {
		inner += fmt.Sprintf("<WebhookHTTPVerb>%s</WebhookHTTPVerb>", string(opts.HTTPVerb))
	}
	if opts.ClearTriggerFields {
		inner += "<tfidsWhich>tfidsAny</tfidsWhich>"
	} else if len(opts.TriggerFields) > 0 {
		inner += "<tfidsWhich>TRUE</tfidsWhich>"
		for _, fid := range opts.TriggerFields {
			inner += fmt.Sprintf("<tfids>%d</tfids>", fid)
		}
	}

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_Webhooks_Edit", body)
	if err != nil {
		return fmt.Errorf("API_Webhooks_Edit: %w", err)
	}

	var resp webhooksEditResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("parsing API_Webhooks_Edit response: %w", err)
	}

	return checkError(&resp.BaseResponse)
}

// WebhooksDeleteResult contains the response from API_Webhooks_Delete.
type WebhooksDeleteResult struct {
	// NumChanged is the number of webhooks deleted
	NumChanged int
}

// webhooksDeleteResponse is the XML response for API_Webhooks_Delete.
type webhooksDeleteResponse struct {
	BaseResponse
	NumChanged int `xml:"numChanged"`
}

// WebhooksDelete removes one or more webhooks.
//
// Example:
//
//	result, err := xmlClient.WebhooksDelete(ctx, tableId, []string{"15", "16"})
//	fmt.Printf("Deleted %d webhooks\n", result.NumChanged)
//
// See: https://help.quickbase.com/docs/api-webhooks-delete
func (c *Client) WebhooksDelete(ctx context.Context, tableId string, actionIds []string) (*WebhooksDeleteResult, error) {
	inner := fmt.Sprintf("<actionIDList>%s</actionIDList>", strings.Join(actionIds, ","))

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_Webhooks_Delete", body)
	if err != nil {
		return nil, fmt.Errorf("API_Webhooks_Delete: %w", err)
	}

	var resp webhooksDeleteResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_Webhooks_Delete response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &WebhooksDeleteResult{
		NumChanged: resp.NumChanged,
	}, nil
}

// WebhooksActivateResult contains the response from API_Webhooks_Activate.
type WebhooksActivateResult struct {
	// NumChanged is the number of webhooks activated
	NumChanged int
}

// webhooksActivateResponse is the XML response for API_Webhooks_Activate.
type webhooksActivateResponse struct {
	BaseResponse
	NumChanged int  `xml:"numChanged"`
	Success    bool `xml:"success"`
}

// WebhooksActivate enables one or more webhooks.
//
// Example:
//
//	result, err := xmlClient.WebhooksActivate(ctx, tableId, []string{"15", "16"})
//	fmt.Printf("Activated %d webhooks\n", result.NumChanged)
//
// See: https://help.quickbase.com/docs/api-webhooks-activate
func (c *Client) WebhooksActivate(ctx context.Context, tableId string, actionIds []string) (*WebhooksActivateResult, error) {
	inner := fmt.Sprintf("<actionIDList>%s</actionIDList>", strings.Join(actionIds, ","))

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_Webhooks_Activate", body)
	if err != nil {
		return nil, fmt.Errorf("API_Webhooks_Activate: %w", err)
	}

	var resp webhooksActivateResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_Webhooks_Activate response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &WebhooksActivateResult{
		NumChanged: resp.NumChanged,
	}, nil
}

// WebhooksDeactivateResult contains the response from API_Webhooks_Deactivate.
type WebhooksDeactivateResult struct {
	// NumChanged is the number of webhooks deactivated
	NumChanged int
}

// webhooksDeactivateResponse is the XML response for API_Webhooks_Deactivate.
type webhooksDeactivateResponse struct {
	BaseResponse
	NumChanged int  `xml:"numChanged"`
	Success    bool `xml:"success"`
}

// WebhooksDeactivate disables one or more webhooks.
//
// Use WebhooksActivate to re-enable deactivated webhooks.
//
// Example:
//
//	result, err := xmlClient.WebhooksDeactivate(ctx, tableId, []string{"15"})
//	fmt.Printf("Deactivated %d webhooks\n", result.NumChanged)
//
// See: https://help.quickbase.com/docs/api-webhooks-deactivate
func (c *Client) WebhooksDeactivate(ctx context.Context, tableId string, actionIds []string) (*WebhooksDeactivateResult, error) {
	inner := fmt.Sprintf("<actionIDList>%s</actionIDList>", strings.Join(actionIds, ","))

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_Webhooks_Deactivate", body)
	if err != nil {
		return nil, fmt.Errorf("API_Webhooks_Deactivate: %w", err)
	}

	var resp webhooksDeactivateResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_Webhooks_Deactivate response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &WebhooksDeactivateResult{
		NumChanged: resp.NumChanged,
	}, nil
}

// WebhooksCopyResult contains the response from API_Webhooks_Copy.
type WebhooksCopyResult struct {
	// ActionID is the ID of the newly copied webhook
	ActionID string
}

// webhooksCopyResponse is the XML response for API_Webhooks_Copy.
type webhooksCopyResponse struct {
	BaseResponse
	ActionID string `xml:"actionId"`
	Success  bool   `xml:"success"`
}

// WebhooksCopy duplicates an existing webhook.
//
// Example:
//
//	result, err := xmlClient.WebhooksCopy(ctx, tableId, "15")
//	fmt.Printf("Created webhook copy with ID: %s\n", result.ActionID)
//
// See: https://help.quickbase.com/docs/api-webhooks-copy
func (c *Client) WebhooksCopy(ctx context.Context, tableId string, actionId string) (*WebhooksCopyResult, error) {
	inner := fmt.Sprintf("<actionId>%s</actionId>", xmlEscape(actionId))

	body := buildRequest(inner)
	respBody, err := c.caller.DoXML(ctx, tableId, "API_Webhooks_Copy", body)
	if err != nil {
		return nil, fmt.Errorf("API_Webhooks_Copy: %w", err)
	}

	var resp webhooksCopyResponse
	if err := xml.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing API_Webhooks_Copy response: %w", err)
	}

	if err := checkError(&resp.BaseResponse); err != nil {
		return nil, err
	}

	return &WebhooksCopyResult{
		ActionID: resp.ActionID,
	}, nil
}
