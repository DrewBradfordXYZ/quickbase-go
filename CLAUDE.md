# CLAUDE.md

This file provides guidance for Claude Code when working with this repository.

## Project Overview

This is the QuickBase Go SDK - a Go client for the QuickBase JSON RESTful API. It mirrors the feature set of [quickbase-js](https://github.com/DrewBradfordXYZ/quickbase-js).

## Architecture

The design and its reasons are in `docs/architecture.md`: one method per operation, request struct in, plain result out. Read it before adding API surface; don't add builders, wrappers or a second way to call an operation.

```
quickbase-go/
├── quickbase.go             # New(), options (wrapping client's), helpers (Row, Value, Where, ...)
├── types_generated.go       # Every generated type re-exported under its name (generated)
├── client/
│   ├── client.go            # HTTP layer: auth, retry, throttle, read-only, schema aliases
│   ├── methods_generated.go # One method per operation (generated)
│   ├── results.go           # What generated methods return: result, raw bytes, or QuickBase's error
│   ├── api.go               # Paging: RunQueryAll/N/Iterator, RunReportAll/N/Iterator
│   ├── auth_options.go      # WithUserToken, WithTempTokens, ... (WithAuth for any strategy)
│   ├── pagination.go        # Paginate, CollectAll, CollectN
│   └── throttle.go          # SlidingWindowThrottle, NoOpThrottle
├── auth/                    # Strategies: user token, temp token, SSO, ticket
├── core/                    # Errors, schema aliases, Date/DateTime, logger
├── xml/                     # The legacy XML API
├── generated/
│   └── quickbase.gen.go     # oapi-codegen output (DO NOT EDIT)
├── cmd/
│   ├── gospec/              # Adapts the shared spec for Go before oapi-codegen
│   ├── generate-methods/    # Writes methods_generated.go and types_generated.go
│   └── schema/              # Schema generator for users
├── spec/                    # Git submodule: the shared OpenAPI spec
└── tests/integration/       # Integration tests (QB_REALM, QB_USER_TOKEN)
```

## Key Patterns

### Generation (`make generate`)
1. `cmd/gospec` adapts `spec/output/quickbase-patched.json` (shared, language-neutral) into `.build/openapi-go.json`: inline results become named `<Operation>Result` types; optional fields of response-only types become plain values; a record value is `any`. Go-only concerns live here, never in the shared spec.
2. oapi-codegen v2.8.2 generates `generated/quickbase.gen.go` (runtime v1.8.0 keeps users on Go 1.24).
3. `cmd/generate-methods` writes a `Client` method per operation and the root package's type aliases.

### Request Flow
1. User calls `client.GetApp(ctx, appID)` (generated), which calls the generated client
2. `authHTTPClient.Do()` checks read-only mode, resolves schema aliases (bodies and a `tableId` parameter), then retries with throttle and auth (`do()`)
3. The method returns the success type, `[]byte` for a response that isn't JSON, or a typed error (`parseAPIError`); a 207 returns the result

### Authentication Flow
1. Auth options (`WithUserToken`, ...) are `client.Option`s that set a strategy, built from the realm when the client is created
2. `client.Client` uses the strategy to add auth headers (or XML body elements)
3. On 401, the strategy can refresh tokens (temp/SSO tokens)

## Common Commands

The `spec/` submodule is the shared OpenAPI spec. `git pull` doesn't move it unless `submodule.recurse` is set (`git config submodule.recurse true`); after pulling, run `git submodule update --init spec`. Generating from a checkout behind the commit this repo records writes a client for an old API, so the generate commands stop first (`scripts/check-spec.sh`). A checkout ahead of it (working on a newer spec) is allowed. `git status` showing ` M spec` means the checkout isn't on the recorded commit.

```bash
# Run all tests
go test ./...

# Run unit tests only
go test ./core/... ./client/...

# Run integration tests (requires .env)
go test ./tests/integration/... -v

# Build
go build ./...

# Tidy dependencies
go mod tidy

# Regenerate from OpenAPI spec (if spec changes)
go generate ./...
```

## Testing

### Unit Tests
Located in `*_test.go` files alongside source code. Use mocked data.

### Integration Tests
Located in `tests/integration/`. Require real QuickBase credentials:
1. Copy `.env.example` to `.env`
2. Set `QB_REALM` and `QB_USER_TOKEN`
3. Tests create ephemeral apps that are auto-cleaned up

## Working with Generated Types

Results have plain fields (no pointers, no nil checks). Request types keep pointers for optional fields, so a `false` or `0` reaches QuickBase; use `quickbase.Ptr`. A record is field ID to `FieldValue{Value: any}`:

```go
data := []quickbase.Record{
	{"6": {Value: "text"}, "7": {Value: 42}},
	quickbase.Row(6, "text", 7, 42),
}
```

## Dependencies

- `github.com/oapi-codegen/runtime` - OpenAPI runtime for generated client
- `github.com/joho/godotenv` - Load .env files for integration tests

## Application Tokens (XML API Only)

Application tokens are an additional security layer for QuickBase apps. When an app has
"Require Application Tokens" enabled, API calls must include a valid app token.

### Key Points

| Auth Method | App Token Required? | Why |
|-------------|---------------------|-----|
| User Token | **No** | User tokens bypass app token checks entirely |
| Temp Token | Only for XML API | JSON API doesn't use app tokens |
| Ticket Auth | Only for XML API | JSON API doesn't use app tokens |
| SSO Token | Only for XML API | JSON API doesn't use app tokens |

### JSON API vs XML API

- **JSON API**: Does not use app tokens. User tokens and temp tokens provide sufficient authentication.
- **XML API**: Uses `<apptoken>` element in request bodies when the app requires them.

The SDK only injects app tokens into XML API requests (via the `xml` sub-package).

### Usage

```go
// For apps requiring application tokens when using ticket auth
client, _ := quickbase.New("myrealm",
    quickbase.WithTicketAuth("user@example.com", "password"),
    quickbase.WithAppToken("your-app-token"),
)

// XML API calls will include the app token
xmlClient := xml.New(client)
roles, _ := xmlClient.GetRoleInfo(ctx, appId)
```

### When You DON'T Need App Tokens

If you're using `WithUserToken()`, you never need to specify an app token - QuickBase
bypasses app token checks for user token authentication.

## Temporary Token Authentication

**This is a key architectural difference between the Go and JS SDKs.**

### The Core Insight
Temp tokens prove a user is logged into QuickBase via their browser session. The benefit is verification - you know the request came from someone actually logged into QuickBase.

### JS SDK (Browser/Code Pages)
The JS SDK can **fetch** temp tokens because it runs in the browser:
```javascript
// In temp-token.ts
const response = await this.fetchApi(url, {
  method: 'GET',
  headers,
  credentials: 'include', // <-- Sends browser cookies!
});
```
- Uses `credentials: 'include'` on the `/auth/temporary/{dbid}` request
- Browser automatically sends QuickBase session cookies
- Works in Code Pages where user is logged into QuickBase
- No user token needed - the browser session IS the auth

### Go SDK (Server-Side)
The Go SDK **receives** pre-existing temp tokens - it cannot fetch them:
- No browser cookies available on server
- Primary use case: receiving tokens from browser clients (e.g., Code Pages)
- Browser fetches tokens using user's QuickBase session, sends via HTTP headers

**Available options:**
- `quickbase.WithTempTokens(map[string]string{"dbid": token})` - Create client with tokens
- `auth.WithInitialTempToken(token)` - Create client with a single token
- `auth.WithInitialTempTokenForTable(token, dbid)` - When you know the table ID
- `strategy.SetToken(dbid, token)` - Add tokens during request lifecycle

### Why use temp tokens over user tokens?
- **Verification**: Proves user is logged into QuickBase right now
- **No credentials storage**: Your server doesn't need to store user tokens
- **Table-scoped**: More restrictive permissions
- **Short-lived**: ~5 minutes, limits damage if leaked

### Why NOT fetch temp tokens with a user token?
You technically CAN call `/auth/temporary/{dbid}` with a user token, but there's no point:
- If you have a user token, just use that directly
- The benefit of temp tokens is proving browser session - if you're using a user token, you've already bypassed that

### Browser-to-Server Token Flow
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  QuickBase  │     │   Browser   │     │  Go Server  │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                   │
       │  User opens       │                   │
       │  Code Page        │                   │
       │◄──────────────────│                   │
       │                   │                   │
       │  Browser fetches  │                   │
       │  temp token       │                   │
       │◄─────────────────►│                   │
       │                   │                   │
       │                   │  Request + token  │
       │                   │  (via headers)    │
       │                   │──────────────────►│
       │                   │                   │
       │                   │     API calls     │
       │◄──────────────────┼───────────────────│
       │                   │                   │
```

1. User opens Code Page in QuickBase (logged in)
2. Browser JavaScript fetches temp token using session cookies
3. Browser sends request to Go server with token in header (e.g., X-QB-Token-{dbid})
4. Go server extracts token from header and creates client
5. Go server makes API calls back to QuickBase using that token

### Automatic DBID Extraction

Both the JS and Go SDKs automatically extract the dbid from API requests:
- Query params: `tableId`, `appId`
- Path: `/tables/{tableId}`, `/apps/{appId}`
- Request body: `from` (runQuery, deleteRecords), `to` (upsert)

This means the SDK knows which table you're accessing without explicit configuration.

See `extractDBID()` and `extractDBIDFromBody()` in `client/client.go`.

### Token Negotiation Pattern (Future Enhancement)

Currently, the browser must know which tables it needs tokens for upfront. A better pattern:

**Problem:** Browser doesn't know which tables the Go server will access.

**Solution:** 401 negotiation - server tells browser what tokens it needs.

```
1. Browser sends request to Go server (no tokens)
2. Go server creates client, makes API call
3. SDK returns MissingTokensError with required dbids
4. Go server returns 401 with X-QB-Tables-Needed header
5. Browser fetches those tokens, retries request
```

**Handler pattern (not yet implemented):**

```go
func tempTokenHandler(w http.ResponseWriter, r *http.Request) {
    // Extract whatever tokens the browser sent (header format is user-defined)
    tokens := map[string]string{
        r.Header.Get("X-QB-Table-ID"): r.Header.Get("X-QB-Temp-Token"),
    }

    client, _ := quickbase.New("myrealm", quickbase.WithTempTokens(tokens))

    // Make API call - SDK extracts dbid from request body/path automatically
    _, err := client.RunQuery(ctx, quickbase.RunQueryBody{From: "bqxyz123"})

    // SDK returns structured error with missing dbids
    var missing *quickbase.MissingTokensError
    if errors.As(err, &missing) {
        w.Header().Set("X-QB-Tables-Needed", strings.Join(missing.DBIDs, ","))
        w.WriteHeader(401)
        return
    }

    // Success - return data
}
```

**Key decisions:**
- Header format (how browser sends tokens) is user-defined, not SDK-defined
- SDK provides `MissingTokensError` type for handlers to catch
- One client per request for temp tokens (tokens are user-scoped, short-lived)

**Related:** See `codepage-hypermedia` repo for browser-side handling. The Datastar adapter uses bracket syntax `[bqxyz123]` to pre-fetch tokens, but could fall back to 401 negotiation.

### Auth-Agnostic Handlers

Handlers shouldn't know or care about auth method. The same handler should work with user tokens, temp tokens, tickets, or SSO.

**Solution:** Middleware injects the client into the request context. Handlers just retrieve it.

```go
// Handler is auth-agnostic - doesn't know how client was created
func getRecords(w http.ResponseWriter, r *http.Request) {
    client := getClient(r)  // from context
    records, _ := client.RunQueryAll(r.Context(), body)
    json.NewEncoder(w).Encode(records)
}
```

The middleware varies by auth type:
- **User token / Ticket / SSO**: Returns shared client (created at startup)
- **Temp token**: Creates client per-request with tokens from headers

This keeps handlers simple and testable. The auth complexity lives in middleware.

See `docs/middleware-and-di.md` for a detailed explanation of these patterns.

## Related Repositories

**[quickbase-js](https://github.com/DrewBradfordXYZ/quickbase-js)** - TypeScript/JavaScript SDK
- Same feature set (auth methods, throttling, retry, error types)
- Same test patterns (unit + integration with ephemeral apps)
- Shared OpenAPI spec (git submodule)

**[codepage-go](https://github.com/DrewBradfordXYZ/codepage-go)** - HTTP middleware for this SDK
- Auth-agnostic handlers (same handler works with any auth method)
- Middleware injects QuickBase client into request context
- Token negotiation for temp tokens (401 with X-QB-Tables-Needed header)
- Works with datastar-go for SSE responses

**[codepage-hypermedia](https://github.com/DrewBradfordXYZ/codepage-hypermedia)** - Browser-side companion
- JavaScript client for Code Pages
- Fetches temp tokens using browser session
- Adapters for Datastar, HTMX, Lit
