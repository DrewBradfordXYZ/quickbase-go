// CLI Schema Generator
//
// Generates a schema definition from a QuickBase application.
//
// Usage:
//
//	go run ./cmd/schema -r <realm> -a <appId> -t <token>
//
// Options:
//
//	-r, --realm    QuickBase realm (e.g., "mycompany")
//	-a, --app      Application ID (e.g., "bqw123abc")
//	-t, --token    User token for authentication (or set QB_USER_TOKEN env var)
//	-o, --output   Output file path (default: stdout)
//	-f, --format   Output format: "go" or "json" (default: "go")
//	-m, --merge    Merge with existing schema file, preserving custom aliases
//	-h, --help     Show help
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/DrewBradfordXYZ/quickbase-go"
)

func main() {
	// Define flags
	var (
		realm  string
		app    string
		token  string
		output string
		format string
		merge  bool
		help   bool
	)

	flag.StringVar(&realm, "r", "", "QuickBase realm (required)")
	flag.StringVar(&realm, "realm", "", "QuickBase realm (required)")
	flag.StringVar(&app, "a", "", "Application ID (required)")
	flag.StringVar(&app, "app", "", "Application ID (required)")
	flag.StringVar(&token, "t", "", "User token (or set QB_USER_TOKEN env var)")
	flag.StringVar(&token, "token", "", "User token (or set QB_USER_TOKEN env var)")
	flag.StringVar(&output, "o", "", "Output file path (default: stdout)")
	flag.StringVar(&output, "output", "", "Output file path (default: stdout)")
	flag.StringVar(&format, "f", "go", "Output format: go or json (default: go)")
	flag.StringVar(&format, "format", "go", "Output format: go or json (default: go)")
	flag.BoolVar(&merge, "m", false, "Merge with existing schema, preserving custom aliases")
	flag.BoolVar(&merge, "merge", false, "Merge with existing schema, preserving custom aliases")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.BoolVar(&help, "help", false, "Show help")

	flag.Usage = showHelp
	flag.Parse()

	if help {
		showHelp()
		os.Exit(0)
	}

	// Use env var if token not provided
	if token == "" {
		token = os.Getenv("QB_USER_TOKEN")
	}

	// Validate required options
	if realm == "" {
		fmt.Fprintln(os.Stderr, "Error: --realm is required")
		os.Exit(1)
	}
	if app == "" {
		fmt.Fprintln(os.Stderr, "Error: --app is required")
		os.Exit(1)
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: --token is required (or set QB_USER_TOKEN env var)")
		os.Exit(1)
	}

	// Merge requires an output file
	if merge && output == "" {
		fmt.Fprintln(os.Stderr, "Error: --merge requires --output to specify the file to merge with")
		os.Exit(1)
	}

	// Fetch schema
	fmt.Fprintf(os.Stderr, "Fetching schema from %s/%s...\n", realm, app)

	schema, err := fetchSchema(realm, app, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Count tables and fields
	tableCount := len(schema.Tables)
	fieldCount := 0
	for _, t := range schema.Tables {
		fieldCount += len(t.Fields)
	}
	fmt.Fprintf(os.Stderr, "Found %d tables with %d fields\n", tableCount, fieldCount)

	// Merge with existing schema if requested
	if merge && output != "" {
		existing, err := loadExistingSchema(output, format)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
		if existing != nil {
			merged, stats := mergeSchemas(existing, schema)
			schema = merged

			fmt.Fprintln(os.Stderr, "Merge complete:")
			fmt.Fprintf(os.Stderr, "  Tables: %d preserved, %d added, %d removed\n",
				stats.TablesPreserved, stats.TablesAdded, stats.TablesRemoved)
			fmt.Fprintf(os.Stderr, "  Fields: %d preserved, %d added, %d removed\n",
				stats.FieldsPreserved, stats.FieldsAdded, stats.FieldsRemoved)
		} else {
			fmt.Fprintf(os.Stderr, "No existing schema found at %s, creating new file\n", output)
		}
	}

	// Format output
	var result string
	switch format {
	case "json":
		result = formatAsJSON(schema)
	case "go":
		result = formatAsGo(schema)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown format %q (use 'go' or 'json')\n", format)
		os.Exit(1)
	}

	// Write output
	if output != "" {
		if err := os.WriteFile(output, []byte(result), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Schema written to %s\n", output)
	} else {
		fmt.Print(result)
	}
}

func showHelp() {
	fmt.Println(`quickbase-go schema - Generate schema from QuickBase application

Usage:
  go run ./cmd/schema [options]

Options:
  -r, --realm <realm>   QuickBase realm (required, e.g., "mycompany")
  -a, --app <appId>     Application ID (required, e.g., "bqw123abc")
  -t, --token <token>   User token (or set QB_USER_TOKEN env var)
  -o, --output <file>   Output file path (default: stdout)
  -f, --format <type>   Output format: "go" or "json" (default: "go")
  -m, --merge           Merge with existing schema, preserving custom aliases
  -h, --help            Show this help message

Examples:
  # Generate Go schema to stdout
  go run ./cmd/schema -r mycompany -a bqw123abc -t your-token

  # Generate and save to file
  go run ./cmd/schema -r mycompany -a bqw123abc -o schema.go

  # Generate JSON format
  go run ./cmd/schema -r mycompany -a bqw123abc -f json -o schema.json

  # Update existing schema with new fields (preserves custom aliases)
  go run ./cmd/schema -r mycompany -a bqw123abc -o schema.go --merge

  # Using environment variable for token
  QB_USER_TOKEN=your-token go run ./cmd/schema -r mycompany -a bqw123abc

  # Or with env vars for all options
  go run ./cmd/schema -r "$QB_REALM" -a "$QB_APP_ID" -t "$QB_USER_TOKEN"`)
}

func fetchSchema(realm, appID, token string) (*quickbase.Schema, error) {
	client, err := quickbase.New(realm, quickbase.WithUserToken(token))
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Fetch all tables
	tablesResp, err := client.API().GetAppTablesWithResponse(ctx, &quickbase.GetAppTablesParams{
		AppId: appID,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching tables: %w", err)
	}
	if tablesResp.JSON200 == nil {
		return nil, fmt.Errorf("fetching tables: status %d", tablesResp.StatusCode())
	}

	schema := &quickbase.Schema{
		Tables: make(map[string]quickbase.TableSchema),
	}

	tableAliases := make(map[string]bool)

	for _, table := range *tablesResp.JSON200 {
		if table.Id == nil || table.Name == nil {
			continue
		}

		tableID := *table.Id
		tableName := *table.Name

		// Generate table alias from name
		tableAlias := labelToAlias(tableName)
		tableAlias = makeUnique(tableAlias, tableAliases)

		// Fetch fields for this table
		fieldsResp, err := client.API().GetFieldsWithResponse(ctx, &quickbase.GetFieldsParams{
			TableId: tableID,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching fields for table %s: %w", tableID, err)
		}
		if fieldsResp.JSON200 == nil {
			return nil, fmt.Errorf("fetching fields for table %s: status %d", tableID, fieldsResp.StatusCode())
		}

		fieldAliases := make(map[string]bool)
		fieldMap := make(map[string]int)

		for _, field := range *fieldsResp.JSON200 {
			if field.Label == nil {
				continue
			}

			// Generate field alias from label
			alias := labelToAlias(*field.Label)
			alias = makeUnique(alias, fieldAliases)

			fieldMap[alias] = int(field.Id)
		}

		schema.Tables[tableAlias] = quickbase.TableSchema{
			ID:     tableID,
			Fields: fieldMap,
		}
	}

	return schema, nil
}

// labelToAlias converts a label to a camelCase alias
func labelToAlias(label string) string {
	// Remove non-alphanumeric chars except spaces
	re := regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	cleaned := strings.TrimSpace(re.ReplaceAllString(label, ""))

	if cleaned == "" {
		return "field"
	}

	// Split by spaces and convert to camelCase
	words := strings.Fields(cleaned)
	var result strings.Builder

	for i, word := range words {
		lower := strings.ToLower(word)
		if i == 0 {
			result.WriteString(lower)
		} else {
			// Capitalize first letter
			for j, r := range lower {
				if j == 0 {
					result.WriteRune(unicode.ToUpper(r))
				} else {
					result.WriteRune(r)
				}
			}
		}
	}

	return result.String()
}

// makeUnique appends a number suffix if alias already exists
func makeUnique(alias string, existing map[string]bool) string {
	if !existing[alias] {
		existing[alias] = true
		return alias
	}

	counter := 2
	for existing[fmt.Sprintf("%s%d", alias, counter)] {
		counter++
	}
	unique := fmt.Sprintf("%s%d", alias, counter)
	existing[unique] = true
	return unique
}

// loadExistingSchema loads an existing schema from a file
func loadExistingSchema(filePath, format string) (*quickbase.Schema, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if format == "json" {
		var schema quickbase.Schema
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, fmt.Errorf("parsing JSON schema: %w", err)
		}
		return &schema, nil
	}

	// Parse Go format - extract the schema using regex
	// Look for table entries like: "alias": { ID: "tableId", ...
	schema := &quickbase.Schema{
		Tables: make(map[string]quickbase.TableSchema),
	}

	content := string(data)

	// Match table entries: "tableAlias": {\n\t\t\tID: "tableId",
	tablePattern := regexp.MustCompile(`"([^"]+)":\s*\{\s*\n\s*ID:\s*"([^"]+)"`)
	tableMatches := tablePattern.FindAllStringSubmatch(content, -1)

	for _, match := range tableMatches {
		if len(match) < 3 {
			continue
		}
		tableAlias := match[1]
		tableID := match[2]

		// Find the fields section for this table
		// Look for content between "tableAlias": { and the closing },
		tableStart := strings.Index(content, fmt.Sprintf(`"%s": {`, tableAlias))
		if tableStart == -1 {
			continue
		}

		// Find Fields: map[string]int{ section
		fieldsStart := strings.Index(content[tableStart:], "Fields: map[string]int{")
		if fieldsStart == -1 {
			continue
		}
		fieldsStart += tableStart

		// Find closing brace of fields map
		fieldsEnd := strings.Index(content[fieldsStart:], "},")
		if fieldsEnd == -1 {
			continue
		}
		fieldsSection := content[fieldsStart : fieldsStart+fieldsEnd]

		// Match field entries: "fieldAlias": fieldId,
		fieldPattern := regexp.MustCompile(`"([^"]+)":\s*(\d+)`)
		fieldMatches := fieldPattern.FindAllStringSubmatch(fieldsSection, -1)

		fieldMap := make(map[string]int)
		for _, fm := range fieldMatches {
			if len(fm) < 3 {
				continue
			}
			fieldAlias := fm[1]
			var fieldID int
			fmt.Sscanf(fm[2], "%d", &fieldID)
			fieldMap[fieldAlias] = fieldID
		}

		schema.Tables[tableAlias] = quickbase.TableSchema{
			ID:     tableID,
			Fields: fieldMap,
		}
	}

	if len(schema.Tables) == 0 {
		return nil, fmt.Errorf("could not parse existing Go schema")
	}

	return schema, nil
}

// MergeStats contains statistics about the merge operation
type MergeStats struct {
	TablesAdded     int
	TablesRemoved   int
	TablesPreserved int
	FieldsAdded     int
	FieldsRemoved   int
	FieldsPreserved int
}

// mergeSchemas merges a fresh schema with an existing schema, preserving custom aliases
func mergeSchemas(existing, fresh *quickbase.Schema) (*quickbase.Schema, MergeStats) {
	stats := MergeStats{}
	merged := &quickbase.Schema{
		Tables: make(map[string]quickbase.TableSchema),
	}

	// Build reverse lookup for existing schema (ID -> alias)
	existingTableIDToAlias := make(map[string]string)
	existingFieldIDToAlias := make(map[string]map[int]string) // tableID -> (fieldID -> alias)

	for alias, table := range existing.Tables {
		existingTableIDToAlias[table.ID] = alias
		existingFieldIDToAlias[table.ID] = make(map[int]string)
		for fieldAlias, fieldID := range table.Fields {
			existingFieldIDToAlias[table.ID][fieldID] = fieldAlias
		}
	}

	// Track which existing tables we've seen (by ID)
	seenTableIDs := make(map[string]bool)

	// Process each table from fresh schema
	for freshTableAlias, freshTable := range fresh.Tables {
		tableID := freshTable.ID
		seenTableIDs[tableID] = true

		// Use existing alias if available, otherwise use fresh alias
		tableAlias := freshTableAlias
		if existingAlias, ok := existingTableIDToAlias[tableID]; ok {
			tableAlias = existingAlias
		}

		existingFieldMap := existingFieldIDToAlias[tableID]

		// Track which existing fields we've seen (by ID)
		seenFieldIDs := make(map[int]bool)
		mergedFields := make(map[string]int)

		// Process each field from fresh schema
		for freshFieldAlias, fieldID := range freshTable.Fields {
			seenFieldIDs[fieldID] = true

			// Use existing alias if available, otherwise use fresh alias
			fieldAlias := freshFieldAlias
			if existingFieldMap != nil {
				if existingAlias, ok := existingFieldMap[fieldID]; ok {
					fieldAlias = existingAlias
				}
			}
			mergedFields[fieldAlias] = fieldID

			if existingFieldMap != nil && existingFieldMap[fieldID] != "" {
				stats.FieldsPreserved++
			} else {
				stats.FieldsAdded++
			}
		}

		// Check for removed fields (in existing but not in fresh)
		if existingFieldMap != nil {
			for fieldID := range existingFieldMap {
				if !seenFieldIDs[fieldID] {
					stats.FieldsRemoved++
				}
			}
		}

		merged.Tables[tableAlias] = quickbase.TableSchema{
			ID:     tableID,
			Fields: mergedFields,
		}

		if _, ok := existingTableIDToAlias[tableID]; ok {
			stats.TablesPreserved++
		} else {
			stats.TablesAdded++
		}
	}

	// Check for removed tables (in existing but not in fresh)
	for _, table := range existing.Tables {
		if !seenTableIDs[table.ID] {
			stats.TablesRemoved++
		}
	}

	return merged, stats
}

func formatAsGo(schema *quickbase.Schema) string {
	var b strings.Builder

	b.WriteString("// QuickBase Schema Definition\n")
	b.WriteString("// Generated by: go run ./cmd/schema\n")
	b.WriteString(fmt.Sprintf("// Generated at: %s\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("\n")
	b.WriteString("package main\n")
	b.WriteString("\n")
	b.WriteString("import \"github.com/DrewBradfordXYZ/quickbase-go\"\n")
	b.WriteString("\n")
	b.WriteString("var schema = &quickbase.Schema{\n")
	b.WriteString("\tTables: map[string]quickbase.TableSchema{\n")

	// Sort table keys for consistent output
	tableAliases := sortedKeys(schema.Tables)

	for _, tableAlias := range tableAliases {
		table := schema.Tables[tableAlias]

		b.WriteString(fmt.Sprintf("\t\t%q: {\n", tableAlias))
		b.WriteString(fmt.Sprintf("\t\t\tID: %q,\n", table.ID))
		b.WriteString("\t\t\tFields: map[string]int{\n")

		// Sort field keys for consistent output
		fieldAliases := sortedFieldKeys(table.Fields)

		for _, fieldAlias := range fieldAliases {
			fieldID := table.Fields[fieldAlias]
			b.WriteString(fmt.Sprintf("\t\t\t\t%q: %d,\n", fieldAlias, fieldID))
		}

		b.WriteString("\t\t\t},\n")
		b.WriteString("\t\t},\n")
	}

	b.WriteString("\t},\n")
	b.WriteString("}\n")

	return b.String()
}

func formatAsJSON(schema *quickbase.Schema) string {
	data, _ := json.MarshalIndent(schema, "", "  ")
	return string(data) + "\n"
}

// sortedKeys returns map keys sorted alphabetically
func sortedKeys(m map[string]quickbase.TableSchema) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple bubble sort for small maps
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func sortedFieldKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple bubble sort for small maps
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
