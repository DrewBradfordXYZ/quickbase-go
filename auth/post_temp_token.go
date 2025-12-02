package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PostTempTokenRequest represents the body QuickBase sends when POSTing
// a temporary token from a Formula-URL or Formula-rich text field.
//
// QuickBase sends: { "tempToken": "..." } as JSON
type PostTempTokenRequest struct {
	TempToken string `json:"tempToken"`
}

// ExtractPostTempToken extracts a temporary token from an incoming HTTP request.
//
// This is used when QuickBase POSTs a temporary token to your server from a
// Formula-URL or Formula-rich text field with the "POST temp token" option enabled.
//
// QuickBase sends the token as JSON: { "tempToken": "..." }
//
// Example usage in an HTTP handler:
//
//	func handleQuickBaseCallback(w http.ResponseWriter, r *http.Request) {
//	    token, err := auth.ExtractPostTempToken(r)
//	    if err != nil {
//	        http.Error(w, "Invalid request", http.StatusBadRequest)
//	        return
//	    }
//
//	    // Create a client with the received token
//	    client, err := quickbase.New("myrealm",
//	        quickbase.WithTempTokenAuth(
//	            auth.WithInitialTempToken(token),
//	        ),
//	    )
//	    if err != nil {
//	        http.Error(w, "Failed to create client", http.StatusInternalServerError)
//	        return
//	    }
//
//	    // Use the client to make API calls back to QuickBase
//	    resp, err := client.API().GetAppWithResponse(r.Context(), appId)
//	    // ...
//	}
func ExtractPostTempToken(r *http.Request) (string, error) {
	if r.Method != http.MethodPost {
		return "", fmt.Errorf("expected POST request, got %s", r.Method)
	}

	contentType := r.Header.Get("Content-Type")

	// Handle JSON body
	if strings.HasPrefix(contentType, "application/json") {
		return extractFromJSON(r.Body)
	}

	// Handle form-encoded body
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		return extractFromForm(r)
	}

	// Try JSON first, then form as fallback
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("reading request body: %w", err)
	}

	// Try JSON
	var jsonReq PostTempTokenRequest
	if err := json.Unmarshal(body, &jsonReq); err == nil && jsonReq.TempToken != "" {
		return jsonReq.TempToken, nil
	}

	return "", fmt.Errorf("could not extract temp token from request (Content-Type: %s)", contentType)
}

func extractFromJSON(body io.Reader) (string, error) {
	var req PostTempTokenRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return "", fmt.Errorf("decoding JSON body: %w", err)
	}
	if req.TempToken == "" {
		return "", fmt.Errorf("tempToken field is empty or missing")
	}
	return req.TempToken, nil
}

func extractFromForm(r *http.Request) (string, error) {
	if err := r.ParseForm(); err != nil {
		return "", fmt.Errorf("parsing form: %w", err)
	}

	// Try common field names
	fieldNames := []string{"tempToken", "temp_token", "temporaryToken", "token"}
	for _, name := range fieldNames {
		if token := r.FormValue(name); token != "" {
			return token, nil
		}
	}

	return "", fmt.Errorf("no temp token found in form fields")
}

// ValidatePostTempToken checks if a token string looks valid.
// QuickBase temp tokens are non-empty strings.
func ValidatePostTempToken(token string) bool {
	return token != "" && len(token) > 10
}
