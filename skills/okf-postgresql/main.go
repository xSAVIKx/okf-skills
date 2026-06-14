// Package main implements the PostgreSQL OKF (Open Knowledge Format) connector.
// It retrieves schemas and table/column descriptions from a PostgreSQL database,
// generating OKF bundles, and syncs OKF edits back into database comments.
package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/xSAVIKx/okf-skills/okf-go"
)

// ColumnSpec represents the schema properties of a PostgreSQL table column.
type ColumnSpec struct {
	Name     string // Column name
	Type     string // Column data type
	Nullable bool   // Is column nullable
	Comment  string // Column comment/description
}

// main is the CLI entrypoint for PostgreSQL connector.
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
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// printUsage outputs the available CLI commands.
func printUsage() {
	fmt.Println("Usage: okf-postgresql <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create OKF bundle from PostgreSQL database")
	fmt.Println("  ingest   - Sync OKF bundle comments back into PostgreSQL database")
}

// runProduce implements the 'produce' subcommand, querying database schemas and descriptions
// using PostgreSQL system catalog queries.
func runProduce(args []string) {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	host := fs.String("host", "localhost", "PostgreSQL host")
	port := fs.Int("port", 5432, "PostgreSQL port")
	user := fs.String("user", "postgres", "PostgreSQL user")
	password := fs.String("password", "", "PostgreSQL password (required)")
	dbName := fs.String("db", "", "PostgreSQL database name (required)")
	schemaName := fs.String("schema", "public", "PostgreSQL schema name")
	outDir := fs.String("out", "", "Output bundle directory (required)")
	tablesStr := fs.String("tables", "", "Filter tables (comma-separated, optional)")
	sample := fs.Int("sample", 0, "Number of sample rows to embed per table (0 = none)")
	profile := fs.Bool("profile", false, "Compute per-column statistics and embed a Data Profile section")
	relationships := fs.Bool("relationships", false, "Extract foreign-key constraints into a Relationships section")
	stats := fs.Bool("stats", false, "Compute row-count and freshness statistics (Stats section)")
	fs.Parse(args)
	*password = resolvePassword(*password)

	if *password == "" || *dbName == "" || *outDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", *host, *port, *user, *password, *dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	var filterTables map[string]bool
	if *tablesStr != "" {
		filterTables = make(map[string]bool)
		for _, t := range strings.Split(*tablesStr, ",") {
			filterTables[strings.TrimSpace(t)] = true
		}
	}

	// 1. Get base tables and views in the PostgreSQL schema and their comments.
	tables, err := listEntities(db, *schemaName, filterTables)
	if err != nil {
		log.Fatalf("Failed to query tables/views: %v", err)
	}

	tablesDir := filepath.Join(*outDir, "tables")
	if err := os.MkdirAll(tablesDir, 0755); err != nil {
		log.Fatalf("Failed to create tables directory: %v", err)
	}

	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")

	for _, tInfo := range tables {
		// 2. Query columns and comments
		colRows, err := db.Query(`
			SELECT
				a.attname AS column_name,
				format_type(a.atttypid, a.atttypmod) AS column_type,
				CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END AS is_nullable,
				COALESCE(col_description(c.oid, a.attnum), '') AS column_comment
			FROM pg_attribute a
			JOIN pg_class c ON c.oid = a.attrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relname = $1 AND n.nspname = $2 AND a.attnum > 0 AND NOT a.attisdropped
			ORDER BY a.attnum`, tInfo.Name, *schemaName)
		if err != nil {
			log.Fatalf("Failed to query columns for table %s: %v", tInfo.Name, err)
		}

		var cols []ColumnSpec
		for colRows.Next() {
			var colName, colType, isNullable, comment string
			if err := colRows.Scan(&colName, &colType, &isNullable, &comment); err != nil {
				log.Fatalf("Failed to scan column: %v", err)
			}
			cols = append(cols, ColumnSpec{
				Name:     colName,
				Type:     colType,
				Nullable: isNullable == "YES",
				Comment:  comment,
			})
		}
		colRows.Close()

		var body bytes.Buffer
		body.WriteString("# Columns\n\n")
		body.WriteString("| Name | Type | Nullable | Comment |\n")
		body.WriteString("| --- | --- | --- | --- |\n")
		for _, col := range cols {
			nullStr := "No"
			if col.Nullable {
				nullStr = "Yes"
			}
			fmt.Fprintf(&body, "| %s | %s | %s | %s |\n", col.Name, col.Type, nullStr, col.Comment)
		}

		bodyStr := body.String()
		if *relationships {
			fks, err := getForeignKeys(db, *schemaName, tInfo.Name)
			if err != nil {
				log.Fatalf("Failed to read foreign keys for table %s: %v", tInfo.Name, err)
			}
			bodyStr = okf.AppendRelationshipsSection(bodyStr, "Relationships", foreignKeyRelationships(fks))
		}
		// Constraints & indexes are cheap catalog reads, emitted by default.
		cons, err := getConstraints(db, *schemaName, tInfo.Name)
		if err != nil {
			log.Fatalf("Failed to read constraints for %s: %v", tInfo.Name, err)
		}
		if s := okf.RenderConstraintsSection(cons); s != "" {
			bodyStr = okf.UpsertSection(bodyStr, "Constraints", s)
		}
		indexes, err := getIndexes(db, *schemaName, tInfo.Name)
		if err != nil {
			log.Fatalf("Failed to read indexes for %s: %v", tInfo.Name, err)
		}
		if s := okf.RenderIndexesSection(indexes); s != "" {
			bodyStr = okf.UpsertSection(bodyStr, "Indexes", s)
		}
		if tInfo.IsView {
			viewSQL, err := getViewDefinition(db, *schemaName, tInfo.Name)
			if err != nil {
				log.Fatalf("Failed to read view definition for %s: %v", tInfo.Name, err)
			}
			if s := okf.RenderViewDefinition(viewSQL); s != "" {
				bodyStr = okf.UpsertSection(bodyStr, "View Definition", s)
			}
		}
		if *profile {
			profiles, err := profileTable(db, *schemaName, tInfo.Name, cols)
			if err != nil {
				log.Fatalf("Failed to profile table %s: %v", tInfo.Name, err)
			}
			bodyStr = okf.UpsertSection(bodyStr, "Data Profile", okf.RenderProfileSection(profiles))
		}
		if *sample > 0 {
			headers, sampleRows, err := sampleTable(db, *schemaName, tInfo.Name, *sample)
			if err != nil {
				log.Fatalf("Failed to sample table %s: %v", tInfo.Name, err)
			}
			bodyStr = okf.UpsertSection(bodyStr, "Sample", okf.RenderSampleSection(headers, sampleRows))
		}
		if *stats {
			ts, err := getTableStats(db, *schemaName, tInfo.Name, cols)
			if err != nil {
				log.Fatalf("Failed to compute stats for %s: %v", tInfo.Name, err)
			}
			if s := okf.RenderStatsSection(ts); s != "" {
				bodyStr = okf.UpsertSection(bodyStr, "Stats", s)
			}
		}

		conceptType := "PostgreSQL Table"
		kindTag := "table"
		if tInfo.IsView {
			conceptType = "PostgreSQL View"
			kindTag = "view"
		}
		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        conceptType,
				Title:       tInfo.Name,
				Description: tInfo.Comment, // preserve the database COMMENT (obj_description) as the description
				Resource:    fmt.Sprintf("postgres://%s:%d/%s/%s/%s", *host, *port, *dbName, *schemaName, tInfo.Name),
				Tags:        []string{"postgres", kindTag},
				Timestamp:   timestamp,
			},
			Body: bodyStr,
		}

		filePath := filepath.Join(tablesDir, tInfo.Name+".md")
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
			log.Fatalf("Failed to write table %s document: %v", tInfo.Name, err)
		}
		kind, action := "Update", "Structure changed for"
		if existing == nil {
			kind, action = "Creation", "Established"
		}
		if err := okf.AppendLogEntry(*outDir, today, kind, fmt.Sprintf("%s [%s](/tables/%s.md).", action, tInfo.Name, tInfo.Name)); err != nil {
			log.Fatalf("Failed to append log entry: %v", err)
		}
		fmt.Printf("Produced concept doc: %s\n", filePath)
	}

	// Produce index.md listing all tables and views
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# Database Schema: %s.%s\n\n", *dbName, *schemaName)
	indexBody.WriteString("This OKF bundle represents the tables, views and comments extracted from PostgreSQL.\n\n")
	indexBody.WriteString("## Tables & Views\n\n")
	for _, tInfo := range tables {
		desc := tInfo.Comment
		if desc == "" {
			desc = "No description available"
		}
		kindLabel := "table"
		if tInfo.IsView {
			kindLabel = "view"
		}
		fmt.Fprintf(&indexBody, "- [%s](tables/%s.md) (%s) - %s\n", tInfo.Name, tInfo.Name, kindLabel, desc)
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
// comparing comments, and executing COMMENT ON statements to sync comments back into PostgreSQL.
func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	host := fs.String("host", "localhost", "PostgreSQL host")
	port := fs.Int("port", 5432, "PostgreSQL port")
	user := fs.String("user", "postgres", "PostgreSQL user")
	password := fs.String("password", "", "PostgreSQL password (required)")
	dbName := fs.String("db", "", "PostgreSQL database name (required)")
	schemaName := fs.String("schema", "public", "PostgreSQL schema name")
	bundleDir := fs.String("bundle", "", "OKF bundle path (required)")
	sync := fs.Bool("sync", false, "Write modifications back to PostgreSQL (optional)")
	fs.Parse(args)
	*password = resolvePassword(*password)

	if *password == "" || *dbName == "" || *bundleDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", *host, *port, *user, *password, *dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	tablesDir := filepath.Join(*bundleDir, "tables")
	if _, err := os.Stat(tablesDir); os.IsNotExist(err) {
		log.Fatalf("Tables directory not found in bundle: %s", tablesDir)
	}

	files, err := os.ReadDir(tablesDir)
	if err != nil {
		log.Fatalf("Failed to read tables: %v", err)
	}

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

		// Check if table exists in PostgreSQL database
		var dbTableComment string
		err = db.QueryRow(`
			SELECT COALESCE(obj_description(c.oid, 'pg_class'), '')
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relkind = 'r' AND n.nspname = $1 AND c.relname = $2`, *schemaName, tableName).Scan(&dbTableComment)

		if err == sql.ErrNoRows {
			fmt.Printf("Table '%s' does not exist in PostgreSQL schema '%s'.\n", tableName, *schemaName)
			continue
		} else if err != nil {
			log.Fatalf("Failed to check table %s: %v", tableName, err)
		}

		dbTableComment = strings.TrimSpace(dbTableComment)
		okfTableComment := strings.TrimSpace(doc.Frontmatter.Description)

		// 1. Sync Table Comment
		if okfTableComment != dbTableComment {
			fmt.Printf("Table '%s' comment mismatch:\n  OKF: %q\n  DB:  %q\n", tableName, okfTableComment, dbTableComment)
			if *sync {
				query := fmt.Sprintf("COMMENT ON TABLE %s.%s IS '%s'", *schemaName, tableName, escapeString(okfTableComment))
				_, err := db.Exec(query)
				if err != nil {
					log.Fatalf("Failed to update comment on table %s: %v. Query: %s", tableName, err, query)
				}
				fmt.Printf("  -> Successfully updated comment on table '%s'.\n", tableName)
			}
		}

		// 2. Sync Column Comments
		parsedCols := parseColumnsFromMarkdown(doc.Body)
		for _, col := range parsedCols {
			var dbComment string
			err = db.QueryRow(`
				SELECT COALESCE(col_description(c.oid, a.attnum), '')
				FROM pg_attribute a
				JOIN pg_class c ON c.oid = a.attrelid
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE c.relname = $1 AND n.nspname = $2 AND a.attname = $3 AND a.attnum > 0 AND NOT a.attisdropped`,
				tableName, *schemaName, col.Name).Scan(&dbComment)

			if err == sql.ErrNoRows {
				fmt.Printf("Table '%s' is missing column '%s' in DB.\n", tableName, col.Name)
				continue
			} else if err != nil {
				log.Fatalf("Query column error: %v", err)
			}

			dbComment = strings.TrimSpace(dbComment)
			okfComment := strings.TrimSpace(col.Comment)

			if okfComment != dbComment {
				fmt.Printf("Table '%s' column '%s' comment mismatch:\n  OKF: %q\n  DB:  %q\n", tableName, col.Name, okfComment, dbComment)
				if *sync {
					query := fmt.Sprintf("COMMENT ON COLUMN %s.%s.%s IS '%s'", *schemaName, tableName, col.Name, escapeString(okfComment))
					_, err := db.Exec(query)
					if err != nil {
						log.Fatalf("Failed to update column comment for %s.%s: %v. Query: %s", tableName, col.Name, err, query)
					}
					fmt.Printf("  -> Successfully updated comment on column '%s.%s'.\n", tableName, col.Name)
				}
			}
		}
	}
	fmt.Println("OKF bundle ingestion / comment sync finished.")
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

		// Parse row: | Name | Type | Nullable | Comment |
		cols = append(cols, ColumnSpec{
			Name:     strings.TrimSpace(parts[1]),
			Type:     strings.TrimSpace(parts[2]),
			Nullable: strings.TrimSpace(strings.ToLower(parts[3])) == "yes",
			Comment:  strings.TrimSpace(parts[4]),
		})
	}
	return cols
}

// escapeString escapes single quotes for safe inclusion in PostgreSQL comments.
func escapeString(val string) string {
	return strings.ReplaceAll(val, "'", "''")
}
