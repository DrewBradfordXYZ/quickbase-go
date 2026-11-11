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

```go
active, _ := client.Query("projects").
	Select("name", "status").
	Where("{'status'.EX.'Active'}").
	SortBy(quickbase.Asc("name")).
	Run(ctx)

client.Upsert("projects").
	Data(quickbase.Row("name", "Alpha", "status", "Active")).
	Run(ctx)
```

Results key values by field ID (`rec["6"]`). To have the query builder's records keyed by alias (`rec["name"]`), turn on `TransformResponses`:

```go
client, _ := quickbase.New("myrealm",
	quickbase.WithUserToken("token"),
	quickbase.WithSchemaOptions(schema, quickbase.SchemaOptions{TransformResponses: true}),
)
records, _ := client.Query("projects").Select("name").Run(ctx)
fmt.Println(records[0]["name"])
```

Fields the schema doesn't name keep their ID. With `RunQueryBody`, resolve field aliases for `Select` with `quickbase.Fields(schema, "projects", "name", "status")`; the table, `Where` and `SortBy` take aliases directly.

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
