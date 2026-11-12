# Schema aliases

A schema gives tables and fields readable names. With one, the client translates names to IDs in requests (table, `Select`, `Where`, `SortBy`, `GroupBy` and upsert data), and you can still use IDs anywhere.

## Defining a schema

```go
schema := quickbase.NewSchema().
	Table("projects", "bqw3ryzab").
	Field("id", 3).
	Field("name", 6).
	Field("status", 7).
	Table("tasks", "bqw4xyzcd").
	Field("title", 6).
	Field("projectId", 8).
	Build()

client, _ := quickbase.New("myrealm",
	quickbase.WithUserToken("token"),
	quickbase.WithSchema(schema),
)
```

Or as a struct:

```go
schema := &quickbase.Schema{
	Tables: map[string]quickbase.TableSchema{
		"projects": {ID: "bqw3ryzab", Fields: map[string]int{"id": 3, "name": 6, "status": 7}},
	},
}
```

## Using aliases

Aliases work in any request: the client resolves them in the HTTP layer, in a body's `from`, `to`, `select`, `sortBy`, `groupBy`, `where`, record data, `fieldsToReturn` and `mergeFieldId`, and in a `TableId` parameter. A table ID the schema doesn't name goes as given.

```go
active, _ := client.RunQueryAll(ctx, quickbase.RunQueryBody{
	From:   "projects",
	Select: quickbase.Fields(schema, "projects", "name", "status"),
	Where:  quickbase.Where("{'status'.EX.'Active'}"),
})

client.Upsert(ctx, quickbase.UpsertBody{
	To:   "projects",
	Data: []quickbase.Record{quickbase.Row("name", "Alpha", "status", "Active")},
})
```

Results key values by field ID (`rec["6"]`). To have `RunQueryAll`, `RunQueryN`, `RunQueryIterator` and the report equivalents key records by alias (`rec["name"]`), turn on `TransformResponses`:

```go
client, _ := quickbase.New("myrealm",
	quickbase.WithUserToken("token"),
	quickbase.WithSchemaOptions(schema, quickbase.SchemaOptions{TransformResponses: true}),
)
records, _ := client.RunQueryAll(ctx, quickbase.RunQueryBody{From: "projects", Select: quickbase.Fields(schema, "projects", "name")})
fmt.Println(records[0]["name"].Value)
```

Fields the schema doesn't name keep their ID. `Select` is a list of field IDs, so `quickbase.Fields(schema, "projects", "name", "status")` resolves names into it; `From`, `To` and `Where` take aliases directly.

A name the schema doesn't have is an error that lists the names it does, and suggests the closest: `... Did you mean 'status'?`

## Generating a schema from an app

```bash
go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN" -o schema.go
go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN" -f json -o schema.json
```

It names tables and fields from their labels. Rename any alias you like, then regenerate with `--merge` (`-m`) when the app changes: aliases you set are kept (matched by ID), new tables and fields are added, and it reports what changed.

```bash
go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN" -o schema.go --merge
```

## Loading a schema from JSON

```go
data, err := os.ReadFile("schema.json")
if err != nil {
	log.Fatal(err)
}
var schema quickbase.Schema
if err := json.Unmarshal(data, &schema); err != nil {
	log.Fatal(err)
}
client, _ := quickbase.New("myrealm", quickbase.WithUserToken("token"), quickbase.WithSchema(&schema))
```

```json
{
  "tables": {
    "projects": { "id": "bqw3ryzab", "fields": { "id": 3, "name": 6, "status": 7 } }
  }
}
```

## The XML API

`xml.New(client, xml.WithSchema(core.ResolveSchema(schema)))` gives the XML client the same aliases. See [xml-api.md](xml-api.md).
