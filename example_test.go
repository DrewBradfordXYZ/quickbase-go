package quickbase_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go"
	"github.com/DrewBradfordXYZ/quickbase-go/auth"
)

// Create a basic client with user token authentication.
// This is the simplest and most common authentication method for server-side apps.
func ExampleNew() {
	client, err := quickbase.New("myrealm",
		quickbase.WithUserToken("b9f3pk_xxxx_xxxxxxxxxxxxxxx"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Use client to make API calls
	_ = client
}

// Authenticate with username/password using ticket authentication.
// Unlike user tokens, tickets properly attribute record changes (createdBy/modifiedBy)
// to the authenticated user.
func ExampleWithTicketAuth() {
	client, err := quickbase.New("myrealm",
		quickbase.WithTicketAuth("user@example.com", "password"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// The password is discarded after first authentication.
	// Tickets are valid for 12 hours by default.
	_ = client
}

// Configure ticket authentication with custom validity period.
func ExampleWithTicketAuth_customHours() {
	client, err := quickbase.New("myrealm",
		quickbase.WithTicketAuth("user@example.com", "password",
			auth.WithTicketHours(24*7), // Valid for 1 week
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

// Authenticate using a SAML assertion for SSO.
// This allows API calls to be made as a specific QuickBase user.
func ExampleWithSSOTokenAuth() {
	// Get SAML assertion from your identity provider (Okta, Azure AD, etc.)
	samlAssertion := "base64url-encoded-saml-assertion"

	client, err := quickbase.New("myrealm",
		quickbase.WithSSOTokenAuth(samlAssertion),
	)
	if err != nil {
		log.Fatal(err)
	}

	// API calls now execute as the authenticated SSO user.
	// "Created By" and "Modified By" fields will show their name.
	_ = client
}

// Handle temp tokens received from browser clients (e.g., Code Pages via Datastar).
// The browser fetches tokens using its QuickBase session and sends them to your server.
func ExampleWithTempTokenAuth() {
	// Tokens are received from browser via HTTP headers, e.g.:
	//   token := r.Header.Get("X-QB-Token-bqr1111")
	tempToken := "token-from-browser"

	client, err := quickbase.New("myrealm",
		quickbase.WithTempTokenAuth(
			auth.WithInitialTempToken(tempToken),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Use the client to make API calls to QuickBase
	_ = client
}

// Configure a client with multiple options for production use.
func ExampleNew_withOptions() {
	client, err := quickbase.New("myrealm",
		quickbase.WithUserToken("b9f3pk_xxxx_xxxxxxxxxxxxxxx"),

		// Retry configuration
		quickbase.WithMaxRetries(5),
		quickbase.WithRetryDelay(time.Second),
		quickbase.WithMaxRetryDelay(30*time.Second),

		// Timeout
		quickbase.WithTimeout(60*time.Second),

		// Connection pool for high-throughput
		quickbase.WithMaxIdleConnsPerHost(10),

		// Proactive rate limiting
		quickbase.WithProactiveThrottle(100),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

// Monitor all API requests for latency tracking and debugging.
func ExampleWithOnRequest() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
			fmt.Printf("%s %s → %d (%dms)\n",
				info.Method,
				info.Path,
				info.StatusCode,
				info.Duration.Milliseconds(),
			)

			// Debug failed requests by inspecting the body
			if info.StatusCode >= 400 {
				fmt.Printf("Request body: %s\n", info.RequestBody)
			}
		}),
	)
	_ = client
}

// Track retry attempts for debugging transient failures.
func ExampleWithOnRetry() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithOnRetry(func(info quickbase.RetryInfo) {
			fmt.Printf("Retrying %s %s (attempt %d, reason: %s, wait: %v)\n",
				info.Method,
				info.Path,
				info.Attempt,
				info.Reason,
				info.WaitTime,
			)
		}),
	)
	_ = client
}

// Get notified when rate limited by QuickBase.
func ExampleWithOnRateLimit() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithOnRateLimit(func(info quickbase.RateLimitInfo) {
			fmt.Printf("Rate limited! Retry after %d seconds (Ray ID: %s)\n",
				info.RetryAfter,
				info.QBAPIRay,
			)
		}),
	)
	_ = client
}

// Enable proactive throttling to avoid 429 errors.
// QuickBase allows 100 requests per 10 seconds per user token.
func ExampleWithProactiveThrottle() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithProactiveThrottle(100), // 100 req/10s
	)
	_ = client
}

// Configure connection pool for high-throughput batch operations.
func ExampleWithMaxIdleConnsPerHost() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		// Allow 10 concurrent connections (default is 6)
		quickbase.WithMaxIdleConnsPerHost(10),
		// Pair with throttling for safe high-throughput
		quickbase.WithProactiveThrottle(100),
	)
	_ = client
}

// Query records from a table - returns first page only.
func ExampleClient_RunQuery() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
	)

	ctx := context.Background()
	result, err := client.RunQuery(ctx, quickbase.RunQueryBody{
		From:   "bqxyz123",                     // table ID
		Select: quickbase.Ints(3, 6, 7),        // field IDs
		Where:  quickbase.Ptr("{6.GT.100}"),    // optional filter
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Got %d of %d total records\n",
		result.Metadata.NumRecords,
		result.Metadata.TotalRecords,
	)
}

// Fetch ALL records from a table with automatic pagination.
func ExampleClient_RunQueryAll() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
	)

	ctx := context.Background()
	records, err := client.RunQueryAll(ctx, quickbase.RunQueryBody{
		From:   "bqxyz123",
		Select: quickbase.Ints(3, 6, 7),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Fetched %d records\n", len(records))
}

// Fetch up to N records with automatic pagination.
func ExampleClient_RunQueryN() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
	)

	ctx := context.Background()
	records, err := client.RunQueryN(ctx, quickbase.RunQueryBody{
		From:   "bqxyz123",
		Select: quickbase.Ints(3, 6, 7),
	}, 500) // max 500 records
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Fetched %d records\n", len(records))
}

// Handle different error types from API responses.
func ExampleClient_GetApp_errorHandling() {
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
	)

	ctx := context.Background()
	app, err := client.GetApp("invalid-app-id").Run(ctx)
	if err != nil {
		var rateLimitErr *quickbase.RateLimitError
		var notFoundErr *quickbase.NotFoundError
		var validationErr *quickbase.ValidationError
		var authErr *quickbase.AuthenticationError

		switch {
		case errors.As(err, &rateLimitErr):
			fmt.Printf("Rate limited. Retry after %d seconds\n", rateLimitErr.RetryAfter)
		case errors.As(err, &notFoundErr):
			fmt.Println("App not found")
		case errors.As(err, &validationErr):
			fmt.Printf("Validation error: %s\n", validationErr.Message)
		case errors.As(err, &authErr):
			fmt.Println("Authentication failed - check your token")
		default:
			fmt.Printf("Error: %v\n", err)
		}
		return
	}

	fmt.Printf("App: %s\n", app.Name)
}

// Use the Ptr helper to create pointers for optional fields.
func ExamplePtr() {
	// For optional string fields
	where := quickbase.Ptr("{6.GT.100}")

	// For optional int fields
	skip := quickbase.Ptr(100)

	_, _ = where, skip
}

// Use the Ints helper to create field ID slices for Select.
func ExampleInts() {
	body := quickbase.RunQueryBody{
		From:   "bqxyz123",
		Select: quickbase.Ints(3, 6, 7, 8, 9), // cleaner than &[]int{3, 6, 7, 8, 9}
	}
	_ = body
}

// Use schema aliases for readable table and field names.
func ExampleWithSchema() {
	// Define schema with readable names
	schema := quickbase.NewSchema().
		Table("projects", "bqxyz123").
		Field("recordId", 3).
		Field("name", 6).
		Field("status", 7).
		Build()

	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithSchema(schema),
	)

	ctx := context.Background()
	result, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
		From:   "projects",                                             // alias instead of "bqxyz123"
		Select: quickbase.Fields(schema, "projects", "name", "status"), // aliases for select
		Where:  quickbase.Ptr("{'status'.EX.'Active'}"),                // aliases in where
	})

	// Response data uses aliases too
	for _, record := range result.Data {
		fmt.Printf("Name: %v, Status: %v\n", record["name"], record["status"])
	}
}

// Use Fields helper to resolve field aliases to IDs.
func ExampleFields() {
	schema := quickbase.NewSchema().
		Table("projects", "bqxyz123").
		Field("recordId", 3).
		Field("name", 6).
		Field("status", 7).
		Build()

	// Fields returns *[]int for use in Select
	fieldIds := quickbase.Fields(schema, "projects", "recordId", "name", "status")
	fmt.Printf("Field IDs: %v\n", *fieldIds)
	// Output: Field IDs: [3 6 7]
}

// Build schema using the fluent builder API.
func ExampleNewSchema() {
	schema := quickbase.NewSchema().
		Table("projects", "bqxyz123").
		Field("recordId", 3).
		Field("name", 6).
		Field("status", 7).
		Table("tasks", "bqabc456").
		Field("recordId", 3).
		Field("title", 8).
		Field("projectId", 9).
		Build()

	fmt.Printf("Tables: %d\n", len(schema.Tables))
	// Output: Tables: 2
}
