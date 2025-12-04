# Middleware and Dependency Injection in Go

A practical guide using the QuickBase SDK as an example.

## The Problem We're Solving

You have HTTP handlers that need a QuickBase client. But *how* that client is created depends on the auth method:

| Auth Method | Client Lifecycle |
|-------------|------------------|
| User Token | Created once at startup, shared |
| Ticket | Created once at startup, shared |
| Temp Token | Created per-request (user-specific tokens) |

**Goal:** Write handlers that don't care about auth. They just use a client.

---

## Part 1: The Naive Approach (What We Want to Avoid)

```go
// BAD: Handler knows about auth details
func getRecords(w http.ResponseWriter, r *http.Request) {
    // Handler has to know: "am I using temp tokens or user tokens?"
    var client *quickbase.Client

    if usingTempTokens {
        token := r.Header.Get("X-QB-Temp-Token")
        client, _ = quickbase.New("realm", quickbase.WithTempTokens(...))
    } else {
        client = sharedClient // from somewhere
    }

    records, _ := client.RunQueryAll(ctx, body)
    // ...
}
```

Problems:
- Every handler repeats this logic
- Handlers are coupled to auth implementation
- Hard to test (can't easily swap in a mock client)

---

## Part 2: Functions as Values

Before understanding DI, you need to understand that **functions are values** in Go.

```go
// A function
func add(a, b int) int {
    return a + b
}

// A variable that holds a function
var operation func(int, int) int

// Assign the function to the variable
operation = add

// Call it
result := operation(2, 3) // 5
```

You can also pass functions as arguments:

```go
func doMath(a, b int, op func(int, int) int) int {
    return op(a, b)
}

result := doMath(2, 3, add) // 5
```

---

## Part 3: Dependency Injection (DI)

**Dependency Injection** just means: *give a thing the stuff it needs, instead of having it create/find the stuff itself.*

### Without DI (hard to test, inflexible)

```go
func getRecords(w http.ResponseWriter, r *http.Request) {
    // Handler CREATES its own client - it's "dependent" on knowing how
    client, _ := quickbase.New("realm", quickbase.WithUserToken("xxx"))
    records, _ := client.RunQueryAll(ctx, body)
}
```

### With DI (flexible, testable)

```go
// Handler RECEIVES a client - the dependency is "injected"
func getRecords(w http.ResponseWriter, r *http.Request, client *quickbase.Client) {
    records, _ := client.RunQueryAll(ctx, body)
}
```

But wait - `http.HandlerFunc` only takes `(w, r)`. How do we pass the client?

### Solution: Closure

A **closure** is a function that "captures" variables from its surrounding scope.

```go
func makeHandler(client *quickbase.Client) http.HandlerFunc {
    // This returns a function that "closes over" the client variable
    return func(w http.ResponseWriter, r *http.Request) {
        // client is available here even though it's not a parameter!
        records, _ := client.RunQueryAll(r.Context(), body)
        json.NewEncoder(w).Encode(records)
    }
}

// Usage
client, _ := quickbase.New("realm", quickbase.WithUserToken("xxx"))
http.HandleFunc("/records", makeHandler(client))
```

### Solution: Struct with Methods

```go
type Server struct {
    client *quickbase.Client
}

func (s *Server) getRecords(w http.ResponseWriter, r *http.Request) {
    // Access client via s.client
    records, _ := s.client.RunQueryAll(r.Context(), body)
    json.NewEncoder(w).Encode(records)
}

// Usage
server := &Server{
    client: client,
}
http.HandleFunc("/records", server.getRecords)
```

---

## Part 4: Middleware

**Middleware** is a function that wraps a handler to add behavior before/after it runs.

### The Shape of Middleware

```go
func middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // BEFORE: do something before the handler

        next.ServeHTTP(w, r)  // call the actual handler

        // AFTER: do something after the handler
    })
}
```

### Example: Logging Middleware

```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        next.ServeHTTP(w, r)  // call the handler

        log.Printf("%s %s took %v", r.Method, r.URL.Path, time.Since(start))
    })
}

// Usage
handler := http.HandlerFunc(getRecords)
wrappedHandler := loggingMiddleware(handler)
http.Handle("/records", wrappedHandler)
```

### Chaining Middleware

Middleware can be chained - each one wraps the next:

```go
handler := http.HandlerFunc(getRecords)
handler = loggingMiddleware(handler)
handler = authMiddleware(handler)
handler = corsMiddleware(handler)

// Request flow: CORS → Auth → Logging → getRecords → Logging → Auth → CORS
```

---

## Part 5: Using Context to Pass Values

HTTP handlers only get `(w, r)`. To pass extra data (like a client), use `context.Context`.

### Storing a Value

```go
ctx := context.WithValue(r.Context(), "mykey", myValue)
r = r.WithContext(ctx)  // create new request with updated context
```

### Retrieving a Value

```go
value := r.Context().Value("mykey")
```

### Type-Safe Keys

Using strings as keys can cause collisions. Better to use a custom type:

```go
// Define a key type (unexported, so only your package can use it)
type contextKey string

const clientKey contextKey = "quickbase-client"

// Store
ctx := context.WithValue(r.Context(), clientKey, client)

// Retrieve (with type assertion)
client := r.Context().Value(clientKey).(*quickbase.Client)
```

---

## Part 6: Putting It All Together

### Auth-Agnostic Handlers with Middleware

```go
// Key for storing client in context
type contextKey string
const qbClientKey contextKey = "qb-client"

// Helper to get client from context
func getClient(r *http.Request) *quickbase.Client {
    return r.Context().Value(qbClientKey).(*quickbase.Client)
}

// Middleware that injects the client
func withQuickBase(getClientFn func(*http.Request) *quickbase.Client) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            client := getClientFn(r)
            ctx := context.WithValue(r.Context(), qbClientKey, client)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Handler - doesn't know or care about auth!
func getRecords(w http.ResponseWriter, r *http.Request) {
    client := getClient(r)
    records, err := client.RunQueryAll(r.Context(), body)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    json.NewEncoder(w).Encode(records)
}
```

### For User Token (Shared Client)

```go
func main() {
    // Create client once at startup
    sharedClient, _ := quickbase.New("realm", quickbase.WithUserToken("xxx"))

    // Middleware always returns the same client
    qbMiddleware := withQuickBase(func(r *http.Request) *quickbase.Client {
        return sharedClient
    })

    // Apply to handlers
    http.Handle("/records", qbMiddleware(http.HandlerFunc(getRecords)))
    http.ListenAndServe(":8080", nil)
}
```

### For Temp Token (Per-Request Client)

```go
func main() {
    // Middleware creates a new client per request
    qbMiddleware := withQuickBase(func(r *http.Request) *quickbase.Client {
        token := r.Header.Get("X-QB-Temp-Token")
        tableID := r.Header.Get("X-QB-Table-ID")

        client, _ := quickbase.New("realm",
            quickbase.WithTempTokens(map[string]string{tableID: token}),
        )
        return client
    })

    // Same handlers work!
    http.Handle("/records", qbMiddleware(http.HandlerFunc(getRecords)))
    http.ListenAndServe(":8080", nil)
}
```

### The Handler Never Changed

Notice that `getRecords` is identical in both cases. It just calls `getClient(r)` and uses it. The middleware handles the auth complexity.

---

## Part 7: Adding 401 Negotiation for Temp Tokens

The temp token middleware can handle the "missing token" case:

```go
func tempTokenMiddleware(realm string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract tokens from request
            tokens := extractTokens(r)

            client, _ := quickbase.New(realm, quickbase.WithTempTokens(tokens))
            ctx := context.WithValue(r.Context(), qbClientKey, client)

            // Wrap response writer to catch MissingTokensError
            rw := &tokenNegotiationWriter{
                ResponseWriter: w,
                request:        r,
            }

            next.ServeHTTP(rw, r.WithContext(ctx))
        })
    }
}

// When handler returns MissingTokensError, convert to 401 with header
type tokenNegotiationWriter struct {
    http.ResponseWriter
    request *http.Request
}

// This is conceptual - actual implementation would need error propagation
```

The key insight: **handlers stay simple, middleware handles complexity**.

---

## Summary

| Concept | What It Is | Why It Helps |
|---------|-----------|--------------|
| **Dependency Injection** | Pass dependencies in, don't create them inside | Testable, flexible, swappable |
| **Closure** | Function that captures outer variables | Pass extra data to handlers |
| **Middleware** | Function that wraps a handler | Add behavior without changing handlers |
| **Context** | Request-scoped key-value store | Pass data through the request chain |

The pattern:
1. Middleware creates/fetches the client based on auth method
2. Middleware stores client in context
3. Handler retrieves client from context
4. Handler doesn't know how client was created

This makes handlers **auth-agnostic** - the same handler works with user tokens, temp tokens, tickets, or SSO.
