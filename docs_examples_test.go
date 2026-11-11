package quickbase_test

// The code in README.md and docs/*.md is copied from these examples, which
// the compiler checks (go vet, go test). Change them together.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v2"
	"github.com/DrewBradfordXYZ/quickbase-go/v2/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/v2/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v2/xml"
)

// The README's quick start: get an app, query a table, add records.
func Example_quickStart() {
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
}

// Builders exist for every operation; results have nil-safe accessors.
func Example_builders() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	tables, _ := client.GetAppTables().AppId("bqxyz123").Run(ctx)
	for _, t := range tables {
		fmt.Println(t.Id(), t.Name())
	}

	fields, _ := client.GetFields("bqtable1").Run(ctx)
	for _, f := range fields {
		fmt.Println(f.Id(), f.Label(), f.FieldType())
	}

	newApp, _ := client.CreateApp().Name("My App").Description("Created via API").Run(ctx)
	fmt.Println(newApp.Id())

	deleted, _ := client.DeleteRecords("bqtable1").Where("{3.EX.123}").Run(ctx)
	fmt.Println(deleted.NumberDeleted())
}

// RunQuery returns one page; RunQueryAll and RunQueryN paginate.
func Example_runQuery() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()
	body := quickbase.RunQueryBody{
		From:   "bqtable1",
		Select: quickbase.Ints(3, 6, 7),
		Where:  quickbase.Where("{6.GT.100}"),
		SortBy: quickbase.SortBy(quickbase.Sort(6, quickbase.ASC)),
	}

	page, _ := client.RunQuery(ctx, body)
	fmt.Printf("%d of %d records\n", page.Metadata().NumRecords(), page.Metadata().TotalRecords())
	for _, rec := range page.Records() {
		fmt.Println(rec["6"])
	}

	all, _ := client.RunQueryAll(ctx, body)      // every page
	first, _ := client.RunQueryN(ctx, body, 500) // up to 500
	fmt.Println(len(quickbase.UnwrapRecords(all)), len(first))
}

// Schema aliases: readable table and field names instead of IDs.
func Example_schemaAliases() {
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
	ctx := context.Background()

	active, _ := client.Query("projects").
		Select("name", "status").
		Where("{'status'.EX.'Active'}").
		SortBy(quickbase.Asc("name")).
		Run(ctx)
	fmt.Println(len(active))

	_, _ = client.Upsert("projects").
		Data(quickbase.Row("name", "Alpha", "status", "Active")).
		Run(ctx)
}

// Loading a schema from JSON (as cmd/schema -f json writes it).
func Example_schemaFromJSON() {
	data, err := os.ReadFile("schema.json")
	if err != nil {
		log.Fatal(err)
	}
	var schema quickbase.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		log.Fatal(err)
	}
	_, _ = quickbase.New("myrealm", quickbase.WithUserToken("token"), quickbase.WithSchema(&schema))
}

// Authentication methods.
func Example_authentication() {
	// User token: server-side scripts and services
	_, _ = quickbase.New("myrealm", quickbase.WithUserToken("b9f3pk_xxx"))

	// Ticket: a user's email and password; records show that user as creator
	_, _ = quickbase.New("myrealm",
		quickbase.WithTicketAuth("user@example.com", "password", auth.WithTicketHours(24*7)),
	)

	// Temporary tokens a browser (code page) fetched and sent to your server, per table
	_, _ = quickbase.New("myrealm",
		quickbase.WithTempTokens(map[string]string{"bqtable1": "token-from-browser"}),
	)

	// SSO: exchange a SAML assertion for a token that acts as that user
	_, _ = quickbase.New("myrealm", quickbase.WithSSOTokenAuth("base64url-saml-assertion"))

	// App token: only XML API calls with ticket or temp-token auth need one
	_, _ = quickbase.New("myrealm",
		quickbase.WithTicketAuth("user@example.com", "password"),
		quickbase.WithAppToken("app-token"),
	)
}

// Retries, throttling, timeouts and hooks.
func Example_reliability() {
	_, _ = quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithMaxRetries(5),                 // attempts in all, default 3
		quickbase.WithRetryDelay(time.Second),       // first backoff, default 1s
		quickbase.WithMaxRetryDelay(30*time.Second), // backoff cap, default 30s
		quickbase.WithTimeout(60*time.Second),       // per request, default 30s
		quickbase.WithProactiveThrottle(100),        // at most 100 requests per 10s
		quickbase.WithMaxIdleConnsPerHost(10),       // default 6
		quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
			log.Printf("%s %s -> %d (%s)", info.Method, info.Path, info.StatusCode, info.Duration)
		}),
		quickbase.WithOnRetry(func(info quickbase.RetryInfo) {
			log.Printf("retrying %s (attempt %d, %s, in %s)", info.Path, info.Attempt, info.Reason, info.WaitTime)
		}),
		quickbase.WithOnRateLimit(func(info quickbase.RateLimitInfo) {
			log.Printf("rate limited; retry after %ds", info.RetryAfter)
		}),
	)
}

// Typed errors.
func Example_errors() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"), quickbase.WithReadOnly())
	_, err := client.GetApp("bqxyz123").Run(context.Background())

	var rateLimited *quickbase.RateLimitError
	var notFound *quickbase.NotFoundError
	var readOnly *quickbase.ReadOnlyError
	switch {
	case errors.As(err, &rateLimited):
		log.Printf("rate limited; retry after %ds", rateLimited.RetryAfter)
	case errors.As(err, &notFound):
		log.Print("no such app")
	case errors.As(err, &readOnly):
		log.Printf("write blocked: %s %s", readOnly.Method, readOnly.Path)
	case err != nil:
		log.Print(err)
	}
}

// Solutions are QBL (YAML); file downloads are raw content.
func Example_solutionsAndFiles() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	exported, _ := client.ExportSolution("solution-id").Run(ctx)
	qbl := exported.Body // the QBL document

	_, _ = client.UpdateSolution("solution-id").Body(qbl).Run(ctx)

	file, _ := client.DownloadFile("bqtable1", 12, 8, 0).Run(ctx)
	fmt.Println(len(file.Body)) // base64 file content
}

// Escape hatches: the full response, and the generated client.
func Example_rawAccess() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	app, _ := client.GetApp("bqxyz123").Run(ctx)
	if raw := app.Raw(); raw != nil && raw.JSON200 != nil {
		fmt.Println(raw.HTTPResponse.Header.Get("qb-api-ray"))
	}

	resp, _ := client.API().GetAppWithResponse(ctx, "bqxyz123")
	if resp.JSON200 != nil {
		fmt.Println(resp.JSON200.Name)
	}
}

// The legacy XML API, for what the JSON API lacks (roles, groups, code pages).
func Example_xmlAPI() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	x := xml.New(client)
	ctx := context.Background()

	roles, err := x.GetRoleInfo(ctx, "bqxyz123")
	if err != nil {
		var xmlErr *xml.Error
		if errors.As(err, &xmlErr) {
			log.Printf("XML error %d: %s", xmlErr.Code, xmlErr.Text)
		}
		return
	}
	for _, role := range roles.Roles {
		fmt.Println(role.ID, role.Name, role.Access.Description)
	}

	page, _ := x.GetDBPage(ctx, "bqxyz123", "dashboard.html")
	fmt.Println(len(page))
}

// docs/queries.md: the query builder's options, reports, upserts, helpers.
func Example_queriesGuide() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	// Query builder: fields and sorts by ID (or alias, with a schema)
	records, _ := client.Query("bqtable1").
		Select(3, 6, 7).
		Where("{7.EX.'Active'}").
		SortBy(quickbase.Asc(6), quickbase.Desc(3)).
		GroupBy(7).
		RunN(ctx, 1000) // stop at 1,000 records
	fmt.Println(len(records))

	// One page, with metadata
	page, _ := client.Query("bqtable1").Select(3, 6).Top(50).Skip(100).RunRaw(ctx)
	fmt.Println(page.Metadata().TotalRecords())

	// Reports run with the report's own filter, sort and columns
	reportRows, _ := client.RunReportAll(ctx, "7", "bqtable1")
	fmt.Println(len(quickbase.UnwrapRecords(reportRows)))

	// Upsert: insert, or update the record whose merge field matches
	result, _ := client.Upsert("bqtable1").
		MergeFieldId(9). // a unique field, e.g. an external ID
		Data(
			quickbase.Row(9, "EXT-001", 6, "Alpha", 7, "Active"),
			quickbase.Row(9, "EXT-002", 6, "Beta", 7, "Pending"),
		).
		FieldsToReturn(6, 7).
		Run(ctx)
	fmt.Println(result.Metadata().CreatedRecordIds(), result.Metadata().UpdatedRecordIds())

	// Values with explicit types, for when Row's inference isn't what you want
	_ = quickbase.Record{"6": quickbase.Value("text"), "8": quickbase.Value([]string{"a", "b"})}

	// Pointers for optional fields
	_ = quickbase.Ptr(100)
	_ = quickbase.DerefOr((*string)(nil), "default")
}

// docs/schema-aliases.md: aliases in RunQueryBody.
func Example_schemaAliasesBody() {
	schema := quickbase.NewSchema().Table("projects", "bqw3ryzab").Field("name", 6).Field("status", 7).Build()
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"), quickbase.WithSchema(schema))
	page, _ := client.RunQuery(context.Background(), quickbase.RunQueryBody{
		From:   "projects",
		Select: quickbase.Fields(schema, "projects", "name", "status"),
		Where:  quickbase.Where("{'status'.EX.'Active'}"),
	})
	for _, rec := range page.Records() {
		fmt.Println(rec["6"])
	}
	_ = &quickbase.Schema{Tables: map[string]quickbase.TableSchema{"projects": {ID: "bqw3ryzab", Fields: map[string]int{"id": 3}}}}
}

// docs/authentication.md: a server called from a code page, with the
// temporary token the browser fetched for a table.
func Example_tempTokenHandler() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		client, err := quickbase.New("myrealm",
			quickbase.WithTempTokens(map[string]string{
				"bqtable1": r.Header.Get("X-QB-Token-bqtable1"),
			}),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		records, err := client.Query("bqtable1").Select(3, 6).Run(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		fmt.Fprintf(w, "%d records", len(records))
	}
	http.HandleFunc("/records", handler)
}

// docs/authentication.md: ticket auth signs out locally.
func Example_ticketSignOut() {
	client, _ := quickbase.New("myrealm", quickbase.WithTicketAuth("user@example.com", "password"))
	defer client.SignOut()
}

// docs/xml-api.md: the XML client with schema aliases, and result helpers.
func Example_xmlAliases() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	schema := core.NewSchema().
		Table("projects", "bqxyz123").
		Field("name", 6).
		Field("status", 7).
		Table("tasks", "bqabc456").
		Field("title", 6).
		Build()
	x := xml.New(client, xml.WithSchema(core.ResolveSchema(schema)))

	count, _ := x.GetNumRecords(ctx, "projects")
	fmt.Println(count)

	record, _ := x.GetRecordInfo(ctx, "projects", 123)
	fmt.Println(record.Field("name").Value, record.FieldByID(7).Value)

	dbs, _ := x.GrantedDBs(ctx, xml.GrantedDBsOptions{})
	fmt.Println(dbs.Database("projects").Name)

	dtm, _ := x.GetAppDTMInfo(ctx, "bqxyz123")
	fmt.Println(dtm.Table("tasks").LastModifiedTime)

	sch, _ := x.GetSchema(ctx, "projects")
	fmt.Println(sch.Field("name"), sch.ChildTable("tasks"))

	users, _ := x.UserRoles(ctx, "bqxyz123")
	for _, u := range users.Users {
		fmt.Println(u.Name, u.Roles)
	}
	_ = xml.IsUnauthorized(nil) || xml.IsNotFound(nil)
}

// docs/queries.md: streaming a large table a page at a time.
func Example_iterator() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()
	body := quickbase.RunQueryBody{From: "bqtable1", Select: quickbase.Ints(3, 6)}

	for record, err := range client.RunQueryIterator(ctx, body) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(record["6"].Value)
	}
}

// docs/schema-aliases.md: records keyed by alias.
func Example_recordsByAlias() {
	schema := quickbase.NewSchema().Table("projects", "bqw3ryzab").Field("name", 6).Build()
	client, _ := quickbase.New("myrealm",
		quickbase.WithUserToken("token"),
		quickbase.WithSchemaOptions(schema, quickbase.SchemaOptions{TransformResponses: true}),
	)
	records, _ := client.Query("projects").Select("name").Run(context.Background())
	for _, rec := range records {
		fmt.Println(rec["name"])
	}
}
