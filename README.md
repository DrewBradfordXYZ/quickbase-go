# QuickBase Go SDK

A Go client for the QuickBase JSON RESTful API.

[![Go Reference](https://pkg.go.dev/badge/github.com/DrewBradfordXYZ/quickbase-go.svg)](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go)

## Features

- **Friendly API** - Clean wrapper methods like `RunQuery`, `RunQueryAll`, `Upsert`, `GetApp`
- **Automatic Pagination** - `RunQueryAll` fetches all records across pages automatically
- **Helper Functions** - `Ptr()`, `Ints()` for cleaner code with optional fields
- **Multiple Auth Methods** - User token, temporary token (via POST callback), and SSO
- **Automatic Retry** - Exponential backoff with jitter for rate limits and server errors
- **Proactive Throttling** - Optional client-side request throttling (100 req/10s)
- **Custom Error Types** - Specific error types for 400, 401, 403, 404, 429, 5xx responses
- **Full API Access** - Low-level generated client available via `client.API()`

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

    ctx := context.Background()

    // Get app details
    app, err := client.GetApp(ctx, "your-app-id")
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

### Temporary Token (POST Callback)

Temp tokens are short-lived (~5 min), table-scoped tokens that verify a user is logged into QuickBase. Unlike the JS SDK which can fetch temp tokens using browser cookies, **Go servers receive temp tokens from QuickBase** via POST callbacks.

**How it works:**
1. Configure a Formula-URL field in QuickBase with the "POST temp token" option
2. When a user clicks the link, QuickBase POSTs `{"tempToken": "..."}` to your server
3. Your server extracts the token and uses it to make API calls back to QuickBase

```go
import "github.com/DrewBradfordXYZ/quickbase-go/auth"

func handleQuickBaseCallback(w http.ResponseWriter, r *http.Request) {
    // Extract the temp token from the POST body
    token, err := auth.ExtractPostTempToken(r)
    if err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Create a client with the received token
    client, err := quickbase.New("myrealm",
        quickbase.WithTempTokenAuth(
            auth.WithInitialTempToken(token),
        ),
    )
    if err != nil {
        http.Error(w, "Failed to create client", http.StatusInternalServerError)
        return
    }

    // Use the client to make API calls back to QuickBase
    resp, err := client.API().GetAppWithResponse(r.Context(), appId)
    // ...
}
```

**Why use temp tokens?**
- Verifies the user is actually logged into QuickBase (via their browser session)
- Table-scoped (more restrictive than user tokens)
- No need to store user credentials on your server

See [QuickBase docs](https://help.quickbase.com/docs/post-temporary-token-from-a-quickbase-field) for configuring POST temp tokens.

### SSO Token

```go
client, err := quickbase.New("mycompany",
    quickbase.WithSSOTokenAuth("your-saml-token"),
)
```

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
```

## API Usage

The SDK provides friendly wrapper methods with cleaner types:

```go
ctx := context.Background()

// Get app details
app, err := client.GetApp(ctx, appId)
fmt.Println("App name:", app.Name)

// Get fields for a table
fields, err := client.GetFields(ctx, tableId)
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

// Insert/Update records
upsertResult, err := client.Upsert(ctx, quickbase.UpsertBody{
    To: tableId,
    Data: &[]quickbase.Record{
        {"6": quickbase.FieldValue{Value: fieldValue("New Record")}},
    },
})
fmt.Println("Created:", upsertResult.CreatedRecordIDs)

// Delete records
deleteResult, err := client.DeleteRecords(ctx, quickbase.DeleteRecordsBody{
    From:  tableId,
    Where: "{3.EX.123}",
})
fmt.Println("Deleted:", deleteResult.NumberDeleted)
```

### Helper Functions

```go
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
app, err := client.GetApp(ctx, "invalid-id")
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
app, err := client.GetApp(ctx, appId)
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
