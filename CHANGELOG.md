# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`RunQueryIterator` and `RunReportIterator`**: stream every record a page at a time (`for record, err := range ...`), so a large table never has to fit in memory. The package docs already pointed to `RunQueryIterator`; it didn't exist.
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

- **BREAKING** (152 by `apidiff`, all number types): numbers the spec left without a format were `float32`, which holds whole numbers exactly only up to 16,777,216, so a 9-digit account ID or a large group ID was changed before it was sent. IDs are now `int64`: `gid` (group member, manager and subgroup methods), `accountId`, `relationshipId`, `summaryFid` and `TransferUserToken`'s `Id`. Every other such number is `float64`: `FieldValueValue1` (a record's numeric value), app memory estimates, report column `Justification` and goal `Value`, and analytics totals. Callers passing a `float32` pass an `int64` or `float64` instead.
- **Generated from QuickBase's current spec.** The spec now builds from the OpenAPI 3 that QuickBase's API portal publishes (`spec/tools/fetch-upstream.ts`) instead of a December 2025 Swagger 2.0 copy. `apidiff` (golang.org/x/exp) counts 198 incompatible changes against the previous commit; by kind:
  - **BREAKING** (18): fields QuickBase marks required became values instead of pointers: app variables' `Name` and `Value` (`CreateApp`, `UpdateApp`, `CopyApp`, `GetApp`), `TransferUserToken`'s `From`, `To` and `Id`, and upsert's `Data` (`Data: data`, not `Data: &data`).
  - **BREAKING** (46 + 15): inline struct types became named types (every object now keeps unlisted keys, which needs a named type with `AdditionalProperties`): field-usage counts (`Dashboards`, `TableImports`, `TableRules`), trustee items, analytics summaries, request bodies such as `AddTrusteesJSONRequestBody`, and others. Reading fields is unchanged; code that writes these as struct literals must use the named type.
  - **BREAKING** (40): types that now carry `AdditionalProperties` are no longer comparable with `==` (for example `AddTrusteesFailureItem`).
  - **BREAKING** (8): `GenerateDocument`'s `templateId` and `recordId` are `int`, not `float32`.
  - **BREAKING**: audit payload changes (`Current`, `Previous`, `Fields`) are typed items instead of `[]interface{}`.
  - **BREAKING**: `RunReport` takes no request body (the API never read one): `RunReportBody`, `RunReportJSONRequestBody` and `RunReportWithBody` are gone; use the builder.
  - Solution responses are typed more closely: `CreateSolutionResponse.JSON200` and `ChangesetSolutionResponse.JSON200` are `*map[string]interface{}`, and `GetSolutionPublicResponse`'s error responses are error structs.
  - `GenerateDocumentParams.Accept` is a `*string` (the spec no longer lists its values).
- **BREAKING**: Report `Properties` (`GetTableReports()`, `GetReport()`) changed from `*map[string]interface{}` to a struct with typed fields for table, chart, summary and map report settings (`ColumnProperties`, `ChartType`, `Summarize`, ...). Keys not listed are kept in `AdditionalProperties`. Replace `(*props)["columnProperties"]` with `props.ColumnProperties`, and other keys with `props.AdditionalProperties["key"]`.
- `Query.Fields`, `Query.SortBy` and `Query.GroupBy` on reports are now typed fields, so they no longer appear in `Query.AdditionalProperties`.

### Fixed

- **Read-only mode let two solution writes through**: `UpdateSolutionToRecord` (`GET /solutions/{id}/fromrecord`, which changes apps) and `ExportSolutionToRecord` (`GET /solutions/{id}/torecord`, which writes a record) are GETs, and only `/solutions/fromrecord` was blocked. Both now return `ReadOnlyError`. `ChangesetSolutionFromRecord` only reads and is still allowed.
- **`Value` rounded numbers**: it sent every number as `float32`, so `Value(1234567.89)` wrote 1234567.875 and `Value(20000001)` wrote 20000000. Numbers are sent as `float64`.
- **A cancelled request reported a timeout**: cancelling the context returned a `TimeoutError` naming the client's timeout. It now returns the context's error (`context.Canceled` or `context.DeadlineExceeded`), and `TimeoutError` means `WithTimeout` ran out, which before came back as a raw `*url.Error`.
- **A 401 retried after a token refresh leaked its connection**: the response body wasn't closed.
- **`WithConvertDates` did nothing** (no code read it) and is deprecated. Dates stay the strings QuickBase sends.
- **`SchemaOptions.TransformResponses` did nothing**: no code read it, so records were always keyed by field ID although it was documented as on by default. It now keys the query builder's records (`Run`, `RunN`) by alias, and is off by default (`DefaultSchemaOptions` returns it false), so nothing changes unless you turn it on with `WithSchemaOptions`.
- **`RunQueryAll` and `RunQueryN` ignored schema aliases**: `RunQuery` resolved them, but the paging methods sent the body as given, so `From: "projects"` or `'status'` in a filter reached QuickBase unresolved. They now resolve aliases first. They also set each page's skip on the caller's `Options`, changing the caller's body; they now use a copy.
- **Cancelling a request didn't cancel its retry wait**: backoff waits used `time.Sleep`, so a call whose context ended kept waiting out the backoff (up to 30s by default). The wait now returns the context's error as soon as it ends.
- **`WithMaxRetries(0)` made no request**: the option counts attempts in all (default 3: one try and two retries), so 0 skipped the request and returned neither a response nor an error, and callers dereferenced nil. Values below 1 now mean a single attempt; the doc comments said "retry attempts" and now describe what it counts.
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
