package client

import (
	"encoding/json"
	"net/http"

	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/generated"
)

// jsonResult returns a generated method's success result, or QuickBase's
// error. With multiStatus, a 207 (QuickBase applied part of the request, such
// as an upsert with rejected values) returns the result: its body has the
// success shape.
func jsonResult[T any](status int, body []byte, httpResp *http.Response, ok *T, multiStatus bool) (*T, error) {
	if ok != nil {
		return ok, nil
	}
	if multiStatus && status == http.StatusMultiStatus {
		var result T
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}
	return nil, parseAPIError(status, body, httpResp)
}

// rawResult returns a response that isn't JSON (QBL, file content), or
// QuickBase's error.
func rawResult(status int, body []byte, httpResp *http.Response) ([]byte, error) {
	if status >= 200 && status < 300 {
		return body, nil
	}
	return nil, parseAPIError(status, body, httpResp)
}

// lineErrors returns an upsert's rejected values as a LineErrorsError when
// the client has WithLineErrorsAsErrors.
func (c *Client) lineErrors(result *generated.UpsertResult) error {
	if !c.lineErrorsAsErrors || result == nil {
		return nil
	}
	if lineErrors := result.Metadata.LineErrors; len(lineErrors) > 0 {
		return core.NewLineErrorsError(lineErrors)
	}
	return nil
}
