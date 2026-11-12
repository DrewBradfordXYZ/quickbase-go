# Queries and records

## RunQuery, RunQueryAll, RunQueryN

A query is a `RunQueryBody`. Tables and fields can be IDs, or aliases when the client has a [schema](schema-aliases.md).

```go
records, _ := client.RunQueryN(ctx, quickbase.RunQueryBody{
	From:   "bqtable1",
	Select: quickbase.Ints(3, 6, 7),
	Where:  quickbase.Where("{7.EX.'Active'}"),
	SortBy: quickbase.SortBy(quickbase.Sort(6, quickbase.ASC), quickbase.Sort(3, quickbase.DESC)),
}, 1000) // stop at 1,000 records
```

| Method | Returns |
|---|---|
| `RunQuery(ctx, body)` | One page, with `Data`, `Fields` and `Metadata` |
| `RunQueryAll(ctx, body)` | Every page's records |
| `RunQueryN(ctx, body, n)` | Up to `n` records |
| `RunQueryIterator(ctx, body)` | Each record, a page fetched at a time |

```go
page, _ := client.RunQuery(ctx, quickbase.RunQueryBody{
	From:    "bqtable1",
	Select:  quickbase.Ints(3, 6),
	Options: &quickbase.QueryOptions{Top: quickbase.Ptr(50), Skip: quickbase.Ptr(100)},
})
fmt.Println(page.Metadata.TotalRecords)
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

QuickBase sizes pages by record size, often about 100 records.

## Records

A `Record` is what QuickBase sends: field ID (or alias, with `TransformResponses`) to `{Value}`, where `Value` is a plain Go value (`string`, `float64`, `bool`, `[]any`, `map[string]any`, `nil`).

```go
rec["6"].Value   // one value
rec.Values()     // map[string]any
```

`Decode` fills a struct by `qb` tags (the field ID, or an alias), converting each value to the field's Go type. A blank value leaves the zero value, so use a pointer where blank must differ from zero:

```go
type Project struct {
	ID     int                `qb:"3"`
	Name   string             `qb:"6"`
	Due    quickbase.Date     `qb:"7"`  // Date field
	Closed quickbase.DateTime `qb:"8"`  // Date/Time field
	Budget *float64           `qb:"9"`  // nil when blank
	Owner  *quickbase.User    `qb:"4"`
}

var p Project
err := rec.Decode(&p)

projects, err := quickbase.DecodeRecords[Project](records) // a slice at once
```

To send a struct, `quickbase.RecordFrom(v)` builds a record from its tags; a nil pointer field is left out, so a struct of pointers says exactly which fields to change.

## Other lists

`GetRelationshipsIterator(ctx, tableID)` and `GetUsersIterator(ctx, params, body)` yield every relationship or user, fetching a page at a time; `GetRelationships` and `GetUsers` return one page.

## Reports

A report runs with its own filter, sort and columns:

```go
reportRows, _ := client.RunReportAll(ctx, "7", "bqtable1") // report ID, table ID
fmt.Println(len(reportRows), reportRows[0].Values())
```

`RunReportN(ctx, reportID, tableID, n)` stops at `n` records, `RunReportIterator` streams them, and `client.RunReport(ctx, reportID, quickbase.RunReportParams{TableId: table})` returns one page.

## Upserts

`Upsert` adds records, or updates the record whose merge field matches. Without `MergeFieldId`, QuickBase matches on the table's key field (usually Record ID#, field 3); `MergeFieldId` can name any unique field instead.

```go
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
```

`Row` takes field/value pairs: field IDs, or aliases with a schema. Or build a record yourself, field ID to `{Value}`:

```go
quickbase.Record{"6": {Value: "text"}, "8": {Value: []string{"a", "b"}}}
```

When QuickBase rejects a value (a date in the wrong form, text in a numeric field), it still saves the record, with that field blank, and answers 207. `Upsert` returns the result, whose `Metadata.LineErrors` lists the rejections by record position; with `WithLineErrorsAsErrors()` it also returns them as a `*LineErrorsError`.

### Dates

QuickBase has two kinds of date field, and neither accepts the other's form: a Date field takes only `"2024-01-15"`, a Date/Time field only a timestamp (RFC 3339). A value in the wrong form doesn't fail the upsert; QuickBase saves the record with that field blank. So a date says which it is:

```go
quickbase.Row(
	7, quickbase.DateOf(due),            // Date field: due's day, in due's location
	8, quickbase.DateTimeOf(time.Now()), // Date/Time field: sent in UTC
)
```

A bare `time.Time` could be either: `Value` panics on one, and `Upsert` returns a `PlainTimeError` for one in its data, before sending. QuickBase returns dates as strings, and records keep them as strings; `ParseDate` and `ParseDateTime` read them. `Date{Year, Month, Day}` is a day with no time zone, and `d.In(loc)` is its midnight in `loc`.

## Deletes

```go
deleted, _ := client.DeleteRecords(ctx, quickbase.DeleteRecordsBody{From: "bqtable1", Where: quickbase.DeleteWhere("{3.EX.123}")})
fmt.Println(deleted.NumberDeleted)
```

## Helpers

| Helper | Returns | For |
|---|---|---|
| `Row(field, value, ...)` | `Record` | Upsert data |
| `Value(v)` | `FieldValue` | A single record value (refuses a bare `time.Time`) |
| `DateOf(t)`, `DateTimeOf(t)` | `Date`, `DateTime` | Date and Date/Time field values |
| `Ints(3, 6, 7)` | `*[]int` | `Select` |
| `Fields(schema, table, aliases...)` | `*[]int` | `Select` from aliases |
| `Where(query)` | `*WhereUnion` | `RunQueryBody.Where` |
| `DeleteWhere(query)` | `DeleteWhereUnion` | `DeleteRecordsBody.Where` |
| `SortBy(Sort(field, ASC), ...)` | `*SortByUnion` | `RunQueryBody.SortBy` |
| `GroupBy(fields...)` | `*[]GroupByItem` | `RunQueryBody.GroupBy` |
| `Options(top, skip)` | `*QueryOptions` | `RunQueryBody.Options` |
| `Ptr(v)` | `*T` | Optional request fields |
| `Deref(p)`, `DerefOr(p, def)` | `T` | Reading optional request fields |
| `rec.Values()` | `map[string]any` | A record as field ID to value |
| `rec.Decode(&v)`, `DecodeRecords[T](records)` | | Records into structs with `qb` tags |
| `RecordFrom(v)` | `Record` | A struct with `qb` tags into a record to send |

Query strings use QuickBase's syntax: `{field.OPERATOR.'value'}`, joined with `AND` and `OR`. See QuickBase's [query components](https://help.quickbase.com/docs/components-of-a-query).
