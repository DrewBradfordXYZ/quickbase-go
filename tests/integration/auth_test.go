package integration

import (
	"context"
	"os"
	"testing"

	"github.com/DrewBradfordXYZ/quickbase-go"
	"github.com/DrewBradfordXYZ/quickbase-go/auth"
)

func TestUserTokenAuth(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	testCtx := getTestContext(t)

	t.Run("works with valid user token", func(t *testing.T) {
		client, err := quickbase.New(qbRealm, quickbase.WithUserToken(qbUserToken))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetApp failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}
		if *resp.JSON200.Id != testCtx.AppID {
			t.Errorf("App ID = %s, want %s", *resp.JSON200.Id, testCtx.AppID)
		}
	})

	t.Run("fails with invalid user token", func(t *testing.T) {
		client, err := quickbase.New(qbRealm, quickbase.WithUserToken("invalid_token_12345"))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)

		// Should get an error or non-200 response
		if err == nil && resp.JSON200 != nil {
			t.Error("Expected error or non-200 response with invalid token")
		}
	})
}

func TestClientOptions(t *testing.T) {
	skipIfNoCredentials(t)
	ctx := context.Background()
	testCtx := getTestContext(t)

	t.Run("works with debug enabled", func(t *testing.T) {
		client, err := quickbase.New(qbRealm,
			quickbase.WithUserToken(qbUserToken),
			quickbase.WithDebug(true),
		)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetApp failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}
	})

	t.Run("works with proactive throttle", func(t *testing.T) {
		client, err := quickbase.New(qbRealm,
			quickbase.WithUserToken(qbUserToken),
			quickbase.WithProactiveThrottle(100),
		)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Make a few requests to verify throttle doesn't break things
		for i := 0; i < 3; i++ {
			resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)
			if err != nil {
				t.Fatalf("GetApp failed on iteration %d: %v", i, err)
			}
			if resp.JSON200 == nil {
				t.Fatalf("Expected JSON200 response on iteration %d, got status %d", i, resp.StatusCode())
			}
		}
	})

	t.Run("works with custom retry settings", func(t *testing.T) {
		client, err := quickbase.New(qbRealm,
			quickbase.WithUserToken(qbUserToken),
			quickbase.WithMaxRetries(5),
		)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetApp failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}
	})
}

// TestTicketAuth tests authentication using username/password via API_Authenticate.
// Requires QB_USERNAME and QB_PASSWORD environment variables.
func TestTicketAuth(t *testing.T) {
	skipIfNoCredentials(t)

	username := os.Getenv("QB_USERNAME")
	password := os.Getenv("QB_PASSWORD")

	if username == "" || password == "" {
		t.Skip("Skipping ticket auth test: QB_USERNAME or QB_PASSWORD not set")
	}

	ctx := context.Background()
	testCtx := getTestContext(t)

	t.Run("authenticates with username and password", func(t *testing.T) {
		client, err := quickbase.New(qbRealm,
			quickbase.WithTicketAuth(username, password),
		)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// First request triggers authentication
		resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetApp failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}
		if *resp.JSON200.Id != testCtx.AppID {
			t.Errorf("App ID = %s, want %s", *resp.JSON200.Id, testCtx.AppID)
		}
	})

	t.Run("works with custom ticket hours", func(t *testing.T) {
		client, err := quickbase.New(qbRealm,
			quickbase.WithTicketAuth(username, password,
				auth.WithTicketHours(24),
			),
		)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		resp, err := client.API().GetAppWithResponse(ctx, testCtx.AppID)
		if err != nil {
			t.Fatalf("GetApp failed: %v", err)
		}
		if resp.JSON200 == nil {
			t.Fatalf("Expected JSON200 response, got status %d", resp.StatusCode())
		}
	})

	t.Run("fails with invalid credentials", func(t *testing.T) {
		client, err := quickbase.New(qbRealm,
			quickbase.WithTicketAuth("invalid@example.com", "wrongpassword"),
		)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Request should fail during authentication
		_, err = client.API().GetAppWithResponse(ctx, testCtx.AppID)
		if err == nil {
			t.Error("Expected error with invalid credentials, got nil")
		}
	})
}
