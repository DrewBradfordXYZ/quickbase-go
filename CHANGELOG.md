# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Response properties the spec omitted**, found by comparing live responses with the spec (`spec/tools/audit-live.ts`):
  - Field properties: `SummaryQuery` (a summary field's filter), `PrimaryKey`, `MasterTableId`, `MasterTableKeyFid`, `FieldCaching`, `OpenTargetIn`. A reference field's parent table is `MasterTableId`; `MasterChoiceTableId` is not returned for it.
  - Field usage: `CustomEmails`, `DocumentTemplates`.
  - Reports: `Incomplete`, and `Query.Fields`, `Query.SortBy`, `Query.GroupBy`.
- **Changes from QuickBase's current spec** (merged add-only from the API portal; see `spec/tools/fetch-upstream.ts`):
  - `GetUserApps` (`GET /users/{userId}/apps`).
  - `alias` query parameter on `CreateSolution` and `CreateSolutionFromRecord`.
  - Response properties on `CreateApp` (`AncestorId`), `UpdateSolution` and `GetSolutionPublic`.
  - `UpdateSolutionToRecord` keeps its name; QuickBase now calls it `updateSolutionFromRecord`.

### Changed

- **Generated from QuickBase's current spec.** The spec now builds from the OpenAPI 3 that QuickBase's API portal publishes (`spec/tools/fetch-upstream.ts`) instead of a December 2025 Swagger 2.0 copy. Where they differ, the current spec is followed:
  - **BREAKING**: fields QuickBase marks required became values instead of pointers: app variables' `Name` and `Value` (`CreateApp`, `UpdateApp`, `CopyApp`, `GetApp` variables), `TransferUserToken`'s `From`, `To` and `Id`, and upsert's `Data` (`Data: data`, not `Data: &data`).
  - **BREAKING**: audit payload changes (`Current`, `Previous`, `Fields`) are typed items instead of `[]interface{}`.
  - **BREAKING**: `RunReport` takes no request body (the API never read one): `RunReportBody`, `RunReportJSONRequestBody` and `RunReportWithBody` are gone; use the builder.
  - `GenerateDocumentParams.Accept` is a `*string` (the spec no longer lists its values).
- **BREAKING**: Report `Properties` (`GetTableReports()`, `GetReport()`) changed from `*map[string]interface{}` to a struct with typed fields for table, chart, summary and map report settings (`ColumnProperties`, `ChartType`, `Summarize`, ...). Keys not listed are kept in `AdditionalProperties`. Replace `(*props)["columnProperties"]` with `props.ColumnProperties`, and other keys with `props.AdditionalProperties["key"]`.
- `Query.Fields`, `Query.SortBy` and `Query.GroupBy` on reports are now typed fields, so they no longer appear in `Query.AdditionalProperties`.

### Fixed

- **Solutions and file downloads**: QBL is YAML and file content isn't JSON, but they were declared JSON, so `ExportSolution` and `DownloadFile` returned an error on every success and `CreateSolution`, `UpdateSolution` and `ChangesetSolution` sent JSON. Now:
  - `ExportSolution`, `DownloadFile` and `GenerateDocument` return the response with its raw content in `Body`.
  - `CreateSolution().Body(qbl)`, `UpdateSolution(id).Body(qbl)` and `ChangesetSolution(id).Body(qbl)` send the QBL as `application/x-yaml` (the client no longer overwrites a body's content type with JSON).
  - **BREAKING**: the JSON body types for those three (`CreateSolutionJSONRequestBody` and friends), `ExportSolutionResult`, `DownloadFileResult` and their `WithResponse` JSON variants are gone.
- **Generation is deterministic**: `generate-builders` ordered each builder's methods by map iteration, so every run rewrote `builders_generated.go`. `make generate` and `make tools` now use oapi-codegen v2.4.0, the version that produced the committed code.

## [2.3.0] - 2026-03-02

### Fixed

- **XML API Token Refresh**: The legacy XML API (`DoXML`) now correctly handles expired authentication tickets (errcode 8) by attempting a token refresh and retrying the request. Previously, expired XML tickets would fail immediately even if refresh logic was available.

### Changed

- **Internal Request Logic**: Centralized the core request execution cycle (retries, exponential backoff, jitter, proactive throttling, and error recovery) into a single reusable path for both JSON and XML APIs.
- **Refactored Unit Tests**: Updated client unit tests to call `calculateBackoff` directly on the `Client` struct, matching the new architecture.

## [2.2.0] - 2025-12-25

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
