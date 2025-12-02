package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTempTokenStrategy_WithInitialToken(t *testing.T) {
	strategy := NewTempTokenStrategy("testrealm",
		WithInitialTempToken("test-token-123"),
	)

	ctx := context.Background()
	token, err := strategy.GetToken(ctx, "bqtest123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("expected 'test-token-123', got '%s'", token)
	}

	// Second call should use cached token
	token2, err := strategy.GetToken(ctx, "bqtest123")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if token2 != "test-token-123" {
		t.Errorf("expected cached token 'test-token-123', got '%s'", token2)
	}
}

func TestTempTokenStrategy_WithInitialTokenForTable(t *testing.T) {
	strategy := NewTempTokenStrategy("testrealm",
		WithInitialTempTokenForTable("token-for-table", "bqtable456"),
	)

	ctx := context.Background()
	token, err := strategy.GetToken(ctx, "bqtable456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "token-for-table" {
		t.Errorf("expected 'token-for-table', got '%s'", token)
	}

	// Different table should fail
	_, err = strategy.GetToken(ctx, "other-table")
	if err == nil {
		t.Error("expected error for different table without token")
	}
}

func TestTempTokenStrategy_SetToken(t *testing.T) {
	strategy := NewTempTokenStrategy("testrealm")

	ctx := context.Background()

	// Initially no token
	_, err := strategy.GetToken(ctx, "bqtest123")
	if err == nil {
		t.Error("expected error when no token set")
	}

	// Set token
	strategy.SetToken("bqtest123", "manually-set-token")

	// Now should work
	token, err := strategy.GetToken(ctx, "bqtest123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "manually-set-token" {
		t.Errorf("expected 'manually-set-token', got '%s'", token)
	}
}

func TestTempTokenStrategy_ApplyAuth(t *testing.T) {
	strategy := NewTempTokenStrategy("testrealm")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	strategy.ApplyAuth(req, "my-temp-token")

	auth := req.Header.Get("Authorization")
	expected := "QB-TEMP-TOKEN my-temp-token"
	if auth != expected {
		t.Errorf("expected Authorization '%s', got '%s'", expected, auth)
	}
}

func TestTempTokenStrategy_Invalidate(t *testing.T) {
	strategy := NewTempTokenStrategy("testrealm",
		WithInitialTempTokenForTable("token1", "table1"),
	)

	ctx := context.Background()

	// Token should exist
	_, err := strategy.GetToken(ctx, "table1")
	if err != nil {
		t.Fatalf("expected token to exist: %v", err)
	}

	// Invalidate
	strategy.Invalidate("table1")

	// Token should be gone
	_, err = strategy.GetToken(ctx, "table1")
	if err == nil {
		t.Error("expected error after invalidation")
	}
}

func TestTempTokenStrategy_TokenExpiry(t *testing.T) {
	strategy := NewTempTokenStrategy("testrealm",
		WithTempTokenLifespan(50*time.Millisecond),
		WithInitialTempTokenForTable("expiring-token", "table1"),
	)

	ctx := context.Background()

	// Token should exist
	token, err := strategy.GetToken(ctx, "table1")
	if err != nil {
		t.Fatalf("expected token to exist: %v", err)
	}
	if token != "expiring-token" {
		t.Errorf("expected 'expiring-token', got '%s'", token)
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)

	// Token should be expired
	_, err = strategy.GetToken(ctx, "table1")
	if err == nil {
		t.Error("expected error after token expiry")
	}
}

func TestTempTokenStrategy_HandleAuthError(t *testing.T) {
	strategy := NewTempTokenStrategy("testrealm",
		WithInitialTempTokenForTable("token1", "table1"),
	)

	ctx := context.Background()

	// Token should exist
	_, err := strategy.GetToken(ctx, "table1")
	if err != nil {
		t.Fatalf("expected token to exist: %v", err)
	}

	// Handle 401 error - should invalidate
	newToken, err := strategy.HandleAuthError(ctx, 401, "table1", 0, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newToken != "" {
		t.Error("expected empty token (can't refresh server-side)")
	}

	// Token should be invalidated
	_, err = strategy.GetToken(ctx, "table1")
	if err == nil {
		t.Error("expected error after HandleAuthError invalidation")
	}
}
