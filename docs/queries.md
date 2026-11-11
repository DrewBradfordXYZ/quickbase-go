# Queries and records

## The query builder

`client.Query(table)` builds a `RunQuery` request. Tables and fields can be IDs, or aliases when the client has a [schema](schema-aliases.md).

```go
records, _ := client.Query("bqtable1").
	Select(3, 6, 7).
	Where("{7.EX.'Active'}").
	SortBy(quickbase.Asc(6), quickbase.Desc(3)).
	GroupBy(7).
	RunN(ctx, 1000) // stop at 1,000 records
```

| Method | Does |
|---|---|
| `Select(fields...)` | Fields to return |
| `Where(query)` | A QuickBase query string, e.g. `{7.EX.'Active'}` |
| `SortBy(quickbase.Asc(f), quickbase.Desc(f))` | Sort order |
| `GroupBy(fields...)` | Grouping |
| `Top(n)`, `Skip(n)`, `Options(top, skip)` | Page size and offset |
| `Run(ctx)` | Every page, as `[]Record` (field ID → value) |
| `RunN(ctx, n)` | Up to `n` records |
| `RunRaw(ctx)` | One page, as the full result with metadata |

```go
page, _ := client.Query("bqtable1").Select(3, 6).Top(50).Skip(100).RunRaw(ctx)
fmt.Println(page.Metadata().TotalRecords())
```

## RunQuery, RunQueryAll, RunQueryN

The same request as a struct, for when you build it programmatically:

```go
body := quickbase.RunQueryBody{
	From:   "bqtable1",
	Select: quickbase.Ints(3, 6, 7),
	Where:  quickbase.Where("{6.GT.100}"),
	SortBy: quickbase.SortBy(quickbase.Sort(6, quickbase.ASC)),
}

page, _ := client.RunQuery(ctx, body) // one page
fmt.Printf("%d of %d records\n", page.Metadata().NumRecords(), page.Metadata().TotalRecords())
for _, rec := range page.Records() {
	fmt.Println(rec["6"])
}

all, _ := client.RunQueryAll(ctx, body)      // every page
first, _ := client.RunQueryN(ctx, body, 500) // up to 500
```

To process a large table without holding it all in memory, iterate: each page is fetched as the loop reaches it, and breaking out stops fetching.

```go
for record, err := range client.RunQueryIterator(ctx, body) {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(record["6"].Value)
}
```

QuickBase sizes pages by record size, often about 100 records. `RunQuery` and `RunRaw` return a single page; everything else follows the pages for you. `RunQueryAll` and `RunQueryN` return QuickBase's records (each value wrapped in `{value: ...}`); `quickbase.UnwrapRecords` turns them into plain maps, as the builder's `Run` does.

## Reports

A report runs with its own filter, sort and columns:

```go
reportRows, _ := client.RunReportAll(ctx, "7", "bqtable1") // report ID, table ID
fmt.Println(len(quickbase.UnwrapRecords(reportRows)))
```

`RunReportN(ctx, reportID, tableID, n)` stops at `n` records, `RunReportIterator` streams them, and `client.RunReport(reportID, table)` is the one-page builder.

## Upserts

`Upsert` adds records, or updates the record whose merge field matches. Without `MergeFieldId`, QuickBase matches on the table's key field (usually Record ID#, field 3); `MergeFieldId` can name any unique field instead.

```go
result, _ := client.Upsert("bqtable1").
	MergeFieldId(9). // a unique field, e.g. an external ID
	Data(
		quickbase.Row(9, "EXT-001", 6, "Alpha", 7, "Active"),
		quickbase.Row(9, "EXT-002", 6, "Beta", 7, "Pending"),
	).
	FieldsToReturn(6, 7).
	Run(ctx)
fmt.Println(result.Metadata().CreatedRecordIds(), result.Metadata().UpdatedRecordIds())
```

`Row` takes field/value pairs: field IDs, or aliases with a schema. For a value whose type you want to state, build the record yourself:

```go
quickbase.Record{"6": quickbase.Value("text"), "8": quickbase.Value([]string{"a", "b"})}
```

## Deletes

```go
deleted, _ := client.DeleteRecords("bqtable1").Where("{3.EX.123}").Run(ctx)
fmt.Println(deleted.NumberDeleted())
```

## Helpers

| Helper | Returns | For |
|---|---|---|
| `Row(field, value, ...)` | `Record` | Upsert data |
| `Value(v)` | `FieldValue` | A single record value |
| `Ints(3, 6, 7)` | `*[]int` | `Select` |
| `Fields(schema, table, aliases...)` | `*[]int` | `Select` from aliases |
| `Where(query)` | `*WhereUnion` | `RunQueryBody.Where` |
| `SortBy(Sort(field, ASC), ...)` | `*SortByUnion` | `RunQueryBody.SortBy` |
| `Asc(field)`, `Desc(field)` | `SortSpec` | The query builder's `SortBy` |
| `GroupBy(fields...)` | `*[]GroupByItem` | `RunQueryBody.GroupBy` |
| `Options(top, skip)` | `*QueryOptions` | `RunQueryBody.Options` |
| `Ptr(v)` | `*T` | Optional fields |
| `Deref(p)`, `DerefOr(p, def)` | `T` | Reading optional fields |
| `UnwrapRecords(records)` | `[]Record` | Plain values from QuickBase's records |

Query strings use QuickBase's syntax: `{field.OPERATOR.'value'}`, joined with `AND` and `OR`. See QuickBase's [query components](https://help.quickbase.com/docs/components-of-a-query).
