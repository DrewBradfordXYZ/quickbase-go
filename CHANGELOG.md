# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [3.0.0] - 2026-11-12

v3: one method per operation, with plain results. The module path is `github.com/DrewBradfordXYZ/quickbase-go/v3`. [MIGRATION.md](MIGRATION.md) maps every v2 call; [docs/architecture.md](docs/architecture.md) has the reasons.

### Changed
- **BREAKING: one method per operation.** `client.GetApp(ctx, appID)`, `client.GetFields(ctx, tableID, nil)`, `client.Upsert(ctx, quickbase.UpsertBody{...})`: required parameters are arguments (path or query, wherever QuickBase puts them), a body its request type, and optional query parameters an `*<Operation>Options`, last, that can be `nil`. They return the result (`*GetAppResult`, `[]GetFieldsItem`, `[]byte` for QBL and files) or QuickBase's typed error. Generated from the spec (`cmd/generate-methods`).
- **BREAKING: results have plain fields.** Response types' optional fields are values, not pointers (`app.Name`, `result.Metadata.CreatedRecordIds`), and every response is a named type (`<Operation>Result`). `cmd/gospec` adapts the shared spec for Go; generated with oapi-codegen v2.8.2, whose runtime (v1.8.0) still needs only Go 1.24.
- **BREAKING: a record query's `Where` is a string and `SortBy` a list**: `Where: "{7.EX.'Active'}"`, `SortBy: quickbase.SortBy{quickbase.Asc(6)}`, `quickbase.NoSort` to turn sorting off (sends `false`), and `quickbase.RecordIDs(12, 15)` for a list of records. `DeleteRecordsBody.Where` is a string too. `quickbase.Where`, `quickbase.DeleteWhere`, `quickbase.SortBy(...)`, `quickbase.Sort` and their union types are removed.
- Date query parameters (analytics' `day`) are `quickbase.Date`.
- **BREAKING: a record value is a plain Go value** (`any`), not a union type: `rec["6"].Value.(string)`, `quickbase.FieldValue{Value: 42}`.
- **BREAKING: schema aliases are resolved for every method** in the HTTP layer, not only by `RunQuery` and `Query()`: a table in a path (`/tables/{tableId}`, `/files/{tableId}`) or a `tableId` parameter; and, for record operations (query, upsert, delete records, modified since, run formula), the body's `from`, `to`, `select`, `sortBy`, `groupBy`, `where`, record data, `fieldsToReturn` and `mergeFieldId`. A table reference the schema doesn't name goes as given (it was an error), unless it is a near miss of an alias, which is an error suggesting it.
- `TransformResponses` keys the paging helpers' records by alias (`RunQueryAll`, `RunQueryN`, `RunQueryIterator`, `RunReport*`); it keyed `Query()`'s.
- Every generated type is re-exported from the root package under its own name (`quickbase.GetAppResult`), so callers need one import.
- **BREAKING: one record type, with a decoder.** `quickbase.Record` (`core.Record`) is a record as QuickBase sends it, field ID to `{Value}`, everywhere: query and report results, upsert data. `rec.Values()` gives a plain map, `rec.Decode(&v)` and `quickbase.DecodeRecords[T]` fill structs by `qb:"<id or alias>"` tags (a blank value leaves the zero value; `Date`, `DateTime` and `User` decode from their forms), and `quickbase.RecordFrom(v)` builds a record to send, leaving out nil pointer fields. `UnwrapRecord`, `UnwrapRecords` and the `client.Record` map type are removed.

### Added
- **Typed tables.** `go run ./cmd/schema -f structs` writes, for every table of an app, a record struct and a changes struct typed from QuickBase's fields, the field IDs, and a `quickbase.Table[Record, Changes]` with `Query`, `Iterate`, `Get`, `Upsert` and `Delete`. A changes struct's fields are pointers, so a nil field isn't sent, and it leaves out what an upsert can't set (formula, lookup and summary fields, built-in fields), so writing one doesn't compile. `--merge` keeps renamed types and fields. What each field type reads and writes as was checked against a live realm, including that a user field is set by email (a bare user ID stores a wrong user without an error). See [docs/tables.md](docs/tables.md).
- `-p` sets the package of the schema generator's Go output, which was always `main`.
- `GetRelationshipsIterator`, `GetUsersIterator` and `AuditIterator`: every relationship, user or audit event, a page at a time.
- Dates in request bodies and results (the audit log's `Date`, analytics' daily reads) are `quickbase.Date`, not oapi-codegen's date type. A blank (`""` or null) decodes as the zero `Date` or `DateTime`.

### Removed
- **BREAKING:** the fluent builders, the result wrappers with accessor methods (`Raw()`, `Name()`, ...), the `Query()` builder, `SortSpec`, and the union helpers for record values (`FieldValueUnion`). `client.API()` remains for the full generated response.
- **BREAKING:** what v2 deprecated or never used: `PaginatedRequest`, `NewPaginatedRequest`, `PaginationOptions`, `PaginationType` and its constants, `DetectPaginationType` and `HasMorePages` (page with `RunQueryAll`, `RunQueryIterator` and the other iterators, or `Paginate`); `WithConvertDates`, which did nothing; `TransformDates`, `IsISODateString` and `ParseISODate`, which guessed dates from a string's shape (use `ParseDate`, `ParseDateTime`, or `Record.Decode` into `Date` and `DateTime`); `core.TransformResponse`, which nothing called; and `quickbase.Error`, which nothing returned.

### Fixed
- The schema generator's Go output imports `github.com/DrewBradfordXYZ/quickbase-go/v3`; it imported the module path without its major version, which didn't compile.

## [2.4.0] - 2026-11-12

### Added

- **`WithLineErrorsAsErrors`**: `Upsert` returns a `LineErrorsError` when QuickBase rejects values, with the result, so the saved records' IDs aren't lost. Without it, rejections are only in the result's `Metadata().LineErrors`.
- **Auth options in the `client` package** (`client.WithUserToken`, `WithTempTokens`, `WithSSOTokenAuth`, `WithTicketAuth`, `WithTicket`, and `WithAuth` for any `auth.Strategy`); `client.New` takes a nil strategy when one is set.
- **`Date` and `DateTime`**, values for Date and Date/Time fields, which QuickBase accepts in different forms (a Date field only `"2024-01-15"`, a Date/Time field only RFC 3339; checked against a live realm). `DateOf(t)` is t's day in t's location; `DateTimeOf(t)` marshals in UTC. `ParseDate` and `ParseDateTime` read what QuickBase returns. See [docs/queries.md](docs/queries.md#dates).
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

- **BREAKING: writes retry only after a 429.** An upsert or other write that timed out or got a 5xx may have been applied, and sending it again could add its records twice. Writes still retry after a 429 (QuickBase didn't process it), after a 401 with a refreshed token, and when the connection never opened. Reads, including `RunQuery` and `RunReport` (POSTs), retry as before.
- **BREAKING: `quickbase.Option` is `client.Option`** (a type alias), and the top-level `With` functions return the `client` package's options, which document them in full. The two copies had drifted.
- `OnRequest` fires once per attempt, including attempts that refreshed a token.
- Generated code is gofmt-formatted, and CI checks formatting.
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

- **`Upsert` lost its result on a 207**: QuickBase answers 207 when it rejected some values and saved the rest, and `Upsert` returned `Multi-Status (status: 207)` as an error with no result, so the IDs of the records it saved were lost. It now returns the result; the rejections are in `Metadata().LineErrors`.
- **Read-only mode and retries used the wrong paths under `WithBaseURL`**: the endpoint lists name paths under `/v1`, and another base URL's paths didn't match, so read-only mode let GETs that write through (`GenerateDocument`, solutions to and from records) and `RunQuery` counted as a write. Paths are now matched relative to the base URL.
- **A token refreshed on the last attempt returned neither a response nor an error**, and the caller dereferenced nil. The last 401 is now the result.
- An app token or auth token in an XML request body is escaped.

### Deprecated

- `PaginatedRequest` and `NewPaginatedRequest`: nothing returns one. Use `Paginate`, `CollectAll`, `CollectN`, or `RunQueryAll`, `RunQueryN` and `RunQueryIterator`.
- **BREAKING: `Value` sent `null` for every type it didn't list**, which clears the field: a `time.Time`, a `uint`, a file attachment's map. It now sends `Date` and `DateTime` in their fields' forms and anything else as its JSON, and panics on a `time.Time`, which could be either kind of date; wrap it with `DateOf` or `DateTimeOf`. `nil` still clears the field.
- **BREAKING: `Upsert` refuses a `time.Time` in its data** with a `PlainTimeError`, before sending. It went as a timestamp with its offset, which a Date field rejects, leaving the field blank on a record QuickBase still saves.
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
