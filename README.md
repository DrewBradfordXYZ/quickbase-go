# QuickBase Go SDK

A Go client for the QuickBase JSON RESTful API.

[![Go Reference](https://pkg.go.dev/badge/github.com/DrewBradfordXYZ/quickbase-go.svg)](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go)

## Features

- **Typed API Methods** - Full type safety with auto-generated types from OpenAPI spec
- **Multiple Auth Methods** - User token, temporary token, and SSO authentication
- **Automatic Retry** - Exponential backoff with jitter for rate limits and server errors
- **Proactive Throttling** - Optional client-side request throttling (100 req/10s)
- **Custom Error Types** - Specific error types for 400, 401, 403, 404, 429, 5xx responses
- **Debug Logging** - Optional request/response timing logs

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
    resp, err := client.API().GetAppWithResponse(ctx, "your-app-id")
    if err != nil {
        log.Fatal(err)
    }
    if resp.JSON200 != nil {
        fmt.Println("App name:", resp.JSON200.Name)
    }
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

### Temporary Token

```go
client, err := quickbase.New("mycompany",
    quickbase.WithTempTokenAuth(
        auth.WithTempTokenUserToken("your-user-token"),
    ),
)
```

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

All QuickBase API endpoints are available through the generated client:

```go
ctx := context.Background()

// Apps
app, _ := client.API().GetAppWithResponse(ctx, appId)
tables, _ := client.API().GetAppTablesWithResponse(ctx, &generated.GetAppTablesParams{
    AppId: appId,
})

// Tables
table, _ := client.API().GetTableWithResponse(ctx, tableId, &generated.GetTableParams{
    AppId: appId,
})

// Fields
fields, _ := client.API().GetFieldsWithResponse(ctx, &generated.GetFieldsParams{
    TableId: tableId,
})

// Query records
resp, _ := client.API().RunQueryWithResponse(ctx, generated.RunQueryJSONRequestBody{
    From:   tableId,
    Select: &[]int{3, 6, 7},
    Where:  ptr("{6.GT.100}"),
})
for _, record := range *resp.JSON200.Data {
    fmt.Println(record)
}

// Insert/Update records
data := []generated.QuickbaseRecord{
    {
        "6": generated.FieldValue{Value: toFieldValue("New Record")},
        "7": generated.FieldValue{Value: toFieldValue(42)},
    },
}
result, _ := client.API().UpsertWithResponse(ctx, generated.UpsertJSONRequestBody{
    To:   tableId,
    Data: &data,
})

// Delete records
client.API().DeleteRecordsWithResponse(ctx, generated.DeleteRecordsJSONRequestBody{
    From:  tableId,
    Where: "{3.EX.123}",
})
```

## Error Handling

The SDK provides specific error types for different HTTP status codes:

```go
import "github.com/DrewBradfordXYZ/quickbase-go/core"

resp, err := client.API().GetAppWithResponse(ctx, "invalid-id")
if err != nil {
    var rateLimitErr *core.RateLimitError
    var notFoundErr *core.NotFoundError
    var validationErr *core.ValidationError

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

QuickBase enforces a rate limit of **100 requests per 10 seconds** per user token.

### Reactive (Default)

The SDK automatically retries on 429 responses with exponential backoff:

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithMaxRetries(5),
)
```

### Proactive Throttling

Prevent 429 errors by throttling requests client-side:

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithProactiveThrottle(100), // 100 req/10s
)
```

### Rate Limit Callback

Get notified when rate limited:

```go
client, _ := quickbase.New("realm",
    quickbase.WithUserToken("token"),
    quickbase.WithOnRateLimit(func(info quickbase.RateLimitInfo) {
        log.Printf("Rate limited on %s", info.RequestURL)
        log.Printf("Retry after: %d seconds", info.RetryAfter)
        log.Printf("Ray ID: %s", info.QBAPIRay)
    }),
)
```

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

## Related Projects

- [quickbase-js](https://github.com/DrewBradfordXYZ/quickbase-js) - TypeScript/JavaScript SDK

## License

MIT
