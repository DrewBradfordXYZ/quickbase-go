package integration

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/DrewBradfordXYZ/quickbase-go/v3"
)

// Records written and read back live decode into a struct by alias: a
// schema built from the test table, records keyed by alias
// (TransformResponses), qb tags naming the aliases.
func TestDecodeRecordsLive(t *testing.T) {
	ctx := context.Background()
	tc := getTestContext(t)

	schema := quickbase.NewSchema().
		Table("records", tc.TableID).
		Field("id", 3).
		Field("name", tc.TextFieldID).
		Field("amount", tc.NumberFieldID).
		Field("due", tc.DateFieldID).
		Field("active", tc.CheckboxFieldID).
		Build()
	qb, err := quickbase.New(qbRealm, quickbase.WithUserToken(qbUserToken),
		quickbase.WithSchemaOptions(schema, quickbase.SchemaOptions{TransformResponses: true}))
	if err != nil {
		t.Fatal(err)
	}

	due := quickbase.Date{Year: 2024, Month: time.February, Day: 29}
	created, err := qb.Upsert(ctx, quickbase.UpsertBody{
		To: "records",
		Data: []quickbase.Record{
			quickbase.Row("name", "Decoded", "amount", 1234.5, "due", due, "active", true),
			quickbase.Row("name", "Blank"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := created.Metadata.CreatedRecordIds
	if len(ids) != 2 {
		t.Fatalf("created %v", ids)
	}

	records, err := qb.RunQueryAll(ctx, quickbase.RunQueryBody{
		From:   "records",
		Select: quickbase.Fields(schema, "records", "id", "name", "amount", "due", "active"),
		Where:  "{3.EX." + strconv.Itoa(ids[0]) + "}OR{3.EX." + strconv.Itoa(ids[1]) + "}",
		SortBy: quickbase.SortBy{quickbase.Asc(3)},
	})
	if err != nil {
		t.Fatal(err)
	}

	type row struct {
		ID     int            `qb:"id"`
		Name   string         `qb:"name"`
		Amount *float64       `qb:"amount"`
		Due    quickbase.Date `qb:"due"`
		Active bool           `qb:"active"`
	}
	rows, err := quickbase.DecodeRecords[row](records)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("decoded %d rows", len(rows))
	}
	got := rows[0]
	if got.ID != ids[0] || got.Name != "Decoded" || got.Amount == nil || *got.Amount != 1234.5 || got.Due != due || !got.Active {
		t.Errorf("decoded %+v", got)
	}
	blank := rows[1]
	if blank.Name != "Blank" || blank.Amount != nil || blank.Due != (quickbase.Date{}) || blank.Active {
		t.Errorf("blank record decoded as %+v", blank)
	}
}
