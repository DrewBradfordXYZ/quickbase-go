# Migrating from v2 to v3

v3 has one way to call each operation: a method that takes a request struct and returns a plain result. The builders, the result wrappers with their accessor methods, and the `Query()` builder are gone. Why: [docs/architecture.md](docs/architecture.md).

## Import path

```go
import "github.com/DrewBradfordXYZ/quickbase-go/v3"
```

## Calls

Path parameters are arguments, query parameters a params struct, a body its request type. `ctx` comes first.

| v2 | v3 |
|---|---|
| `client.GetApp(appID).Run(ctx)` | `client.GetApp(ctx, appID)` |
| `client.GetAppTables().AppId(appID).Run(ctx)` | `client.GetAppTables(ctx, quickbase.GetAppTablesParams{AppId: appID})` |
| `client.GetFields(tableID).Run(ctx)` | `client.GetFields(ctx, quickbase.GetFieldsParams{TableId: tableID})` |
| `client.CreateApp().Name("A").Description("B").Run(ctx)` | `client.CreateApp(ctx, quickbase.CreateAppBody{Name: "A", Description: quickbase.Ptr("B")})` |
| `client.DeleteRecords(t).Where(q).Run(ctx)` | `client.DeleteRecords(ctx, quickbase.DeleteRecordsBody{From: t, Where: quickbase.DeleteWhere(q)})` |
| `client.Upsert(t).Data(rows...).MergeFieldId(9).Run(ctx)` | `client.Upsert(ctx, quickbase.UpsertBody{To: t, Data: rows, MergeFieldId: quickbase.Ptr(9)})` |
| `client.GetRelationships(t).Run(ctx)` | `client.GetRelationships(ctx, t, quickbase.GetRelationshipsParams{})` |
| `client.ExportSolution(id).Run(ctx)` then `.Body` | `client.ExportSolution(ctx, id, quickbase.ExportSolutionParams{})` returns `[]byte` |
| `client.UpdateSolution(id).Body(qbl).Run(ctx)` | `client.UpdateSolution(ctx, id, quickbase.UpdateSolutionParams{}, qbl)` |
| `client.DownloadFile(t, r, f, v).Run(ctx)` then `.Body` | `client.DownloadFile(ctx, t, r, f, v)` returns `[]byte` |

The method names and parameter types match the generated client's (`client.API()`), which v3 keeps.

## Results

Results are the generated types with plain fields. Accessor methods become fields, and `Raw()` is gone: a field QuickBase didn't send is its zero value.

| v2 | v3 |
|---|---|
| `app.Name()` | `app.Name` |
| `t.Id()`, `f.Label()`, `f.FieldType()` | `t.Id`, `f.Label`, `f.FieldType` |
| `result.Metadata().CreatedRecordIds()` | `result.Metadata.CreatedRecordIds` |
| `deleted.NumberDeleted()` | `deleted.NumberDeleted` |
| `resp.Relationships()` | `resp.Relationships` |
| `if x.Foo != nil && *x.Foo == ...` | `if x.Foo == ...` |
| `app.Raw()` (status, headers) | `client.API().GetAppWithResponse(ctx, appID)` |

Request types keep pointers for optional fields, so a `false` or `0` you set reaches QuickBase: `quickbase.Ptr(v)`.

## Queries

`Query()` is gone; `RunQueryAll`, `RunQueryN` and `RunQueryIterator` take a `RunQueryBody`.

```go
// v2
records, _ := client.Query("projects").Select("name").Where("{'status'.EX.'Active'}").Run(ctx)

// v3
records, _ := client.RunQueryAll(ctx, quickbase.RunQueryBody{
	From:   "projects",
	Select: quickbase.Fields(schema, "projects", "name"),
	Where:  quickbase.Where("{'status'.EX.'Active'}"),
})
```

`RunQuery` returns one page: `page.Data`, `page.Metadata.TotalRecords`; `page.Records()` becomes `quickbase.UnwrapRecords(page.Data)`. `TransformResponses` now keys the paging helpers' records by alias (it keyed `Query()`'s).

## Record values

A record value is a plain Go value (`string`, `float64`, `bool`, `[]any`, `map[string]any`, `nil`), not a union type.

| v2 | v3 |
|---|---|
| `rec["6"].Value.AsFieldValueValue0()` | `rec["6"].Value.(string)` |
| `fv.FromFieldValueValue1(42)` | `quickbase.FieldValue{Value: 42}` |
| `quickbase.Value(x)` | still works (and refuses a bare `time.Time`), or `{Value: x}` |

## Schema aliases

Aliases now work in every request, resolved in one place: bodies (`From`, `To`, `Where`, record data, `FieldsToReturn`, `MergeFieldId`) and a `TableId` parameter. A table ID the schema doesn't name goes as given; it was an error.

## Removed

- Every builder (`GetAppBuilder`, ...) and result wrapper (`AppResult`, `FieldsItem`, ...); `Asc`, `Desc` and `SortSpec` (use `Sort(id, quickbase.ASC)`)
- `Query()`, `QueryBuilder`
- `FieldValueUnion` and the `From…`/`As…` union helpers for record values
