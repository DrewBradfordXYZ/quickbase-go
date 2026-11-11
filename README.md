# QuickBase Go SDK

A Go client for the [QuickBase JSON API](https://developer.quickbase.com/), with the legacy XML API for what JSON lacks.

[![Go Reference](https://pkg.go.dev/badge/github.com/DrewBradfordXYZ/quickbase-go/v2.svg)](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go/v2)

- **Every operation** as a fluent builder, generated from a spec checked against live responses
- **Queries that paginate** for you, with readable **schema aliases** instead of table and field IDs
- **Retries, throttling and hooks** for QuickBase's rate limits
- **Read-only mode** for tools that must never write
- **User token, ticket, temporary token and SSO** authentication

```bash
go get github.com/DrewBradfordXYZ/quickbase-go/v2
```

## Quick start

```go
client, err := quickbase.New("myrealm", quickbase.WithUserToken(os.Getenv("QB_USER_TOKEN")))
if err != nil {
	log.Fatal(err)
}
defer client.Close()
ctx := context.Background()

app, err := client.GetApp("bqxyz123").Run(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println("App:", app.Name())

records, err := client.Query("bqtable1").
	Select(3, 6, 7).
	Where("{7.EX.'Active'}").
	Run(ctx) // every page; RunN(ctx, n) stops at n records
if err != nil {
	log.Fatal(err)
}
for _, rec := range records {
	fmt.Println(rec["6"]) // values by field ID
}

result, err := client.Upsert("bqtable1").
	Data(quickbase.Row(6, "New project", 7, "Active")).
	Run(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println("Created:", result.Metadata().CreatedRecordIds())
```

## Builders for every operation

Each QuickBase operation is a method that returns a builder: set what you need, then `Run(ctx)`. Results have nil-safe accessors, so an absent value is its zero value rather than a nil pointer.

```go
tables, _ := client.GetAppTables().AppId("bqxyz123").Run(ctx)
for _, t := range tables {
	fmt.Println(t.Id(), t.Name())
}

fields, _ := client.GetFields("bqtable1").Run(ctx)
for _, f := range fields {
	fmt.Println(f.Id(), f.Label(), f.FieldType())
}

newApp, _ := client.CreateApp().Name("My App").Description("Created via API").Run(ctx)
deleted, _ := client.DeleteRecords("bqtable1").Where("{3.EX.123}").Run(ctx)
```

The builders and result types are generated from [quickbase-spec](https://github.com/DrewBradfordXYZ/quickbase-spec), so they follow QuickBase's API as it is, including properties QuickBase returns but doesn't document. Browse them on [pkg.go.dev](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go/v2/client).

## Queries and pagination

QuickBase returns a page of records at a time (often about 100). The query builder fetches every page; `RunQuery` gives you one page and its metadata.

```go
body := quickbase.RunQueryBody{
	From:   "bqtable1",
	Select: quickbase.Ints(3, 6, 7),
	Where:  quickbase.Where("{6.GT.100}"),
	SortBy: quickbase.SortBy(quickbase.Sort(6, quickbase.ASC)),
}

page, _ := client.RunQuery(ctx, body)
fmt.Printf("%d of %d records\n", page.Metadata().NumRecords(), page.Metadata().TotalRecords())

all, _ := client.RunQueryAll(ctx, body)      // every page
first, _ := client.RunQueryN(ctx, body, 500) // up to 500
for record, err := range client.RunQueryIterator(ctx, body) { // a page at a time
	_, _ = record, err
}
```

More in [docs/queries.md](docs/queries.md): running reports, upserts, deletes, and the helpers (`Row`, `Value`, `Ints`, `Ptr`, ...).

## Schema aliases

Name your tables and fields once, then use the names everywhere. The client translates them to IDs in requests.

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

active, _ := client.Query("projects").
	Select("name", "status").
	Where("{'status'.EX.'Active'}").
	SortBy(quickbase.Asc("name")).
	Run(ctx)
```

A misspelled alias is an error that suggests the closest name. `go run ./cmd/schema` generates a schema from an existing app. See [docs/schema-aliases.md](docs/schema-aliases.md).

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

Solutions are QBL, which is YAML, and file downloads are raw content. Their builders send and return bytes:

```go
exported, _ := client.ExportSolution("solution-id").Run(ctx)
qbl := exported.Body // the QBL document

client.UpdateSolution("solution-id").Body(qbl).Run(ctx)

file, _ := client.DownloadFile("bqtable1", 12, 8, 0).Run(ctx)
fmt.Println(len(file.Body)) // base64 file content
```

## Going lower

Every result's `Raw()` returns the full generated response, headers included, and `client.API()` is the generated client itself, for anything the builders don't cover.

```go
resp, _ := client.API().GetAppWithResponse(ctx, "bqxyz123")
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
- [docs/schema-aliases.md](docs/schema-aliases.md): aliases, the schema generator, loading from JSON
- [docs/authentication.md](docs/authentication.md): every authentication method
- [docs/reliability.md](docs/reliability.md): rate limits, retries, throttling, monitoring
- [docs/xml-api.md](docs/xml-api.md): the XML API
- [MIGRATION.md](MIGRATION.md): moving from v1
- [CHANGELOG.md](CHANGELOG.md)
- [pkg.go.dev](https://pkg.go.dev/github.com/DrewBradfordXYZ/quickbase-go/v2): the API reference and runnable examples

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
