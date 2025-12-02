package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractPostTempToken_JSON(t *testing.T) {
	body := `{"tempToken": "abc123xyz"}`
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	token, err := ExtractPostTempToken(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "abc123xyz" {
		t.Errorf("expected token 'abc123xyz', got '%s'", token)
	}
}

func TestExtractPostTempToken_FormEncoded(t *testing.T) {
	body := "tempToken=abc123xyz"
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	token, err := ExtractPostTempToken(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "abc123xyz" {
		t.Errorf("expected token 'abc123xyz', got '%s'", token)
	}
}

func TestExtractPostTempToken_FormEncodedAltName(t *testing.T) {
	body := "temp_token=abc123xyz"
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	token, err := ExtractPostTempToken(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "abc123xyz" {
		t.Errorf("expected token 'abc123xyz', got '%s'", token)
	}
}

func TestExtractPostTempToken_NoContentType(t *testing.T) {
	// Should fallback to trying JSON
	body := `{"tempToken": "abc123xyz"}`
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))

	token, err := ExtractPostTempToken(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "abc123xyz" {
		t.Errorf("expected token 'abc123xyz', got '%s'", token)
	}
}

func TestExtractPostTempToken_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/callback", nil)

	_, err := ExtractPostTempToken(req)
	if err == nil {
		t.Fatal("expected error for GET request")
	}
	if !strings.Contains(err.Error(), "expected POST") {
		t.Errorf("error should mention POST: %v", err)
	}
}

func TestExtractPostTempToken_EmptyToken(t *testing.T) {
	body := `{"tempToken": ""}`
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	_, err := ExtractPostTempToken(req)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestExtractPostTempToken_InvalidJSON(t *testing.T) {
	body := `not json`
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	_, err := ExtractPostTempToken(req)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidatePostTempToken(t *testing.T) {
	tests := []struct {
		token string
		valid bool
	}{
		{"abc123xyz789", true},
		{"short", false},
		{"", false},
		{"a", false},
	}

	for _, tt := range tests {
		got := ValidatePostTempToken(tt.token)
		if got != tt.valid {
			t.Errorf("ValidatePostTempToken(%q) = %v, want %v", tt.token, got, tt.valid)
		}
	}
}
