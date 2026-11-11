# Migration Guide: v1.x to v2.0

## Overview

v2.0 introduces **wrapper types around the generated responses**: every field has a nil-safe accessor method, and `Raw()` returns the full generated response.

## Key Changes

### 1. Wrapper Types

Builders now return wrapper types with accessor methods:

```go
app, err := client.GetApp(appId).Run(ctx)

// Every field has a nil-safe accessor method
name := app.Name()                // string
description := app.Description()  // returns "" if nil
created := app.Created()          // returns "" if nil

// Direct pointer access through the raw response
if raw := app.Raw(); raw != nil && raw.JSON200 != nil && raw.JSON200.Description != nil {
    desc := *raw.JSON200.Description
}
```

**Why this approach?**
- **No data loss** - `Raw()` on a result, or the embedded generated type on a list item, has every field
- **Convenience** - Nil-safe methods for optional fields
- **Fully generated** - No manual wrapper maintenance

### 2. Array Responses Return Wrapped Items

```go
fields, err := client.GetFields("tableId").Run(ctx)
// fields is []*FieldsItem (wrapped)

for _, field := range fields {
    // Convenience methods
    label := field.Label()       // nil-safe accessor
    fieldType := field.FieldType()

    // The generated item is embedded
    id := field.GetFieldsItem.Id
}
```

### 3. RunQuery with Records() Method

```go
result, err := client.RunQuery(ctx, body)

// Convenience method for unwrapped records
for _, rec := range result.Records() {
    fmt.Println(rec["6"])  // Field ID as key
}

// Pagination metadata
fmt.Printf("Total: %d\n", result.Metadata().TotalRecords())
fmt.Printf("Returned: %d\n", result.Metadata().NumRecords())
```

### 4. Generated Package Exported

The generated types are available at `github.com/DrewBradfordXYZ/quickbase-go/v2/generated`:

```go
import "github.com/DrewBradfordXYZ/quickbase-go/v2/generated"

// The full generated response, when you need it
var raw *generated.RunQueryResponse = result.Raw()
```

## Wrapper Type Pattern

All wrapper types follow this pattern:

```go
// Holds the generated response
type AppResult struct {
    resp *generated.GetAppResponse
}

// Nil-safe accessors for every field
func (r *AppResult) Description() string {
    if r == nil || r.resp == nil || r.resp.JSON200 == nil || r.resp.JSON200.Description == nil {
        return ""
    }
    return *r.resp.JSON200.Description
}

// The full response (status, headers, body) for anything else
func (r *AppResult) Raw() *generated.GetAppResponse
```

## Migration Steps

1. Update builder `.Run()` calls - return types are now wrapper types
2. Use accessor methods for optional fields (e.g., `app.Description()` instead of `quickbase.Deref(app.Description)`)
3. Use `result.Records()` instead of `quickbase.UnwrapRecords(*result.Data)`
4. For anything the accessors don't cover, use `Raw()` on results and the embedded generated type on list items

## Examples

### Before and After

```go
// v1.x - Manual dereferencing
app, _ := client.GetApp(appId).Run(ctx)
description := quickbase.Deref(app.Description)

// v2.0 - Nil-safe accessor method
app, _ := client.GetApp(appId).Run(ctx)
description := app.Description()

// v1.x - Manual unwrap
result, _ := client.RunQuery(ctx, body)
if result.Data != nil {
    records := quickbase.UnwrapRecords(*result.Data)
}

// v2.0 - Convenience method
result, _ := client.RunQuery(ctx, body)
records := result.Records()  // nil-safe, returns empty slice if no data

// v1.x - Array response
fields, _ := client.GetFields("tableId").Run(ctx)
for _, f := range fields {
    label := quickbase.Deref(f.Label)
}

// v2.0 - Wrapped array items
fields, _ := client.GetFields("tableId").Run(ctx)
for _, f := range fields {
    label := f.Label()  // nil-safe accessor
}
```

## Helper Functions

The following helpers are still available but less commonly needed:

### Deref / DerefOr

```go
// Still useful for nested or complex types
app, _ := client.GetApp(appId).Run(ctx)
description := quickbase.DerefOr(app.Raw().JSON200.Description, "No description")
```

### UnwrapRecords

```go
// Still available for RunQueryAll which returns raw records
rawRecords, _ := client.RunQueryAll(ctx, body)
records := quickbase.UnwrapRecords(rawRecords)
```
