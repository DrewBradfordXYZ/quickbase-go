# Typed tables

`go run ./cmd/schema -f structs` turns every table of an app into Go types: a record struct, a changes struct, the field IDs, and a `quickbase.Table`. Queries return your structs; upserts take them.

```bash
go run github.com/DrewBradfordXYZ/quickbase-go/v3/cmd/schema \
	-r mycompany -a bqw123abc -t "$QB_USER_TOKEN" -f structs -o app/quickbase_gen.go
```

For each table it writes, from the field types QuickBase reports:

```go
// Payment is a record of the Payments table (bu3e4di4p).
type Payment struct {
	RecordID      int             `qb:"3"` // Record ID# (recordid)
	RecordOwner   *quickbase.User `qb:"4"` // Record Owner (user)
	PaymentAmount *float64        `qb:"6"` // Payment Amount (currency)
	PaymentDate   quickbase.Date  `qb:"7"` // Payment Date (date)
	PaymentStatus string          `qb:"9"` // Payment Status (text-multiple-choice)
	UnitNumber    string          `qb:"14"` // Unit Number (text, lookup)
}

// PaymentChanges sets fields of a Payments record in an upsert: a nil field isn't sent.
type PaymentChanges struct {
	RecordID      *int            `qb:"3"`
	PaymentAmount *float64        `qb:"6"`
	PaymentDate   *quickbase.Date `qb:"7"`
	PaymentStatus *string         `qb:"9"`
}

var PaymentFields = struct{ RecordID, PaymentAmount, PaymentDate, PaymentStatus, UnitNumber int }{3, 6, 7, 9, 14}

var Payments = quickbase.NewTable[Payment, PaymentChanges]("bu3e4di4p")
```

## Using them

```go
paid, err := app.Payments.Query(ctx, qb, quickbase.TableQuery{
	Where:  fmt.Sprintf("{%d.EX.'Paid'}", app.PaymentFields.PaymentStatus),
	SortBy: []quickbase.SortField{quickbase.Sort(app.PaymentFields.PaymentDate, quickbase.ASC)},
	Limit:  100,
})
for _, p := range paid {
	fmt.Println(p.RecordID, p.PaymentDate, p.PaymentStatus)
}

one, err := app.Payments.Get(ctx, qb, 1) // by Record ID#; *NotFoundError if there's none

// Add a record: only the fields set are sent
created, err := app.Payments.Upsert(ctx, qb, app.PaymentChanges{
	PaymentAmount: quickbase.Ptr(42.5),
	PaymentDate:   &quickbase.Date{Year: 2024, Month: time.February, Day: 29},
})
id := created.Metadata.CreatedRecordIds[0]

// Change one field; the others keep their values
_, err = app.Payments.Upsert(ctx, qb, app.PaymentChanges{RecordID: &id, PaymentStatus: quickbase.Ptr("Paid")})

n, err := app.Payments.Delete(ctx, qb, fmt.Sprintf("{3.EX.%d}", id)) // an empty query is refused
```

`Limit` stops at that many records (and asks QuickBase for no more). `Iterate` yields records a page at a time, for a table too large to hold in memory. A query selects the record struct's fields; a smaller struct with the same tags selects fewer:

```go
type PaymentSummary struct {
	Amount *float64 `qb:"6"`
}
summaries, err := quickbase.NewTable[PaymentSummary, app.PaymentChanges]("bu3e4di4p").Query(ctx, qb, quickbase.TableQuery{})
```

## Types

| QuickBase field | Record struct | Changes struct |
|---|---|---|
| text, multi-line, multiple choice, rich text, phone, email, URL, time of day | `string` | `*string` |
| numeric, currency, percent, rating, duration (ms) | `*float64` (nil when blank) | `*float64` |
| Record ID# | `int` | `*int` |
| checkbox | `bool` | `*bool` |
| multi-select text | `[]string` | `*[]string` |
| date | `quickbase.Date` | `*quickbase.Date` |
| date/time | `quickbase.DateTime` | `*quickbase.DateTime` |
| user | `*quickbase.User` | `*string`: the email address |
| list-user | `[]quickbase.User` | `*[]string`: email addresses |
| address, file attachment, ... | `any` | `*any` |

A blank value reads as the zero value, except numbers, which are nil, so a blank differs from 0. The changes struct leaves out what QuickBase won't let an upsert set: formula, lookup and summary fields, report links, and the built-in fields Date Created, Date Modified, Record Owner and Last Modified By. A user field is set by email address: QuickBase also takes `{"id": ...}`, but a bare user ID stores a wrong user without an error.

## Regenerating

Rename types and fields freely, then regenerate with `--merge` when the app changes: it keeps your names, matched by table and field ID, adds new tables and fields, and refreshes types. The field IDs variable follows the record type's name (`Payment` → `PaymentFields`).

`-p` names the package; it defaults to the output file's directory.
