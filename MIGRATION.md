# Migration Guide: v1.x to v2.0

## Overview

v2.0 introduces **wrapper types that embed generated types**, providing both full API access and convenience methods. All fields from the OpenAPI spec are accessible directly, with nil-safe accessor methods for optional fields.

## Key Changes

### 1. Wrapper Types with Embedding

Builders now return wrapper types that embed the generated types:

```go
app, err := client.GetApp(appId).Run(ctx)

// Required fields accessed directly (via embedding)
name := app.Name  // string

// Optional fields have nil-safe accessor methods
description := app.Description()  // returns "" if nil
created := app.Created()          // returns "" if nil

// Direct pointer access still available
if app.GetAppData.Description != nil {
    desc := *app.GetAppData.Description
}
```

**Why this approach?**
- **No data loss** - All generated fields are accessible via embedding
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

    // Direct access via embedding
    id := field.Id  // required field - direct access
}
```

### 3. RunQuery with Records() Method

```go
result, err := client.RunQuery(ctx, body)

// Convenience method for unwrapped records
for _, rec := range result.Records() {
    fmt.Println(rec["6"])  // Field ID as key
}

// Direct access to metadata (via embedding)
fmt.Printf("Total: %d\n", result.Metadata.TotalRecords)
fmt.Printf("Returned: %d\n", result.Metadata.NumRecords)
```

### 4. Generated Package Exported

The generated types are available at `github.com/DrewBradfordXYZ/quickbase-go/generated`:

```go
import "github.com/DrewBradfordXYZ/quickbase-go/generated"

// Access the embedded type directly when needed
var rawData *generated.RunQueryData = result.RunQueryData
```

## Wrapper Type Pattern

All wrapper types follow this pattern:

```go
// Embeds the generated type - all fields accessible
type AppResult struct {
    *generated.GetAppData
}

// Nil-safe accessors for pointer fields
func (r *AppResult) Description() string {
    if r == nil || r.GetAppData == nil || r.GetAppData.Description == nil {
        return ""
    }
    return *r.GetAppData.Description
}
```

## Migration Steps

1. Update builder `.Run()` calls - return types are now wrapper types
2. Use accessor methods for optional fields (e.g., `app.Description()` instead of `quickbase.Deref(app.Description)`)
3. Use `result.Records()` instead of `quickbase.UnwrapRecords(*result.Data)`
4. Direct field access via embedding still works for all fields

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
description := quickbase.DerefOr(app.GetAppData.Description, "No description")
```

### UnwrapRecords

```go
// Still available for RunQueryAll which returns raw records
rawRecords, _ := client.RunQueryAll(ctx, body)
records := quickbase.UnwrapRecords(rawRecords)
```
