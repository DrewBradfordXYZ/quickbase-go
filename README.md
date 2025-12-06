# QuickBase Go SDK

A Go client for the QuickBase JSON RESTful API, with optional support for legacy XML API endpoints.

[![Go Reference](https://pkg.go.dev/badge/github.com/DrewBradfordXYZ/quickbase-go.svg)](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go)

## Features

- **Fluent Builders** - `client.GetApp(appId).Run(ctx)`, `client.CreateApp().Name("My App").Run(ctx)`
- **Friendly Result Types** - Clean structs with dereferenced fields instead of pointer-heavy generated types
- **Query Builder** - `client.Query("table").Select().Where().Run(ctx)`
- **Schema Aliases** - Use readable names (`"projects"`, `"name"`) instead of IDs (`"bqxyz123"`, `6`)
- **Fluent Schema Builder** - `NewSchema().Table().Field().Build()` for schema definition
- **Automatic Pagination** - `RunQueryAll` fetches all records across pages
- **Helper Functions** - `Row()`, `Value()`, `Fields()`, `Asc()`, `Desc()`, `Ptr()`, `Ints()`
- **Multiple Auth Methods** - User token, temporary token, SSO, and ticket (username/password)
- **Automatic Retry** - Exponential backoff with jitter for rate limits and server errors
- **Proactive Throttling** - Prevents 429 errors with sliding window rate limiting
- **Typed Errors** - `RateLimitError`, `NotFoundError`, `ValidationError`, etc.
- **Monitoring Hooks** - Track request latency, retries, and errors
- **Full API Access** - Low-level generated client available via `client.API()`
- **Legacy XML API** - Optional `xml` sub-package for endpoints with no JSON equivalent (roles, schema)

## Installation

```bash
go get github.com/DrewBradfordXYZ/quickbase-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/DrewBradfordXYZ/quickbase-go"
)

func main() {
    // Create client with user token
    client, err := quickbase.New("your-realm",
        quickbase.WithUserToken("your-user-token"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Get app details (fluent builder pattern)
    app, err := client.GetApp("your-app-id").Run(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("App name:", app.Name)

    // Query all records from a table
    records, err := client.RunQueryAll(ctx, quickbase.RunQueryBody{
        From:   "your-table-id",
        Select: quickbase.Ints(3, 6, 7),
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found %d records\n", len(records))
}
```

## Authentication

### User Token (Recommended for Server-Side)

```go
client, err := quickbase.New("mycompany",
    quickbase.WithUserToken("b9f3pk_xxx_xxxxxxxxxxxxxx"),
)
```

Generate a user token at: `https://YOUR-REALM.quickbase.com/db/main?a=UserTokens`

### Temporary Token (Browser-Initiated)

Temp tokens are short-lived (~5 min), table-scoped tokens that verify a user is logged into QuickBase. Unlike the JS SDK which can fetch temp tokens using browser cookies, **Go servers receive temp tokens from browser clients** (e.g., Code Pages).

**How it works:**
1. User opens a Code Page in QuickBase (logged in)
2. Browser JavaScript fetches temp token using session cookies
3. Browser sends request to Go server with token in header (e.g., X-QB-Token-{dbid})
4. Go server extracts token and makes API calls back to QuickBase

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Extract tokens from request headers
    tokens := map[string]string{
        "bqr1111": r.Header.Get("X-QB-Token-bqr1111"),
    }

    // Create a client with the received tokens
    client, err := quickbase.New("myrealm",
        quickbase.WithTempTokens(tokens),
    )
    if err != nil {
        http.Error(w, "Failed to create client", http.StatusInternalServerError)
        return
    }

    // Use the client to make API calls back to QuickBase
    app, err := client.GetApp("bqr1111").Run(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    fmt.Fprintf(w, "App: %s", app.Name)
}
```

**Why use temp tokens?**
- Verifies the user is actually logged into QuickBase (via their browser session)
- Table-scoped (more restrictive than user tokens)
- No need to store user credentials on your server

### Ticket Auth (Username/Password)

Ticket authentication lets users log in with their QuickBase email and password. Unlike user tokens, tickets properly attribute record changes (`createdBy`/`modifiedBy`) to the authenticated user.

```go
client, err := quickbase.New("mycompany",
    quickbase.WithTicketAuth("user@example.com", "password"),
)
```

**Key behaviors:**
- Authentication happens lazily on the first API call
- Password is discarded from memory after authentication
- Tickets are valid for 12 hours by default (configurable up to ~6 months)
- When the ticket expires, an error is returned — create a new client with fresh credentials

**With custom ticket validity:**

```go
import "github.com/DrewBradfordXYZ/quickbase-go/auth"

client, err := quickbase.New("mycompany",
    quickbase.WithTicketAuth("user@example.com", "password",
        auth.WithTicketHours(24*7), // 1 week
    ),
)
```

**When to use ticket auth:**
- Third-party services where users shouldn't share user tokens
- Proper audit trails with correct `createdBy`/`modifiedBy` attribution
- Session-based authentication flows

**Signing out:**

```go
// Clear credentials from memory (e.g., when user logs out)
client.SignOut()

// After SignOut, API calls will fail.
// Create a new client with fresh credentials to continue.
```

Note: `SignOut()` clears credentials from local memory only. QuickBase doesn't provide a server-side ticket revocation API — the XML `API_SignOut` endpoint is designed for browser cookie clearing and redirects, not ticket invalidation. Tickets remain valid until they expire naturally.

### SSO Token (SAML)

SSO authentication lets your Go server make API calls *as a specific QuickBase user* rather than a shared service account. This is valuable when:

- **Audit accuracy matters** - Fields like "Created By" and "Modified By" show the actual user, not a service account
- **Security is critical** - No long-lived user token to leak; each user gets a short-lived token tied to their SSO session
- **Per-user permissions** - API calls respect each user's individual QuickBase permissions

**How it works:**

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Your IdP       │     │   Your Go       │     │   QuickBase     │
│  (Okta, Azure)  │     │   Server        │     │   API           │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │                       │
        │  1. User logs into    │                       │
        │     your app via SSO  │                       │
        │──────────────────────►│                       │
        │                       │                       │
        │  2. Generate SAML     │                       │
        │     assertion for     │                       │
        │     this user         │                       │
        │◄──────────────────────│                       │
        │                       │                       │
        │  SAML assertion       │                       │
        │──────────────────────►│                       │
        │                       │                       │
        │                       │  3. Exchange SAML     │
        │                       │     for temp token    │
        │                       │──────────────────────►│
        │                       │                       │
        │                       │  4. API calls as      │
        │                       │     that user         │
        │                       │──────────────────────►│
```

**Prerequisites:**
- Your QuickBase realm has SAML SSO configured
- You can generate SAML assertions from your IdP (Okta API, Azure AD, etc.)

**Usage:**

```go
// Get SAML assertion from your identity provider for this user
samlAssertion := getAssertionFromIdP(userId) // base64url-encoded

client, err := quickbase.New("mycompany",
    quickbase.WithSSOTokenAuth(samlAssertion),
)

// API calls are now made as that specific user
// "Created By" fields will show their name, not a service account
```

The SDK exchanges the SAML assertion for a QuickBase temp token using [RFC 8693 token exchange](https://developer.quickbase.com/operation/exchangeSsoToken).

## Configuration Options

```go
client, err := quickbase.New("mycompany",
    quickbase.WithUserToken("token"),

    // Retry settings
    quickbase.WithMaxRetries(5),              // Default: 3
    quickbase.WithRetryDelay(time.Second),    // Default: 1s
    quickbase.WithMaxRetryDelay(30*time.Second), // Default: 30s

    // Request timeout
    quickbase.WithTimeout(60*time.Second),    // Default: 30s

    // Connection pool (for high-throughput, see "High-Throughput Configuration")
    quickbase.WithMaxIdleConnsPerHost(10),    // Default: 6

    // Proactive rate limiting (100 req/10s is QuickBase's limit)
    quickbase.WithProactiveThrottle(100),

    // Debug logging
    quickbase.WithDebug(true),

    // Rate limit callback
    quickbase.WithOnRateLimit(func(info quickbase.RateLimitInfo) {
        log.Printf("Rate limited! Retry after %ds", info.RetryAfter)
    }),
)
defer client.Close()  // Release idle connections when done
```

## Schema Aliases

Use readable names for tables and fields instead of cryptic IDs. The SDK transforms aliases to IDs in requests and IDs back to aliases in responses.

### Defining a Schema

**Using the fluent builder (recommended):**

```go
schema := quickbase.NewSchema().
    Table("projects", "bqw3ryzab").
        Field("id", 3).
        Field("name", 6).
        Field("status", 7).
        Field("dueDate", 12).
        Field("assignee", 15).
    Table("tasks", "bqw4xyzcd").
        Field("id", 3).
        Field("title", 6).
        Field("projectId", 8).
        Field("completed", 10).
    Build()

client, err := quickbase.New("mycompany",
    quickbase.WithUserToken("token"),
    quickbase.WithSchema(schema),
)
```

**Using a struct (alternative):**

```go
schema := &quickbase.Schema{
    Tables: map[string]quickbase.TableSchema{
        "projects": {
            ID: "bqw3ryzab",
            Fields: map[string]int{
                "id":       3,
                "name":     6,
                "status":   7,
                "dueDate":  12,
                "assignee": 15,
            },
        },
    },
}
```

### Using Aliases in Queries

```go
// Use table and field aliases instead of IDs
result, err := client.RunQuery(ctx, quickbase.RunQueryBody{
    From:   "projects",                                              // Table alias
    Select: quickbase.Fields(schema, "projects", "name", "status"),  // table, then fields
    Where:  quickbase.Ptr("{'status'.EX.'Active'}"),                 // Aliases in where
})

// Response uses aliases and values are automatically unwrapped
for _, record := range result.Data {
    name := record["name"]     // "Project Alpha" (not map[value:Project Alpha])
    status := record["status"] // "Active"
}
```

### The Fields() Helper

Since `Select` expects `*[]int`, use `Fields()` to resolve aliases to IDs:

```go
// Returns *[]int{6, 7} for use in Select
quickbase.Fields(schema, "projects", "name", "status")

// You can also mix with Ints() if you prefer numeric IDs
quickbase.Ints(3, 6, 7)
```

### Upserting with Aliases

Use field aliases in record data with the `Row()` helper:

```go
data := []quickbase.Record{
    quickbase.Row("name", "New Project", "status", "Active"),
}

result, err := client.Upsert(ctx, quickbase.UpsertBody{
    To:   "projects",  // Table alias
    Data: &data,
})
```

The schema transforms `"name"` → `"6"` and `"status"` → `"7"` before sending to the API.

### Response Transformation

Responses are automatically transformed:
- Field ID keys (`"6"`) become aliases (`name`)
- Values are unwrapped from `{"value": X}` to just `X`
- Unknown fields (not in schema) keep their numeric key but are still unwrapped

```go
// Raw API response:
// {"data": [{"6": {"value": "Alpha"}, "99": {"value": "Custom"}}]}

// Transformed response (with schema):
// {"data": [{"name": "Alpha", "99": "Custom"}]}
```

### Disabling Response Transformation

If you prefer to keep field IDs in responses (e.g., for backwards compatibility), use `WithSchemaOptions`:

```go
client, err := quickbase.New("mycompany",
    quickbase.WithUserToken("token"),
    quickbase.WithSchemaOptions(schema, quickbase.SchemaOptions{
        TransformResponses: false,  // Keep field IDs, only unwrap values
    }),
)

// Now responses use field IDs: record["6"] instead of record["name"]
```

### Helpful Error Messages

Typos in aliases return errors with suggestions:

```go
// Returns error: unknown field alias 'stauts' in table 'projects'. Did you mean 'status'?
_, err := client.RunQuery(ctx, quickbase.RunQueryBody{
    From:  "projects",
    Where: quickbase.Ptr("{'stauts'.EX.'Active'}"), // Typo!
})
```

### Generating Schema from QuickBase

Use the CLI to generate a schema from an existing app:

```bash
# Generate Go schema to stdout
go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN"

# Generate and save to file
go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN" -o schema.go

# Generate JSON format
go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN" -f json -o schema.json
```

### Updating Schema with --merge

When your QuickBase app changes (new tables, new fields), use `--merge` to update your schema while preserving any custom aliases you've set:

```bash
# Update schema, preserving custom aliases
go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN" -o schema.go --merge
```

**What merge does:**
- Preserves your custom table and field aliases (matched by ID, not name)
- Adds new tables and fields with auto-generated aliases
- Reports what changed:

```
Merge complete:
  Tables: 2 preserved, 1 added, 0 removed
  Fields: 15 preserved, 3 added, 0 removed
```

This lets you rename auto-generated aliases like `dateCreated` to `created` and keep them through updates.

### Loading Schema from JSON

Store your schema in a JSON file and load it at runtime:

```json
// schema.json
{
  "tables": {
    "projects": {
      "id": "bqw3ryzab",
      "fields": {
        "id": 3,
        "name": 6,
        "status": 7
      }
    }
  }
}
```

```go
import (
    "encoding/json"
    "os"

    "github.com/DrewBradfordXYZ/quickbase-go"
)

// Load schema from JSON file
data, err := os.ReadFile("schema.json")
if err != nil {
    log.Fatal(err)
}

var schema quickbase.Schema
if err := json.Unmarshal(data, &schema); err != nil {
    log.Fatal(err)
}

client, err := quickbase.New("mycompany",
    quickbase.WithUserToken("token"),
    quickbase.WithSchema(&schema),
)
```

## Query Builder

The fluent query builder eliminates repetition when using schema aliases:

```go
// Fluent builder - table specified once
result, err := client.Query("projects").
    Select("name", "status", "dueDate").
    Where("{'status'.EX.'Active'}").
    SortBy(quickbase.Asc("name"), quickbase.Desc("dueDate")).
    Options(100, 0).  // top, skip
    Run(ctx)

// Fetch all records with automatic pagination
records, err := client.Query("projects").
    Select("name", "status").
    RunAll(ctx)
```

### Query Methods

```go
qb := client.Query("projects")           // Start query for table (alias or ID)
    .Select("name", "status")            // Fields to return (aliases or IDs)
    .Where("{'status'.EX.'Active'}")     // Filter clause
    .SortBy(quickbase.Asc("name"))       // Sort order
    .GroupBy("status")                   // Group by fields
    .Options(100, 0)                     // Pagination (top, skip)
    .Run(ctx)                            // Execute and return first page
    .RunAll(ctx)                         // Execute and return all pages
```

### Upsert Builder

```go
result, err := client.UpsertTo("projects").
    MergeOn("externalId").                                    // Merge field for updates
    Row("externalId", "EXT-001", "name", "Alpha", "status", "Active").
    Row("externalId", "EXT-002", "name", "Beta", "status", "Pending").
    Return("name", "status").                                 // Fields to return
    Run(ctx)

fmt.Printf("Created: %v, Updated: %v\n",
    result.CreatedRecordIDs, result.UpdatedRecordIDs)
```

### Asc/Desc Helpers

The `Asc()` and `Desc()` helpers accept both field IDs and aliases:

```go
// With field IDs (no schema needed)
quickbase.Asc(6)
quickbase.Desc(7)

// With aliases (schema configured)
quickbase.Asc("name")
quickbase.Desc("dueDate")
```

## API Usage

The SDK provides fluent builders with friendly result types:

```go
ctx := context.Background()

// Get app details - returns GetAppResult with dereferenced fields
app, err := client.GetApp(appId).Run(ctx)
fmt.Println("App name:", app.Name)
fmt.Println("Created:", app.Created)  // string, not *string

// Get table info - returns TableInfo
table, err := client.GetTable(tableId).Run(ctx)
fmt.Println("Table:", table.Name, "Alias:", table.Alias)

// Get all tables in an app - returns []TableInfo
tables, err := client.GetAppTables(appId).Run(ctx)
for _, t := range tables {
    fmt.Printf("Table %s: %s\n", t.ID, t.Name)
}

// Get fields for a table - returns []FieldDetails
fields, err := client.GetFields(tableId).Run(ctx)
for _, f := range fields {
    fmt.Printf("Field %d: %s (%s)\n", f.ID, f.Label, f.FieldType)
}

// Query records - IMPORTANT: QuickBase returns ~100 records per page by default
// Use RunQueryAll to get all records, or RunQueryN to limit

// Single page only
result, err := client.RunQuery(ctx, quickbase.RunQueryBody{
    From:   tableId,
    Select: quickbase.Ints(3, 6, 7),
    Where:  quickbase.Ptr("{6.GT.100}"),
})

// All records (auto-paginates)
allRecords, err := client.RunQueryAll(ctx, quickbase.RunQueryBody{
    From: tableId,
})

// Up to N records (auto-paginates)
first500, err := client.RunQueryN(ctx, quickbase.RunQueryBody{
    From: tableId,
}, 500)

// Insert/Update records - returns UpsertResult
data := []quickbase.Record{
    quickbase.Row("name", "New Record", "count", 42),
}
upsertResult, err := client.Upsert(tableId).Data(&data).Run(ctx)
fmt.Println("Created:", upsertResult.CreatedRecordIDs)
fmt.Println("Updated:", upsertResult.UpdatedRecordIDs)

// Delete records - returns DeleteRecordsResult
deleteResult, err := client.DeleteRecords(tableId).Where("{3.EX.123}").Run(ctx)
fmt.Println("Deleted:", deleteResult.NumberDeleted)

// Create an app - returns GetAppResult
newApp, err := client.CreateApp().
    Name("My New App").
    Description("Created via API").
    Run(ctx)
fmt.Println("Created app:", newApp.ID)

// Get report info - returns ReportInfo
report, err := client.GetReport(tableId, reportId).Run(ctx)
fmt.Println("Report:", report.Name, "Type:", report.Type)
```

### Friendly Result Types

The SDK provides clean result types that dereference pointers and flatten nested structures:

| Operation | Result Type | Key Fields |
|-----------|-------------|------------|
| `GetApp`, `CreateApp`, `UpdateApp`, `CopyApp` | `GetAppResult` | ID, Name, Description, Created, Updated, DateFormat, TimeZone |
| `GetTable`, `GetAppTables` | `TableInfo` | ID, Name, Alias, Description, NextRecordID, KeyFieldID, etc. |
| `GetFields` | `[]FieldDetails` | ID, Label, FieldType |
| `GetReport`, `GetTableReports` | `ReportInfo` | ID, Name, Type, Description, OwnerID, UsedCount |
| `Upsert` | `UpsertResult` | CreatedRecordIDs, UpdatedRecordIDs, UnchangedRecordIDs, TotalNumberOfRecordsProcessed |
| `DeleteRecords` | `DeleteRecordsResult` | NumberDeleted |

These result types have non-pointer fields with sensible defaults for missing values, making them much easier to work with than the raw generated types.

### Helper Functions

```go
// Row creates a Record from key-value pairs (most concise)
quickbase.Row("name", "Alice", "age", 30, "active", true)
quickbase.Row(6, "Alice", 7, 30)  // also works with field IDs

// Value creates a FieldValue for upserts (when not using Row)
quickbase.Value("text value")
quickbase.Value(123)
quickbase.Value(true)
quickbase.Value([]string{"a", "b"})  // multi-select

// Fields resolves aliases to IDs for Select (requires schema)
quickbase.Fields(schema, "projects", "name", "status")  // returns *[]int{6, 7}

// Sorting helpers
quickbase.SortBy(quickbase.Asc(6), quickbase.Desc(7))  // sortBy parameter
quickbase.Asc(6)   // ascending by field 6
quickbase.Desc(7)  // descending by field 7

// Query options
quickbase.Options(100, 0)  // top=100, skip=0

// GroupBy helper
quickbase.GroupBy(6, 7)  // group by fields 6 and 7

// Ptr returns a pointer (for optional string/int fields)
quickbase.Ptr("some string")
quickbase.Ptr(123)

// Ints returns *[]int (for Select fields)
quickbase.Ints(3, 6, 7)

// Strings returns *[]string
quickbase.Strings("a", "b", "c")
```

### Available Methods

All QuickBase API endpoints are available as wrapper methods:

| Category | Methods |
|----------|---------|
| **Records** | `RunQuery`, `RunQueryAll`, `RunQueryN`, `Upsert`, `DeleteRecords` |
| **Apps** | `GetApp`, `CreateApp`, `UpdateApp`, `DeleteApp`, `CopyApp`, `GetAppEvents` |
| **Tables** | `GetTable`, `GetAppTables`, `CreateTable`, `UpdateTable`, `DeleteTable` |
| **Fields** | `GetField`, `GetFields`, `CreateField`, `UpdateField`, `DeleteFields`, `GetFieldUsage`, `GetFieldsUsage` |
| **Relationships** | `GetRelationships`, `CreateRelationship`, `UpdateRelationship`, `DeleteRelationship` |
| **Reports** | `GetReport`, `GetTableReports`, `RunReport` |
| **Files** | `DownloadFile`, `DeleteFile` |
| **Users** | `GetUsers`, `DenyUsers`, `UndenyUsers`, `DenyUsersAndGroups` |
| **Groups** | `AddMembersToGroup`, `RemoveMembersFromGroup`, `AddManagersToGroup`, `RemoveManagersFromGroup`, `AddSubgroupsToGroup`, `RemoveSubgroupsFromGroup` |
| **User Tokens** | `CloneUserToken`, `DeleteUserToken`, `DeactivateUserToken`, `TransferUserToken` |
| **Other** | `RunFormula`, `Audit`, `GenerateDocument` |

### Low-Level API Access

For endpoints not covered by wrapper methods or when you need full control, access the generated API directly:

```go
resp, err := client.API().GetAppWithResponse(ctx, appId)
if resp.JSON200 != nil {
    fmt.Println(resp.JSON200.Name)
}
```

## Error Handling

The SDK provides specific error types for different HTTP status codes:

```go
app, err := client.GetApp("invalid-id").Run(ctx)
if err != nil {
    var rateLimitErr *quickbase.RateLimitError
    var notFoundErr *quickbase.NotFoundError
    var validationErr *quickbase.ValidationError

    switch {
    case errors.As(err, &rateLimitErr):
        log.Printf("Rate limited. Retry after %d seconds", rateLimitErr.RetryAfter)
    case errors.As(err, &notFoundErr):
        log.Println("Resource not found")
    case errors.As(err, &validationErr):
        log.Printf("Validation error: %s", validationErr.Message)
    default:
        log.Printf("Error: %v", err)
    }
}
```

Available error types:
- `RateLimitError` - HTTP 429
- `AuthenticationError` - HTTP 401
- `AuthorizationError` - HTTP 403
- `NotFoundError` - HTTP 404
- `ValidationError` - HTTP 400
- `ServerError` - HTTP 5xx
- `TimeoutError` - Request timeout

## Rate Limiting

QuickBase enforces a rate limit of **100 requests per 10 seconds** per user token. This SDK follows [QuickBase's official rate limiting guidance](https://developer.quickbase.com/rateLimit) — relying on server-side `Retry-After` headers by default, with optional client-side throttling.

### How 429 Errors Are Handled

When the SDK receives a 429 (Too Many Requests) response, it automatically:

1. **Extracts rate limit info** from response headers (`Retry-After`, `cf-ray`, `qb-api-ray`)
2. **Calls the `onRateLimit` callback** if configured, allowing you to log or monitor
3. **Waits before retrying** - uses the `Retry-After` header if present, otherwise exponential backoff with jitter
4. **Retries the request** up to `maxRetries` times (default: 3)
5. **Returns a `RateLimitError`** if all retries are exhausted

```
Request fails with 429
        ↓
Extract Retry-After header
        ↓
Call onRateLimit callback (if set)
        ↓
Wait (Retry-After or exponential backoff)
        ↓
Retry request (up to maxRetries)
        ↓
Return RateLimitError if exhausted
```

### Retry Configuration

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithMaxRetries(5),              // Default: 3
    quickbase.WithRetryDelay(time.Second),    // Initial delay, default: 1s
    quickbase.WithMaxRetryDelay(30*time.Second), // Max delay, default: 30s
    quickbase.WithBackoffMultiplier(2.0),     // Exponential multiplier, default: 2
)
```

The backoff formula with jitter: `delay = initialDelay * (multiplier ^ attempt) ± 10%`

### Proactive Throttling

Prevent 429 errors entirely by throttling requests client-side using a sliding window algorithm:

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithProactiveThrottle(100), // 100 req/10s
)
```

This tracks request timestamps and blocks new requests when the limit would be exceeded, waiting until the oldest request exits the 10-second window.

### Rate Limit Callback

Get notified when rate limited (called before retry):

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithOnRateLimit(func(info quickbase.RateLimitInfo) {
        log.Printf("Rate limited on %s", info.RequestURL)
        log.Printf("Retry after: %d seconds", info.RetryAfter)
        log.Printf("Ray ID: %s", info.QBAPIRay)
        log.Printf("Attempt: %d", info.Attempt)
    }),
)
```

### Handling RateLimitError

If retries are exhausted, a `*RateLimitError` is returned:

```go
app, err := client.GetApp(appId).Run(ctx)
if err != nil {
    var rateLimitErr *quickbase.RateLimitError
    if errors.As(err, &rateLimitErr) {
        log.Printf("Rate limited after %d attempts", rateLimitErr.RateLimitInfo.Attempt)
        log.Printf("Retry after: %d seconds", rateLimitErr.RetryAfter)
    }
}
```

## High-Throughput Configuration

For batch operations like bulk imports, exports, or report generation, you may want to tune both connection pooling and throttling to maximize throughput while staying within rate limits.

### Understanding the Interaction

| Setting | What it controls | Default |
|---------|-----------------|---------|
| `WithMaxIdleConnsPerHost` | How many concurrent requests *can* be in flight | 6 |
| `WithProactiveThrottle` | How many requests *should* be made per 10 seconds | disabled |

**Without throttling:** 6 connections with ~100ms latency = ~60 requests/second possible, but QuickBase only allows 10 req/s sustained (100/10s). You'll burst, hit 429s, wait, repeat.

**With throttling:** Requests are spread evenly across the 10-second window, avoiding 429s entirely.

### Recommended Configuration for Batch Operations

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),

    // Allow more concurrent connections for parallel requests
    quickbase.WithMaxIdleConnsPerHost(10),

    // Spread requests to avoid 429 errors
    quickbase.WithProactiveThrottle(100),
)
```

This allows up to 10 requests in parallel while ensuring you never exceed 100 requests per 10 seconds.

### Connection Pool Settings

```go
// Connection pool tuning (optional)
quickbase.WithMaxIdleConnsPerHost(10), // Concurrent connections (default: 6)
quickbase.WithMaxIdleConns(100),       // Total pool size (default: 100)
quickbase.WithIdleConnTimeout(2*time.Minute), // Keep connections warm
```

**When to increase `MaxIdleConnsPerHost`:**
- Bulk record operations (importing/exporting thousands of records)
- Fetching data from multiple tables concurrently
- Report generation hitting multiple endpoints

**Why the default is 6:** This matches browser standards and handles typical concurrent patterns (e.g., fetching app metadata + tables + fields simultaneously) without encouraging excessive parallelism.

## Monitoring

The SDK provides hooks for observability, allowing you to track request latency, errors, and retries for dashboards, logging, or metrics collection.

### Request Hook

Track every API request:

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
        log.Printf("%s %s → %d (%dms)",
            info.Method,
            info.Path,
            info.StatusCode,
            info.Duration.Milliseconds(),
        )
    }),
)
```

Output:
```
POST /v1/records/query → 200 (142ms)
GET /v1/apps/bqxyz123 → 200 (87ms)
POST /v1/records/query → 429 (12ms)
```

`RequestInfo` fields:
| Field | Type | Description |
|-------|------|-------------|
| `Method` | string | HTTP method (GET, POST, etc.) |
| `Path` | string | URL path (e.g., /v1/apps/bqxyz123) |
| `StatusCode` | int | HTTP status code (0 if network error) |
| `Duration` | time.Duration | Request latency |
| `Attempt` | int | Attempt number (1 = first try, 2+ = retries) |
| `Error` | error | Non-nil if request failed |
| `RequestBody` | []byte | Request body (for debugging failed requests) |

**Debugging failed requests:**

```go
quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
    if info.StatusCode >= 400 {
        log.Printf("Request failed: %s %s → %d\nBody: %s",
            info.Method, info.Path, info.StatusCode, info.RequestBody)
    }
})
```

### Retry Hook

Track retry attempts:

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithOnRetry(func(info quickbase.RetryInfo) {
        log.Printf("Retrying %s %s (attempt %d, reason: %s, wait: %v)",
            info.Method, info.Path, info.Attempt, info.Reason, info.WaitTime)
    }),
)
```

`RetryInfo` fields:
| Field | Type | Description |
|-------|------|-------------|
| `Method` | string | HTTP method |
| `Path` | string | URL path |
| `Attempt` | int | Which attempt is coming next (2 = first retry) |
| `Reason` | string | Why retrying: "429", "503", "network error" |
| `WaitTime` | time.Duration | How long until retry |

### Prometheus Example

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "quickbase_request_duration_seconds",
            Buckets: []float64{.05, .1, .25, .5, 1, 2.5},
        },
        []string{"method", "path", "status"},
    )
    retryTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "quickbase_retries_total"},
        []string{"reason"},
    )
)

client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
        requestDuration.WithLabelValues(
            info.Method, info.Path, strconv.Itoa(info.StatusCode),
        ).Observe(info.Duration.Seconds())
    }),
    quickbase.WithOnRetry(func(info quickbase.RetryInfo) {
        retryTotal.WithLabelValues(info.Reason).Inc()
    }),
)
```

## Pagination

**Important:** QuickBase API endpoints like `RunQuery` do **not** return all records by default. They return a single page (typically ~100 records depending on record size). If you have 1,000 records and call `RunQuery` once, you'll only get the first ~100.

### Available Methods

| Method | Description |
|--------|-------------|
| `RunQuery(ctx, body)` | Single page only (~100 records) |
| `RunQueryAll(ctx, body)` | All records (auto-paginates) |
| `RunQueryN(ctx, body, n)` | Up to N records (auto-paginates) |
| `client.Paginate(ctx, fetcher)` | Iterator for memory-efficient streaming |
| `client.CollectAll(ctx, fetcher)` | Low-level: collect all into slice |
| `client.CollectN(ctx, fetcher, n)` | Low-level: collect up to N |

### Simple: Use RunQueryAll

The easiest way to get all records:

```go
// Fetches ALL records automatically (handles pagination internally)
allRecords, err := client.RunQueryAll(ctx, quickbase.RunQueryBody{
    From:   tableId,
    Select: quickbase.Ints(3, 6, 7),
})
fmt.Printf("Fetched %d records\n", len(allRecords))
```

### Fetch Limited Records

```go
// Fetch up to 500 records (across multiple pages if needed)
records, err := client.RunQueryN(ctx, body, 500)
```

### Single Page (Default)

```go
// RunQuery returns just the first page
result, err := client.RunQuery(ctx, body)
fmt.Printf("Got %d of %d total records\n",
    result.Metadata.NumRecords,
    result.Metadata.TotalRecords)
```

### Advanced: Manual Pagination

For custom pagination logic, use the low-level helpers:

```go
import "github.com/DrewBradfordXYZ/quickbase-go/client"

// Define a page fetcher
fetcher := func(ctx context.Context, skip int, nextToken string) (*Response, error) {
    // Your custom fetch logic
}

// Iterate over records (memory-efficient for large datasets)
for record, err := range client.Paginate(ctx, fetcher) {
    if err != nil {
        log.Fatal(err)
    }
    // Process each record
}
```

### Pagination Types

QuickBase uses two pagination styles depending on the endpoint:

- **Skip-based**: Uses `skip` parameter (e.g., `RunQuery`)
- **Token-based**: Uses `nextPageToken` or `nextToken` (e.g., `GetUsers`, `GetAuditLogs`)

The SDK auto-detects which style to use based on the response metadata.

## Legacy XML API

The QuickBase JSON API doesn't expose some endpoints available in the legacy XML API, particularly for **roles** and **comprehensive schema information**. The optional `xml` sub-package provides access to these endpoints while reusing the main client's authentication, retry, and throttling infrastructure.

> **Note:** The XML API is legacy and may be discontinued by QuickBase in the future. Use JSON API methods where possible. This sub-package will be removed when QuickBase discontinues the XML API.

### Installation

The `xml` package is included with the SDK but imported separately:

```go
import (
    "github.com/DrewBradfordXYZ/quickbase-go"
    "github.com/DrewBradfordXYZ/quickbase-go/xml"
)
```

### Usage

```go
// Create main client (JSON API)
qb, err := quickbase.New("myrealm", quickbase.WithUserToken("token"))
if err != nil {
    log.Fatal(err)
}

// Create XML client from main client
xmlClient := xml.New(qb)

ctx := context.Background()

// Get all roles defined in an app
roles, err := xmlClient.GetRoleInfo(ctx, appId)
if err != nil {
    log.Fatal(err)
}
for _, role := range roles.Roles {
    fmt.Printf("Role %d: %s (%s)\n", role.ID, role.Name, role.Access.Description)
}

// Get all users and their role assignments
users, err := xmlClient.UserRoles(ctx, appId)
for _, user := range users.Users {
    fmt.Printf("%s: %v\n", user.Name, user.Roles)
}

// Get comprehensive schema (fields, reports, variables)
schema, err := xmlClient.GetSchema(ctx, tableId)
for _, field := range schema.Table.Fields {
    fmt.Printf("Field %d: %s (%s)\n", field.ID, field.Label, field.FieldType)
}
```

### Available Methods

**App Discovery:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GrantedDBs(ctx, opts)` | API_GrantedDBs | List all apps/tables user can access |
| `FindDBByName(ctx, name, parentsOnly)` | API_FindDBByName | Find an app by name |
| `GetDBInfo(ctx, dbid)` | API_GetDBInfo | Get app/table metadata (record count, manager, timestamps) |
| `GetNumRecords(ctx, tableId)` | API_GetNumRecords | Get total record count for a table |

**Role Management:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetRoleInfo(ctx, appId)` | API_GetRoleInfo | Get all roles defined in an application |
| `UserRoles(ctx, appId)` | API_UserRoles | Get all users and their role assignments |
| `GetUserRole(ctx, appId, userId, includeGroups)` | API_GetUserRole | Get roles for a specific user |
| `AddUserToRole(ctx, appId, userId, roleId)` | API_AddUserToRole | Assign a user to a role |
| `RemoveUserFromRole(ctx, appId, userId, roleId)` | API_RemoveUserFromRole | Remove a user from a role |
| `ChangeUserRole(ctx, appId, userId, currentRole, newRole)` | API_ChangeUserRole | Change a user's role |

**User Information:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetUserInfo(ctx, email)` | API_GetUserInfo | Get user info by email address |

**Application Variables:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetDBVar(ctx, appId, varName)` | API_GetDBVar | Get an application variable value |
| `SetDBVar(ctx, appId, varName, value)` | API_SetDBVar | Set an application variable value |

**Schema Information:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `GetSchema(ctx, dbid)` | API_GetSchema | Get comprehensive app/table metadata |

**Record Information:**

| Method | XML Action | Description |
|--------|------------|-------------|
| `DoQueryCount(ctx, tableId, query)` | API_DoQueryCount | Get count of matching records (no data fetch) |
| `GetRecordInfo(ctx, tableId, recordId)` | API_GetRecordInfo | Get record with field metadata |
| `GetRecordInfoByKey(ctx, tableId, keyValue)` | API_GetRecordInfo | Get record by key field value |

### Error Handling

XML API errors are returned as `*xml.Error` with error codes:

```go
roles, err := xmlClient.GetRoleInfo(ctx, appId)
if err != nil {
    var xmlErr *xml.Error
    if errors.As(err, &xmlErr) {
        fmt.Printf("XML API error %d: %s\n", xmlErr.Code, xmlErr.Text)
    }

    // Helper functions for common error types
    if xml.IsUnauthorized(err) {
        fmt.Println("Not authorized")
    }
    if xml.IsNotFound(err) {
        fmt.Println("Resource not found")
    }
}
```

### Why Use XML API?

Each API has unique capabilities:

| XML-Only | JSON-Only |
|----------|-----------|
| Roles & role assignments | Relationships |
| Group creation/deletion | Solutions (app packaging) |
| Application variables (DBVars) | Platform analytics |
| Code pages | Audit logs |
| Webhooks management | Field usage statistics |
| User provisioning | Document templates |

For a comprehensive comparison, see [docs/xml-api-reference.md](docs/xml-api-reference.md).

## Development

```bash
# Clone with submodules (includes OpenAPI spec)
git clone --recurse-submodules https://github.com/DrewBradfordXYZ/quickbase-go.git

# Or initialize submodules after clone
git submodule update --init

# Run tests
go test ./...

# Run integration tests (requires .env with credentials)
cp .env.example .env
# Edit .env with your QB_REALM and QB_USER_TOKEN
go test ./tests/integration/... -v
```

### Updating the OpenAPI Spec

The `spec/` directory is a Git submodule pointing to [quickbase-spec](https://github.com/DrewBradfordXYZ/quickbase-spec). Each SDK pins to a specific commit, so spec updates are controlled:

```bash
# Update to latest spec
cd spec
git pull origin main
cd ..
git add spec
git commit -m "Update quickbase-spec submodule"

# Regenerate wrapper methods
go run ./cmd/generate-wrappers/main.go
```

This reads `spec/output/quickbase-patched.json` and generates `client/api_generated.go`.

## Related Projects

- [quickbase-js](https://github.com/DrewBradfordXYZ/quickbase-js) - TypeScript/JavaScript SDK

## License

MIT
