// Package main implements the Google Cloud BigQuery OKF (Open Knowledge Format) connector.
// It retrieves dataset schemas and table/field descriptions from BigQuery,
// generating OKF bundles, and syncs OKF edits back into BigQuery metadata.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/xSAVIKx/okf-skills/okf-go"
	"google.golang.org/api/iterator"
)

// ColumnSpec represents the schema properties of a BigQuery table column/field.
type ColumnSpec struct {
	Name        string // Field name
	Type        string // Field data type
	Required    string // Field mode (Required, Nullable, or Repeated)
	Description string // Field description/comment
}

// main is the CLI entrypoint for BigQuery connector.
// version is the build version, injected via -ldflags "-X main.version=..." by
// install.sh; it defaults to "dev" for plain `go build`.
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "produce":
		runProduce(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
	case "schema":
		if err := okf.PrintSchema(os.Stdout, buildSchema()); err != nil {
			log.Fatalf("Failed to print schema: %v", err)
		}
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// printUsage outputs the available CLI commands.
func printUsage() {
	fmt.Println("Usage: okf-bigquery <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create OKF bundle from BigQuery dataset")
	fmt.Println("  ingest   - Sync OKF bundle descriptions back to BigQuery")
}

// runProduce implements the 'produce' subcommand, querying BigQuery tables and schemas
// using the Google Cloud Client API and exporting them into OKF Markdown files.
func runProduce(args []string) {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	projectID := fs.String("project", "", "GCP Project ID (required)")
	datasetID := fs.String("dataset", "", "BigQuery Dataset ID (required)")
	outDir := fs.String("out", "", "Output bundle directory (required)")
	tablesStr := fs.String("tables", "", "Filter tables (comma-separated, optional)")
	sample := fs.Int("sample", 0, "Number of sample rows to embed per table (0 = none)")
	profile := fs.Bool("profile", false, "Compute per-column statistics and embed a Data Profile section")
	relationships := fs.Bool("relationships", false, "Extract foreign-key constraints into a Relationships section")
	stats := fs.Bool("stats", false, "Compute row-count and freshness statistics (Stats section)")
	fs.Parse(args)

	if *projectID == "" || *datasetID == "" || *outDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, *projectID)
	if err != nil {
		log.Fatalf("Failed to initialize BigQuery client: %v", err)
	}
	defer client.Close()

	var filterTables map[string]bool
	if *tablesStr != "" {
		filterTables = make(map[string]bool)
		for _, t := range strings.Split(*tablesStr, ",") {
			filterTables[strings.TrimSpace(t)] = true
		}
	}

	tablesDir := filepath.Join(*outDir, "tables")
	if err := os.MkdirAll(tablesDir, 0755); err != nil {
		log.Fatalf("Failed to create tables directory: %v", err)
	}

	ds := client.Dataset(*datasetID)
	dsMeta, err := ds.Metadata(ctx)
	if err != nil {
		log.Fatalf("Failed to query dataset metadata: %v", err)
	}

	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")
	// tableEntry records each discovered entity's name and kind ("table"/"view")
	// so the index can label views distinctly from regular tables.
	type tableEntry struct {
		Name string
		Kind string
	}
	var tables []tableEntry

	// List tables in dataset
	it := ds.Tables(ctx)
	for {
		t, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Table list iterator error: %v", err)
		}

		if filterTables != nil && !filterTables[t.TableID] {
			continue
		}

		tMeta, err := t.Metadata(ctx)
		if err != nil {
			log.Fatalf("Failed to get table metadata for %s: %v", t.TableID, err)
		}

		// View awareness: BigQuery's table metadata already tells us whether this
		// is a logical view (Type == ViewTable) and, if so, holds the defining SQL
		// in ViewQuery — no extra INFORMATION_SCHEMA round-trip is needed.
		isView := tMeta.Type == bigquery.ViewTable

		kind := "table"
		if isView {
			kind = "view"
		}
		tables = append(tables, tableEntry{Name: t.TableID, Kind: kind})

		var body bytes.Buffer
		body.WriteString("# Columns\n\n")
		body.WriteString("| Name | Type | Required | Description |\n")
		body.WriteString("| --- | --- | --- | --- |\n")
		for _, field := range tMeta.Schema {
			reqStr := "Nullable"
			if field.Required {
				reqStr = "Required"
			} else if field.Repeated {
				reqStr = "Repeated"
			}
			fmt.Fprintf(&body, "| %s | %s | %s | %s |\n",
				field.Name, field.Type, reqStr, field.Description)
		}

		bodyStr := body.String()
		if *relationships {
			fks, err := getForeignKeys(ctx, client, *projectID, *datasetID, t.TableID)
			if err != nil {
				log.Fatalf("Failed to read foreign keys for table %s: %v", t.TableID, err)
			}
			bodyStr = okf.AppendRelationshipsSection(bodyStr, "Relationships", foreignKeyRelationships(fks))
		}
		// Constraints are a cheap catalog read (UNIQUE / PRIMARY KEY informational
		// constraints). BigQuery has no CHECK constraints and no secondary indexes,
		// so there is no Indexes section.
		cons, err := getConstraints(ctx, client, *projectID, *datasetID, t.TableID)
		if err != nil {
			log.Fatalf("Failed to read constraints for table %s: %v", t.TableID, err)
		}
		if s := okf.RenderConstraintsSection(cons); s != "" {
			bodyStr = okf.UpsertSection(bodyStr, "Constraints", s)
		}
		if isView {
			if s := okf.RenderViewDefinition(tMeta.ViewQuery); s != "" {
				bodyStr = okf.UpsertSection(bodyStr, "View Definition", s)
			}
		}
		if *profile {
			profiles, err := profileTable(ctx, client, *projectID, *datasetID, t.TableID, tMeta.Schema)
			if err != nil {
				log.Fatalf("Failed to profile table %s: %v", t.TableID, err)
			}
			bodyStr = okf.UpsertSection(bodyStr, "Data Profile", okf.RenderProfileSection(profiles))
		}
		if *sample > 0 {
			headers, sampleRows, err := sampleTable(ctx, client, *projectID, *datasetID, t.TableID, *sample, tMeta.Schema)
			if err != nil {
				log.Fatalf("Failed to sample table %s: %v", t.TableID, err)
			}
			bodyStr = okf.UpsertSection(bodyStr, "Sample", okf.RenderSampleSection(headers, sampleRows))
		}
		if *stats {
			ts, err := getTableStats(ctx, client, *projectID, *datasetID, t.TableID, tMeta.NumRows, tMeta.Schema)
			if err != nil {
				log.Fatalf("Failed to compute stats for %s: %v", t.TableID, err)
			}
			if s := okf.RenderStatsSection(ts); s != "" {
				bodyStr = okf.UpsertSection(bodyStr, "Stats", s)
			}
		}

		conceptType := "BigQuery Table"
		kindTag := "table"
		if isView {
			conceptType = "BigQuery View"
			kindTag = "view"
		}
		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        conceptType,
				Title:       t.TableID,
				Description: tMeta.Description,
				Resource:    fmt.Sprintf("bigquery://%s/%s/%s", *projectID, *datasetID, t.TableID),
				Tags:        []string{"bigquery", kindTag},
				Timestamp:   timestamp,
			},
			Body: bodyStr,
		}

		filePath := filepath.Join(tablesDir, t.TableID+".md")
		// Incremental produce: preserve an unchanged concept byte-for-byte (keeping
		// any enriched description/body), rewrite only when the structure changed.
		var existing *okf.ConceptDoc
		if e, err := okf.ReadConceptDoc(filePath); err == nil {
			existing = e
		}
		merged, changed := okf.MergeConcept(existing, fresh)
		if !changed {
			fmt.Printf("Unchanged, preserved: %s\n", filePath)
			continue
		}
		if err := okf.WriteConceptDoc(filePath, merged); err != nil {
			log.Fatalf("Failed to write table %s document: %v", t.TableID, err)
		}
		kind, action := "Update", "Structure changed for"
		if existing == nil {
			kind, action = "Creation", "Established"
		}
		if err := okf.AppendLogEntry(*outDir, today, kind, fmt.Sprintf("%s [%s](/tables/%s.md).", action, t.TableID, t.TableID)); err != nil {
			log.Fatalf("Failed to append log entry: %v", err)
		}
		fmt.Printf("Produced concept doc: %s\n", filePath)
	}

	// Produce index.md listing all dataset tables
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# BigQuery Dataset: %s.%s\n\n", *projectID, *datasetID)
	if dsMeta.Description != "" {
		fmt.Fprintf(&indexBody, "%s\n\n", dsMeta.Description)
	} else {
		indexBody.WriteString("This OKF bundle represents the tables and schema descriptions extracted from BigQuery.\n\n")
	}
	indexBody.WriteString("## Tables\n\n")
	for _, table := range tables {
		fmt.Fprintf(&indexBody, "- [%s](tables/%s.md) - BigQuery %s %s\n", table.Name, table.Name, table.Kind, table.Name)
	}

	indexDoc := okf.ConceptDoc{
		Frontmatter: okf.Frontmatter{
			OKFVersion: "0.1",
		},
		Body: indexBody.String(),
	}
	if err := okf.WriteConceptDoc(filepath.Join(*outDir, "index.md"), indexDoc); err != nil {
		log.Fatalf("Failed to write index.md: %v", err)
	}
	fmt.Println("Produced index.md successfully.")
}

// runIngest implements the 'ingest' subcommand, parsing OKF bundles,
// comparing descriptions, and calling the BigQuery update_table APIs to sync comments.
func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	projectID := fs.String("project", "", "GCP Project ID (required)")
	datasetID := fs.String("dataset", "", "BigQuery Dataset ID (required)")
	bundleDir := fs.String("bundle", "", "OKF bundle path (required)")
	sync := fs.Bool("sync", false, "Write descriptions back to BigQuery (optional)")
	fs.Parse(args)

	if *projectID == "" || *datasetID == "" || *bundleDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, *projectID)
	if err != nil {
		log.Fatalf("Failed to initialize BigQuery client: %v", err)
	}
	defer client.Close()

	tablesDir := filepath.Join(*bundleDir, "tables")
	if _, err := os.Stat(tablesDir); os.IsNotExist(err) {
		log.Fatalf("Tables directory not found in bundle: %s", tablesDir)
	}

	files, err := os.ReadDir(tablesDir)
	if err != nil {
		log.Fatalf("Failed to read tables: %v", err)
	}

	ds := client.Dataset(*datasetID)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(tablesDir, file.Name())
		doc, err := okf.ReadConceptDoc(filePath)
		if err != nil {
			log.Fatalf("Failed to read concept doc %s: %v", filePath, err)
		}

		tableName := doc.Frontmatter.Title
		if tableName == "" {
			tableName = strings.TrimSuffix(file.Name(), ".md")
		}

		t := ds.Table(tableName)
		tMeta, err := t.Metadata(ctx)
		if err != nil {
			fmt.Printf("Table '%s' does not exist in BigQuery dataset.\n", tableName)
			continue
		}

		var update bigquery.TableMetadataToUpdate
		var newSchema bigquery.Schema
		metadataUpdated := false
		schemaUpdated := false

		// 1. Sync Table Description
		okfTableDesc := strings.TrimSpace(doc.Frontmatter.Description)
		if okfTableDesc != tMeta.Description {
			fmt.Printf("Table '%s' description mismatch:\n  OKF: %q\n  BQ:  %q\n", tableName, okfTableDesc, tMeta.Description)
			update.Description = okfTableDesc
			metadataUpdated = true
		}

		// 2. Sync Field Descriptions
		parsedCols := parseColumnsFromMarkdown(doc.Body)
		parsedColsMap := make(map[string]string)
		for _, col := range parsedCols {
			parsedColsMap[strings.ToLower(col.Name)] = col.Description
		}

		for _, field := range tMeta.Schema {
			okfDesc, found := parsedColsMap[strings.ToLower(field.Name)]
			if found && okfDesc != field.Description {
				fmt.Printf("Table '%s' field '%s' description mismatch:\n  OKF: %q\n  BQ:  %q\n", tableName, field.Name, okfDesc, field.Description)
				field.Description = okfDesc
				schemaUpdated = true
			}
			newSchema = append(newSchema, field)
		}

		if schemaUpdated {
			update.Schema = newSchema
			metadataUpdated = true
		}

		if metadataUpdated {
			if *sync {
				_, err := t.Update(ctx, update, tMeta.ETag)
				if err != nil {
					log.Fatalf("Failed to update BigQuery table %s metadata: %v", tableName, err)
				}
				fmt.Printf("  -> Successfully updated metadata in BigQuery for table '%s'.\n", tableName)
			}
		}
	}
	fmt.Println("OKF bundle ingestion / BigQuery sync finished.")
}

// parseColumnsFromMarkdown parses the columns markdown table.
func parseColumnsFromMarkdown(body string) []ColumnSpec {
	// The schema table lives under the "# Columns" heading. A bundle produced with
	// --profile/--sample also embeds "## Data Profile" / "## Sample" tables, whose
	// rows must not be parsed as schema columns. Isolate the Columns section first;
	// fall back to the whole body for bundles without that heading.
	if section, ok := okf.GetSectionAny(body, "Columns"); ok {
		body = section
	}
	var cols []ColumnSpec
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		if strings.Contains(line, "---") || strings.Contains(strings.ToLower(line), "type") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		// Parse row: | Name | Type | Required | Description |
		cols = append(cols, ColumnSpec{
			Name:        strings.TrimSpace(parts[1]),
			Type:        strings.TrimSpace(parts[2]),
			Required:    strings.TrimSpace(parts[3]),
			Description: strings.TrimSpace(parts[4]),
		})
	}
	return cols
}
