# Architecture (v3)

Decided 2026-11-12, for v3.

## The rule

**One method per operation. Request struct in, response struct out. Iterators for lists. Settings on the client.**

```go
app, err := qb.GetApp(ctx, appID)
fmt.Println(app.Name, app.Created) // plain fields

fields, err := qb.GetFields(ctx, quickbase.GetFieldsParams{TableId: tableID})

for record, err := range qb.RunQueryIterator(ctx, quickbase.RunQueryBody{From: tableID}) {
	...
}
```

This is the shape most Go SDKs share, and it fits Go: struct literals name their fields, so a call reads like named arguments without a builder; an error comes back from the call that caused it, not from a `Run()` at the end of a chain; and there is one way to call each operation, so the docs, examples and users agree.

## Why v2 changed

v2 grew five ways to make a call: the generated client (`API()`), fluent builders (33 of them, each turning its setters into a map, then JSON, then the generated body type), result wrappers with an accessor per field (8,658 generated lines), body-based helpers (`RunQuery`, `RunQueryAll`), and a `Query()` builder. Each was added to make the one below it easier to use. The root cause was the generated types: every optional field a pointer, responses as anonymous structs, and record values as a union type with `From…`/`As…` methods. The wrappers hid those, and every gap in them (enums, nested structs, required fields) sent callers back to `if x.Foo != nil && x.Foo.Bar != nil`.

v3 fixes the types where they are made, and drops the layers that hid them.

## How

**The types, at the source.** The shared spec stays language-neutral. `cmd/gospec` adapts it for Go before code generation (`make generate`):

- An operation's inline response object becomes a named type, `<Operation>Result` (`GetAppResult`), so methods return a type callers can name.
- An optional field of a type used only in responses is a plain value, not a pointer (`x-go-type-skip-optional-pointer`): a missing field reads as its zero value. Types used in requests keep pointers, where "not set" must differ from zero (a `false` that turns a setting off).
- A record field's value is `any` (`x-go-type: any`): text, numbers, lists and objects decode into plain Go values.

This needs oapi-codegen v2.8.2 (v2.4.0 generates code that doesn't compile with plain optional fields on types that keep unlisted keys). The generated code needs oapi-codegen's runtime v1.8.0, which needs Go 1.24, as before; the generator itself needs Go 1.25 to run, which `go run` fetches.

**The methods.** `cmd/generate-methods` writes one method on `Client` per operation, from the generated client's signatures:

- Path parameters are arguments; query parameters are one params struct; a JSON body is the generated body type. A body that isn't JSON (QBL) is `[]byte`, and so is a response that isn't JSON (QBL, file content).
- It returns the success type, or the error QuickBase returned (`*ValidationError`, `*NotFoundError`, ...).
- A 207 (QuickBase applied part of the request) returns the result; `WithLineErrorsAsErrors` makes an upsert's rejected values an error too.

`API()` stays, for the raw generated client and its responses.

**Aliases, once.** With a schema, table and field aliases are resolved in the HTTP layer, for every method: a table in a path (`/tables/{tableId}`, `/files/{tableId}`) or a `tableId` parameter; and, for record operations (query, upsert, delete records, modified since, run formula), the body's `from`, `to`, `select`, `sortBy`, `groupBy`, `where`, record data, `fieldsToReturn` and `mergeFieldId`. Other bodies are left alone (`transferUserToken`'s `from` and `to` are user IDs). A table reference the schema doesn't name goes as given, unless it is a near miss of an alias, which is an error suggesting it: telling IDs from aliases by shape rejected real IDs. No method resolves aliases on its own.

**Records, once.** A record is `map[string]any`. One decoder turns QuickBase's records into these, and into the typed tables' structs (next).

## Order

1. Types at the source, generated methods, aliases in the HTTP layer; remove the builders, result wrappers and union helpers.
2. One record decoder.
3. Typed tables: generated structs per table, `Table[Record, Changes]`.
4. Remove what is left of the old layers (`Query()`, unused `core` transforms); generate the root package's re-exports.
5. Migrate basetypes.

## Not chosen

- **Keeping the builders and wrappers, and filling their gaps.** Each gap was another generator feature, and the layers would remain: five ways to call, and two types per response.
- **Plain fields in request types too.** A request needs "not set" (omitted) to differ from zero; a `false` or `0` has to reach QuickBase when you send it.
- **Changing the shared spec.** Other languages generate from it, and v2's pinned generator can't compile the extensions.
- **A different generator.** oapi-codegen with these extensions gives the types we want; the problem was how they were used, not the tool.
