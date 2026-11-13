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

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/auth"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/core"
	"github.com/DrewBradfordXYZ/quickbase-go/v3/xml"
)

// The README's quick start: get an app, query a table, add records.
func Example_quickStart() {
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
		Where:  "{7.EX.'Active'}",
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
}

// One method per operation: arguments for the path, a params struct for the
// query, the body type for the body. Results have plain fields.
func Example_methods() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	tables, _ := client.GetAppTables(ctx, "bqxyz123")
	for _, t := range tables {
		fmt.Println(t.Id, t.Name)
	}

	fields, _ := client.GetFields(ctx, "bqtable1", nil)
	for _, f := range fields {
		fmt.Println(f.Id, f.Label, f.FieldType)
	}

	newApp, _ := client.CreateApp(ctx, quickbase.CreateAppBody{Name: "My App", Description: quickbase.Ptr("Created via API")})
	fmt.Println(newApp.Id)

	deleted, _ := client.DeleteRecords(ctx, quickbase.DeleteRecordsBody{From: "bqtable1", Where: "{3.EX.123}"})
	fmt.Println(deleted.NumberDeleted)
}

// RunQuery returns one page; RunQueryAll and RunQueryN paginate.
func Example_runQuery() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()
	body := quickbase.RunQueryBody{
		From:   "bqtable1",
		Select: quickbase.Ints(3, 6, 7),
		Where:  "{6.GT.100}",
		SortBy: quickbase.SortBy{quickbase.Asc(6)},
	}

	page, _ := client.RunQuery(ctx, body)
	fmt.Printf("%d of %d records\n", page.Metadata.NumRecords, page.Metadata.TotalRecords)
	for _, rec := range page.Data {
		fmt.Println(rec["6"].Value)
	}

	all, _ := client.RunQueryAll(ctx, body)      // every page
	first, _ := client.RunQueryN(ctx, body, 500) // up to 500
	fmt.Println(len(all), len(first))
}

// Schema aliases: readable table and field names instead of IDs, resolved
// for every request.
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

	active, _ := client.RunQueryAll(ctx, quickbase.RunQueryBody{
		From:   "projects",
		Select: quickbase.Fields(schema, "projects", "name", "status"),
		Where:  "{'status'.EX.'Active'}",
	})
	fmt.Println(len(active))

	_, _ = client.Upsert(ctx, quickbase.UpsertBody{
		To:   "projects",
		Data: []quickbase.Record{quickbase.Row("name", "Alpha", "status", "Active")},
	})
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
	_, err := client.GetApp(context.Background(), "bqxyz123")

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

	qbl, _ := client.ExportSolution(ctx, "solution-id", nil) // the QBL document

	_, _ = client.UpdateSolution(ctx, "solution-id", qbl, nil)

	file, _ := client.DownloadFile(ctx, "bqtable1", 12, 8, 0)
	fmt.Println(len(file)) // base64 file content
}

// The generated client, for the full response (status, headers).
func Example_rawAccess() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))

	resp, _ := client.API().GetAppWithResponse(context.Background(), "bqxyz123")
	fmt.Println(resp.StatusCode(), resp.HTTPResponse.Header.Get("qb-api-ray"))
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

// docs/queries.md: query options, reports, upserts, helpers.
func Example_queriesGuide() {
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	// Fields, filter, sort and group by field ID
	records, _ := client.RunQueryN(ctx, quickbase.RunQueryBody{
		From:   "bqtable1",
		Select: quickbase.Ints(3, 6, 7),
		Where:  "{7.EX.'Active'}",
		SortBy: quickbase.SortBy{quickbase.Asc(6), quickbase.Desc(3)},
	}, 1000) // stop at 1,000 records
	fmt.Println(len(records))

	// One page, with metadata
	page, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
		From:    "bqtable1",
		Select:  quickbase.Ints(3, 6),
		Options: &quickbase.QueryOptions{Top: quickbase.Ptr(50), Skip: quickbase.Ptr(100)},
	})
	fmt.Println(page.Metadata.TotalRecords)

	// Reports run with the report's own filter, sort and columns
	reportRows, _ := client.RunReportAll(ctx, "7", "bqtable1")
	fmt.Println(len(reportRows), reportRows[0].Values())

	// Upsert: insert, or update the record whose merge field matches
	result, _ := client.Upsert(ctx, quickbase.UpsertBody{
		To:           "bqtable1",
		MergeFieldId: quickbase.Ptr(9), // a unique field, e.g. an external ID
		Data: []quickbase.Record{
			quickbase.Row(9, "EXT-001", 6, "Alpha", 7, "Active"),
			quickbase.Row(9, "EXT-002", 6, "Beta", 7, "Pending"),
		},
		FieldsToReturn: quickbase.Ints(6, 7),
	})
	fmt.Println(result.Metadata.CreatedRecordIds, result.Metadata.UpdatedRecordIds)

	// A record by hand: field ID to {value}
	_ = quickbase.Record{"6": {Value: "text"}, "8": {Value: []string{"a", "b"}}}

	// Pointers for optional request fields
	_ = quickbase.Ptr(100)
	_ = quickbase.DerefOr((*string)(nil), "default")
}

// docs/schema-aliases.md: aliases in the body of any request.
func Example_schemaAliasesBody() {
	schema := quickbase.NewSchema().Table("projects", "bqw3ryzab").Field("name", 6).Field("status", 7).Build()
	client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"), quickbase.WithSchema(schema))
	page, _ := client.RunQuery(context.Background(), quickbase.RunQueryBody{
		From:   "projects",
		Select: quickbase.Fields(schema, "projects", "name", "status"),
		Where:  "{'status'.EX.'Active'}",
	})
	for _, rec := range page.Data {
		fmt.Println(rec["6"].Value)
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
		records, err := client.RunQueryAll(r.Context(), quickbase.RunQueryBody{From: "bqtable1", Select: quickbase.Ints(3, 6)})
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
	records, _ := client.RunQueryAll(context.Background(), quickbase.RunQueryBody{
		From:   "projects",
		Select: quickbase.Fields(schema, "projects", "name"),
	})
	for _, rec := range records {
		fmt.Println(rec["name"].Value)
	}
}

// docs/queries.md, Dates: a date says which kind of field it's for.
func Example_dates() {
	due := time.Date(2024, time.January, 15, 9, 0, 0, 0, time.UTC)
	record := quickbase.Row(
		7, quickbase.DateOf(due), // Date field: due's day, in due's location
		8, quickbase.DateTimeOf(due), // Date/Time field: sent in UTC
	)
	b, _ := json.Marshal(record)
	fmt.Println(string(b))
	// Output: {"7":{"value":"2024-01-15"},"8":{"value":"2024-01-15T09:00:00Z"}}
}

// docs/queries.md, Records: values, and decoding into a struct by qb tags.
func Example_records() {
	var rec quickbase.Record
	_ = json.Unmarshal([]byte(`{"3":{"value":12},"6":{"value":"Alpha"},"7":{"value":"2024-01-15"},"9":{"value":null}}`), &rec)

	type Project struct {
		ID     int            `qb:"3"`
		Name   string         `qb:"6"`
		Due    quickbase.Date `qb:"7"` // Date field
		Budget *float64       `qb:"9"` // nil when blank
	}
	var p Project
	if err := rec.Decode(&p); err != nil {
		log.Fatal(err)
	}
	fmt.Println(p.ID, p.Name, p.Due, p.Budget == nil, rec.Values()["6"])
	// Output: 12 Alpha 2024-01-15 true Alpha
}

// docs/tables.md: typed tables. go run ./cmd/schema -f structs writes these
// types for every table of an app; here, by hand.
type Payment struct {
	RecordID      int             `qb:"3"`
	PaymentAmount *float64        `qb:"6"` // nil when blank
	PaymentDate   quickbase.Date  `qb:"7"`
	PaymentStatus string          `qb:"9"`
	RecordOwner   *quickbase.User `qb:"4"`
}

type PaymentChanges struct {
	RecordID      *int            `qb:"3"`
	PaymentAmount *float64        `qb:"6"`
	PaymentDate   *quickbase.Date `qb:"7"`
	PaymentStatus *string         `qb:"9"`
}

var PaymentFields = struct{ RecordID, PaymentAmount, PaymentDate, PaymentStatus int }{3, 6, 7, 9}

var Payments = quickbase.NewTable[Payment, PaymentChanges]("bu3e4di4p")

func Example_tables() {
	qb, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"))
	ctx := context.Background()

	paid, err := Payments.Query(ctx, qb, quickbase.TableQuery{
		Where:  fmt.Sprintf("{%d.EX.'Paid'}", PaymentFields.PaymentStatus),
		SortBy: quickbase.SortBy{quickbase.Asc(PaymentFields.PaymentDate)},
		Limit:  100,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range paid {
		fmt.Println(p.RecordID, p.PaymentDate, p.PaymentStatus)
	}

	one, _ := Payments.Get(ctx, qb, 1)
	fmt.Println(one.PaymentAmount)

	// Add a record: only the fields set are sent
	created, _ := Payments.Upsert(ctx, qb, PaymentChanges{
		PaymentAmount: quickbase.Ptr(42.5),
		PaymentDate:   &quickbase.Date{Year: 2024, Month: time.February, Day: 29},
	})
	id := created.Metadata.CreatedRecordIds[0]

	// Change one field of it; the others keep their values
	_, _ = Payments.Upsert(ctx, qb, PaymentChanges{RecordID: &id, PaymentStatus: quickbase.Ptr("Paid")})

	_, _ = Payments.Delete(ctx, qb, fmt.Sprintf("{3.EX.%d}", id))
}
