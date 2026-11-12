# QuickBase Go SDK

A Go client for the [QuickBase JSON API](https://developer.quickbase.com/), with the legacy XML API for what JSON lacks.

[![Go Reference](https://pkg.go.dev/badge/github.com/DrewBradfordXYZ/quickbase-go/v3.svg)](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go/v3)

- **Every operation** as one method, generated from a spec checked against live responses: request struct in, plain result out
- **Queries that paginate** for you, with readable **schema aliases** instead of table and field IDs
- **Retries, throttling and hooks** for QuickBase's rate limits
- **Read-only mode** for tools that must never write
- **User token, ticket, temporary token and SSO** authentication

```bash
go get github.com/DrewBradfordXYZ/quickbase-go/v3
```

## Quick start

```go
client, err := quickbase.New("myrealm", quickbase.WithUserToken(os.Getenv("QB_USER_TOKEN")))
if err != nil {
	log.Fatal(err)
}
defer client.Close()
ctx := context.Background()

app, err := client.GetApp(ctx, "bqxyz123")
if err != nil {
	log.Fatal(err)
}
fmt.Println("App:", app.Name)

records, err := client.RunQueryAll(ctx, quickbase.RunQueryBody{
	From:   "bqtable1",
	Select: quickbase.Ints(3, 6, 7),
	Where:  quickbase.Where("{7.EX.'Active'}"),
}) // every page; RunQueryN(ctx, body, n) stops at n records
if err != nil {
	log.Fatal(err)
}
for _, rec := range records {
	fmt.Println(rec["6"].Value) // values by field ID
}

result, err := client.Upsert(ctx, quickbase.UpsertBody{
	To:   "bqtable1",
	Data: []quickbase.Record{quickbase.Row(6, "New project", 7, "Active")},
})
if err != nil {
	log.Fatal(err)
}
fmt.Println("Created:", result.Metadata.CreatedRecordIds)
```

## One method per operation

Every QuickBase operation is a method on the client. Path parameters are arguments, query parameters a params struct, and the body its request type. A method returns the result or QuickBase's error. Results have plain fields: a value QuickBase didn't send is its zero value, so there's nothing to nil-check.

```go
tables, _ := client.GetAppTables(ctx, quickbase.GetAppTablesParams{AppId: "bqxyz123"})
for _, t := range tables {
	fmt.Println(t.Id, t.Name)
}

fields, _ := client.GetFields(ctx, quickbase.GetFieldsParams{TableId: "bqtable1"})
for _, f := range fields {
	fmt.Println(f.Id, f.Label, f.FieldType)
}

newApp, _ := client.CreateApp(ctx, quickbase.CreateAppBody{Name: "My App", Description: quickbase.Ptr("Created via API")})
deleted, _ := client.DeleteRecords(ctx, quickbase.DeleteRecordsBody{From: "bqtable1", Where: quickbase.DeleteWhere("{3.EX.123}")})
```

Optional request fields are pointers (`quickbase.Ptr`), so a `false` or `0` you set reaches QuickBase. The methods and types are generated from [quickbase-spec](https://github.com/DrewBradfordXYZ/quickbase-spec), so they follow QuickBase's API as it is, including properties QuickBase returns but doesn't document. Browse them on [pkg.go.dev](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go/v3/client). Why it's shaped this way: [docs/architecture.md](docs/architecture.md).

## Queries and pagination

QuickBase returns a page of records at a time (often about 100). `RunQuery` gives you one page and its metadata; `RunQueryAll`, `RunQueryN` and `RunQueryIterator` fetch the pages for you.

```go
body := quickbase.RunQueryBody{
	From:   "bqtable1",
	Select: quickbase.Ints(3, 6, 7),
	Where:  quickbase.Where("{6.GT.100}"),
	SortBy: quickbase.SortBy(quickbase.Sort(6, quickbase.ASC)),
}

page, _ := client.RunQuery(ctx, body)
fmt.Printf("%d of %d records\n", page.Metadata.NumRecords, page.Metadata.TotalRecords)

all, _ := client.RunQueryAll(ctx, body)      // every page
first, _ := client.RunQueryN(ctx, body, 500) // up to 500
for record, err := range client.RunQueryIterator(ctx, body) { // a page at a time
	_, _ = record, err
}
```

More in [docs/queries.md](docs/queries.md): running reports, upserts, deletes, and the helpers (`Row`, `Value`, `Ints`, `Ptr`, ...).

## Schema aliases

Name your tables and fields once, then use the names in any request: the client translates them to IDs in bodies (`From`, `To`, `Where`, record data) and in a `TableId` parameter.

```go
schema := quickbase.NewSchema().
	Table("projects", "bqw3ryzab").
	Field("id", 3).
	Field("name", 6).
	Field("status", 7).
	Build()

client, _ := quickbase.New("myrealm",
	quickbase.WithUserToken("token"),
	quickbase.WithSchema(schema),
)

active, _ := client.RunQueryAll(ctx, quickbase.RunQueryBody{
	From:   "projects",
	Select: quickbase.Fields(schema, "projects", "name", "status"),
	Where:  quickbase.Where("{'status'.EX.'Active'}"),
})
```

A misspelled alias is an error that suggests the closest name. `go run ./cmd/schema` generates a schema from an existing app. See [docs/schema-aliases.md](docs/schema-aliases.md).

## Typed tables

`go run ./cmd/schema -f structs` turns every table of an app into Go types, typed from QuickBase's fields, and queries and upserts use them:

```go
paid, err := app.Payments.Query(ctx, qb, quickbase.TableQuery{
	Where: fmt.Sprintf("{%d.EX.'Paid'}", app.PaymentFields.PaymentStatus),
})
fmt.Println(paid[0].PaymentDate, *paid[0].PaymentAmount)

_, err = app.Payments.Upsert(ctx, qb, app.PaymentChanges{RecordID: &id, PaymentStatus: quickbase.Ptr("Paid")}) // only this field changes
```

Formula, lookup and summary fields are in the record struct but not the changes struct, so writing one doesn't compile. See [docs/tables.md](docs/tables.md).

## Authentication

| Method | Option | Use it for |
|---|---|---|
| User token | `WithUserToken(token)` | Scripts and services |
| Ticket | `WithTicketAuth(email, password)` | Acting as a user who signs in; records show them as creator |
| Temporary tokens | `WithTempTokens(map[table]token)` | A server called from a code page, with tokens the browser fetched |
| SSO | `WithSSOTokenAuth(samlAssertion)` | Acting as a user signed in through your identity provider |
| App token | `WithAppToken(token)` | Only XML API calls with ticket or temporary-token auth |

Details, including the temporary-token and SSO flows: [docs/authentication.md](docs/authentication.md).

## Rate limits, retries and monitoring

QuickBase allows 100 requests per 10 seconds per user token. The client retries 429s and server errors with exponential backoff (honoring `Retry-After`), can throttle itself so it never hits the limit, and reports every request and retry to hooks you provide.

```go
quickbase.New("myrealm",
	quickbase.WithUserToken("token"),
	quickbase.WithMaxRetries(5),                 // attempts in all, default 3
	quickbase.WithProactiveThrottle(100),        // at most 100 requests per 10s
	quickbase.WithTimeout(60*time.Second),       // per request, default 30s
	quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
		log.Printf("%s %s -> %d (%s)", info.Method, info.Path, info.StatusCode, info.Duration)
	}),
)
```

See [docs/reliability.md](docs/reliability.md) for every option, batch-job tuning and a Prometheus example.

## Read-only mode

`WithReadOnly()` makes the client refuse every write before it is sent: JSON writes (including the GET endpoints that write, such as `generateDocument`), and XML write actions. Reads that use POST, like `RunQuery`, `RunReport` and `Audit`, still work. A blocked call returns `*quickbase.ReadOnlyError`.

## Errors

Errors are typed by cause: `RateLimitError` (429), `AuthenticationError` (401), `AuthorizationError` (403), `NotFoundError` (404), `ValidationError` (400), `ServerError` (5xx), `TimeoutError` and `ReadOnlyError`.

```go
var rateLimited *quickbase.RateLimitError
var notFound *quickbase.NotFoundError
switch {
case errors.As(err, &rateLimited):
	log.Printf("rate limited; retry after %ds", rateLimited.RetryAfter)
case errors.As(err, &notFound):
	log.Print("no such app")
}
```

## Solutions and files

Solutions are QBL, which is YAML, and file downloads are raw content. Their methods send and return bytes:

```go
qbl, _ := client.ExportSolution(ctx, "solution-id", quickbase.ExportSolutionParams{}) // the QBL document

client.UpdateSolution(ctx, "solution-id", quickbase.UpdateSolutionParams{}, qbl)

file, _ := client.DownloadFile(ctx, "bqtable1", 12, 8, 0)
fmt.Println(len(file)) // base64 file content
```

## Going lower

`client.API()` is the generated client itself, for the full response (status, headers):

```go
resp, _ := client.API().GetAppWithResponse(ctx, "bqxyz123")
fmt.Println(resp.StatusCode(), resp.HTTPResponse.Header.Get("qb-api-ray"))
if resp.JSON200 != nil {
	fmt.Println(resp.JSON200.Name)
}
```

## The legacy XML API

The `xml` package reaches what the JSON API doesn't: roles, groups, code pages, app variables, webhooks, CSV imports. It shares the main client's authentication, retries and throttling.

```go
x := xml.New(client)
roles, _ := x.GetRoleInfo(ctx, "bqxyz123")
for _, role := range roles.Roles {
	fmt.Println(role.ID, role.Name, role.Access.Description)
}
```

Every method, schema aliases and error handling: [docs/xml-api.md](docs/xml-api.md). How the two APIs compare: [docs/xml-api-reference.md](docs/xml-api-reference.md).

## Documentation

- [docs/queries.md](docs/queries.md): queries, pagination, upserts, deletes and helpers
- [docs/tables.md](docs/tables.md): typed tables generated from an app
- [docs/schema-aliases.md](docs/schema-aliases.md): aliases, the schema generator, loading from JSON
- [docs/authentication.md](docs/authentication.md): every authentication method
- [docs/reliability.md](docs/reliability.md): rate limits, retries, throttling, monitoring
- [docs/xml-api.md](docs/xml-api.md): the XML API
- [docs/architecture.md](docs/architecture.md): why the SDK is shaped as it is
- [MIGRATION.md](MIGRATION.md): moving from v2 (and from v1)
- [CHANGELOG.md](CHANGELOG.md)
- [pkg.go.dev](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go/v3): the API reference and runnable examples

## Development

```bash
git clone --recurse-submodules https://github.com/DrewBradfordXYZ/quickbase-go.git
go test ./...                 # integration tests run when QB_REALM and QB_USER_TOKEN are set (.env)
make generate                 # regenerate from the pinned spec in spec/
```

`spec/` is a submodule of [quickbase-spec](https://github.com/DrewBradfordXYZ/quickbase-spec). To take a newer spec, check out its commit in `spec/` and run `make generate`. CI requires the generated code to match the spec, and fails on incompatible API changes (`apidiff`) that `CHANGELOG.md` doesn't mark **BREAKING**.

The code in this README and `docs/` comes from `docs_examples_test.go`, which the compiler checks.

## Related

- [quickbase-js](https://github.com/DrewBradfordXYZ/quickbase-js): the TypeScript SDK, from the same spec

## License

MIT
