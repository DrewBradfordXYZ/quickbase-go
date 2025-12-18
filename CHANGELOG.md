# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.9.0] - 2025-12-17

### Fixed

- **ownerId type mismatch**: QuickBase API returns `ownerId` as a string for personal reports, but the OpenAPI spec defined it as integer. This caused JSON unmarshal errors when fetching reports via `GetTableReports()` or `GetReport()`.

### Changed

- **BREAKING**: `ReportInfo.OwnerID` type changed from `int` to `interface{}`. This allows the field to accept both integer and string values from the API.

### Technical Details

The fix was applied at three levels:

1. **quickbase-spec** (submodule): Added `OwnerId` component schema that accepts any type, and patched report endpoints to use it via `$ref`.

2. **oapi-codegen**: The generated `OwnerId` type is now `interface{}` instead of `int`.

3. **Builder transforms**: Updated `ReportInfo` struct and transform mappings to use `interface{}` for the OwnerID field.

### Migration

If you were using `ReportInfo.OwnerID` as an integer:

```go
// Before (v1.8.x)
report, _ := client.GetReport(tableId, reportId).Run(ctx)
fmt.Printf("Owner: %d\n", report.OwnerID)

// After (v1.9.0)
report, _ := client.GetReport(tableId, reportId).Run(ctx)
if ownerID, ok := report.OwnerID.(float64); ok {
    fmt.Printf("Owner: %d\n", int(ownerID))
} else if ownerID, ok := report.OwnerID.(string); ok {
    fmt.Printf("Owner: %s\n", ownerID)
}
```

Note: JSON numbers unmarshal to `float64` in Go when the target is `interface{}`.

## [1.8.1] - 2025-12-XX

### Added

- Initial documented release with fluent builders

## [1.8.0] - 2025-12-XX

### Added

- Fluent builder pattern for all API operations
- Friendly result types with dereferenced fields
- Query builder with schema aliases
- Automatic pagination support
