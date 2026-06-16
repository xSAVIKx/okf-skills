// Package main implements the MySQL OKF (Open Knowledge Format) connector.
// It retrieves schemas and table/column comments from a MySQL database,
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

	_ "github.com/go-sql-driver/mysql"
	"github.com/xSAVIKx/okf-skills/okf-go"
)

// ColumnSpec represents the schema properties of a MySQL table column.
type ColumnSpec struct {
	Name     string // Column name
	Type     string // Column data type
	Key      string // Column key index type (e.g., PRI, MUL, UNI)
	Nullable bool   // Is column nullable
	Default  string // Column default value
	Extra    string // Extra column specifiers (e.g., auto_increment)
	Comment  string // Column comment/description
}

// main is the CLI entrypoint for MySQL connector.
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
	fmt.Println("Usage: okf-mysql <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create OKF bundle from MySQL database")
	fmt.Println("  ingest   - Sync OKF bundle comments back into MySQL database")
}

// runProduce implements the 'produce' subcommand, querying database metadata and comments
// from INFORMATION_SCHEMA tables and producing the OKF bundle.
func runProduce(args []string) {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	host := fs.String("host", "localhost", "MySQL host")
	port := fs.Int("port", 3306, "MySQL port")
	user := fs.String("user", "", "MySQL user (required)")
	password := fs.String("password", "", "MySQL password (required)")
	dbName := fs.String("db", "", "MySQL database schema (required)")
	outDir := fs.String("out", "", "Output bundle directory (required)")
	tablesStr := fs.String("tables", "", "Filter tables (comma-separated, optional)")
	sample := fs.Int("sample", 0, "Number of sample rows to embed per table (0 = none)")
	profile := fs.Bool("profile", false, "Compute per-column statistics and embed a Data Profile section")
	relationships := fs.Bool("relationships", false, "Extract foreign-key relationships and embed a Relationships section")
	stats := fs.Bool("stats", false, "Compute row-count and freshness statistics (Stats section)")
	fs.Parse(args)
	*password = resolvePassword(*password)

	if *user == "" || *password == "" || *dbName == "" || *outDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", *user, *password, *host, *port, *dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	var filterTables map[string]bool
	if *tablesStr != "" {
		filterTables = make(map[string]bool)
		for _, t := range strings.Split(*tablesStr, ",") {
			filterTables[strings.TrimSpace(t)] = true
		}
	}

	// 1. Get base tables and views in schema. TABLE_TYPE distinguishes a VIEW from
	// a BASE TABLE so the bundle can label each entity and emit a view definition.
	rows, err := db.Query("SELECT TABLE_NAME, TABLE_TYPE, COALESCE(TABLE_COMMENT, '') FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE IN ('BASE TABLE', 'VIEW')", *dbName)
	if err != nil {
		log.Fatalf("Failed to query tables: %v", err)
	}
	defer rows.Close()

	tablesDir := filepath.Join(*outDir, "tables")
	if err := os.MkdirAll(tablesDir, 0755); err != nil {
		log.Fatalf("Failed to create tables directory: %v", err)
	}

	type TableInfo struct {
		Name    string
		Comment string
		IsView  bool
	}
	var tables []TableInfo

	timestamp := time.Now().Format(time.RFC3339)
	today := time.Now().Format("2006-01-02")

	for rows.Next() {
		var name, tableType, comment string
		if err := rows.Scan(&name, &tableType, &comment); err != nil {
			log.Fatalf("Failed to scan table info: %v", err)
		}
		// Strip InnoDB partition comments from MySQL
		if idx := strings.Index(comment, "; InnoDB free:"); idx != -1 {
			comment = comment[:idx]
		}
		if strings.HasPrefix(comment, "InnoDB free:") {
			comment = ""
		}
		comment = strings.TrimSpace(comment)

		if filterTables == nil || filterTables[name] {
			tables = append(tables, TableInfo{Name: name, Comment: comment, IsView: tableType == "VIEW"})
		}
	}

	for _, tInfo := range tables {
		// 2. Query columns and comments
		colRows, err := db.Query(`
			SELECT COLUMN_NAME, COLUMN_TYPE, COLUMN_KEY, IS_NULLABLE, COALESCE(COLUMN_DEFAULT, ''), EXTRA, COALESCE(COLUMN_COMMENT, '')
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION`, *dbName, tInfo.Name)
		if err != nil {
			log.Fatalf("Failed to query columns for table %s: %v", tInfo.Name, err)
		}

		var cols []ColumnSpec
		for colRows.Next() {
			var colName, colType, colKey, isNullable, dflt, extra, comment string
			if err := colRows.Scan(&colName, &colType, &colKey, &isNullable, &dflt, &extra, &comment); err != nil {
				log.Fatalf("Failed to scan column: %v", err)
			}
			cols = append(cols, ColumnSpec{
				Name:     colName,
				Type:     colType,
				Key:      colKey,
				Nullable: isNullable == "YES",
				Default:  dflt,
				Extra:    extra,
				Comment:  comment,
			})
		}
		colRows.Close()

		var body bytes.Buffer
		body.WriteString("# Columns\n\n")
		body.WriteString("| Name | Type | Key | Nullable | Default | Extra | Comment |\n")
		body.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, col := range cols {
			nullStr := "No"
			if col.Nullable {
				nullStr = "Yes"
			}
			fmt.Fprintf(&body, "| %s | %s | %s | %s | %s | %s | %s |\n",
				col.Name, col.Type, col.Key, nullStr, col.Default, col.Extra, col.Comment)
		}

		bodyStr := body.String()
		if *relationships {
			fks, err := getForeignKeys(db, *dbName, tInfo.Name)
			if err != nil {
				log.Fatalf("Failed to read foreign keys for table %s: %v", tInfo.Name, err)
			}
			bodyStr = okf.AppendRelationshipsSection(bodyStr, "Relationships", foreignKeyRelationships(fks))
		}
		// Constraints & indexes are cheap catalog reads, emitted by default.
		indexes, err := getIndexes(db, *dbName, tInfo.Name)
		if err != nil {
			log.Fatalf("Failed to read indexes for %s: %v", tInfo.Name, err)
		}
		cons, err := getConstraints(db, *dbName, tInfo.Name)
		if err != nil {
			log.Fatalf("Failed to read constraints for %s: %v", tInfo.Name, err)
		}
		if s := okf.RenderConstraintsSection(cons); s != "" {
			bodyStr = okf.UpsertSection(bodyStr, "Constraints", s)
		}
		if s := okf.RenderIndexesSection(indexes); s != "" {
			bodyStr = okf.UpsertSection(bodyStr, "Indexes", s)
		}
		if tInfo.IsView {
			viewSQL, err := getViewDefinition(db, *dbName, tInfo.Name)
			if err != nil {
				log.Fatalf("Failed to read view definition for %s: %v", tInfo.Name, err)
			}
			if s := okf.RenderViewDefinition(viewSQL); s != "" {
				bodyStr = okf.UpsertSection(bodyStr, "View Definition", s)
			}
		}
		if *profile {
			profiles, err := profileTable(db, tInfo.Name, cols)
			if err != nil {
				log.Fatalf("Failed to profile table %s: %v", tInfo.Name, err)
			}
			bodyStr = okf.UpsertSection(bodyStr, "Data Profile", okf.RenderProfileSection(profiles))
		}
		if *sample > 0 {
			headers, sampleRows, err := sampleTable(db, tInfo.Name, *sample)
			if err != nil {
				log.Fatalf("Failed to sample table %s: %v", tInfo.Name, err)
			}
			bodyStr = okf.UpsertSection(bodyStr, "Sample", okf.RenderSampleSection(headers, sampleRows))
		}
		if *stats {
			ts, err := getTableStats(db, tInfo.Name, cols)
			if err != nil {
				log.Fatalf("Failed to compute stats for %s: %v", tInfo.Name, err)
			}
			if s := okf.RenderStatsSection(ts); s != "" {
				bodyStr = okf.UpsertSection(bodyStr, "Stats", s)
			}
		}

		conceptType := "MySQL Table"
		kindTag := "table"
		if tInfo.IsView {
			conceptType = "MySQL View"
			kindTag = "view"
		}
		// Preserve comment-driven descriptions: a table/view COMMENT, when present,
		// remains the description; otherwise fall back to a typed default.
		description := tInfo.Comment
		if description == "" {
			description = fmt.Sprintf("MySQL %s %s", kindTag, tInfo.Name)
		}
		fresh := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
				Type:        conceptType,
				Title:       tInfo.Name,
				Description: description,
				Resource:    fmt.Sprintf("mysql://%s:%d/%s/%s", *host, *port, *dbName, tInfo.Name),
				Tags:        []string{"mysql", kindTag},
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

	// Produce index.md listing all tables
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# Database Schema: %s\n\n", *dbName)
	indexBody.WriteString("This OKF bundle represents the tables and comments extracted from MySQL.\n\n")
	indexBody.WriteString("## Tables\n\n")
	for _, tInfo := range tables {
		kind := "table"
		if tInfo.IsView {
			kind = "view"
		}
		desc := tInfo.Comment
		if desc == "" {
			desc = fmt.Sprintf("MySQL %s %s", kind, tInfo.Name)
		}
		fmt.Fprintf(&indexBody, "- [%s](tables/%s.md) - %s\n", tInfo.Name, tInfo.Name, desc)
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
// comparing comments, and executing MODIFY COLUMN queries to sync descriptions back to MySQL.
func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	host := fs.String("host", "localhost", "MySQL host")
	port := fs.Int("port", 3306, "MySQL port")
	user := fs.String("user", "", "MySQL user (required)")
	password := fs.String("password", "", "MySQL password (required)")
	dbName := fs.String("db", "", "MySQL database schema (required)")
	bundleDir := fs.String("bundle", "", "OKF bundle path (required)")
	sync := fs.Bool("sync", false, "Write modifications back to MySQL (optional)")
	fs.Parse(args)
	*password = resolvePassword(*password)

	if *user == "" || *password == "" || *dbName == "" || *bundleDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", *user, *password, *host, *port, *dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
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

		// Check if table exists in MySQL database
		var dbTableComment string
		err = db.QueryRow(`
			SELECT COALESCE(TABLE_COMMENT, '') 
			FROM INFORMATION_SCHEMA.TABLES 
			WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, *dbName, tableName).Scan(&dbTableComment)

		if err == sql.ErrNoRows {
			fmt.Printf("Table '%s' does not exist in MySQL database.\n", tableName)
			continue
		} else if err != nil {
			log.Fatalf("Failed to check table %s: %v", tableName, err)
		}

		// Strip InnoDB partition comments
		if idx := strings.Index(dbTableComment, "; InnoDB free:"); idx != -1 {
			dbTableComment = dbTableComment[:idx]
		}
		dbTableComment = strings.TrimSpace(dbTableComment)

		// 1. Sync Table Comment
		okfTableComment := strings.TrimSpace(doc.Frontmatter.Description)
		if okfTableComment != dbTableComment {
			fmt.Printf("Table '%s' comment mismatch:\n  OKF: %q\n  DB:  %q\n", tableName, okfTableComment, dbTableComment)
			if *sync {
				query := fmt.Sprintf("ALTER TABLE `%s` COMMENT = '%s'", tableName, escapeString(okfTableComment))
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
			var dbColType, dbIsNullable, dbDflt, dbExtra, dbComment string
			err = db.QueryRow(`
				SELECT COLUMN_TYPE, IS_NULLABLE, COALESCE(COLUMN_DEFAULT, ''), EXTRA, COALESCE(COLUMN_COMMENT, '')
				FROM INFORMATION_SCHEMA.COLUMNS
				WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
				*dbName, tableName, col.Name).Scan(&dbColType, &dbIsNullable, &dbDflt, &dbExtra, &dbComment)

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
					// We must recreate the exact column specification and append the new comment
					nullSpec := "NULL"
					if dbIsNullable == "NO" {
						nullSpec = "NOT NULL"
					}
					dfltSpec := ""
					if dbDflt != "" && dbDflt != "NULL" {
						dfltSpec = "DEFAULT " + dbDflt
					}
					query := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s %s %s %s COMMENT '%s'",
						tableName, col.Name, dbColType, nullSpec, dfltSpec, dbExtra, escapeString(okfComment))

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
		if len(parts) < 8 {
			continue
		}

		// Parse row: | Name | Type | Key | Nullable | Default | Extra | Comment |
		cols = append(cols, ColumnSpec{
			Name:     strings.TrimSpace(parts[1]),
			Type:     strings.TrimSpace(parts[2]),
			Key:      strings.TrimSpace(parts[3]),
			Nullable: strings.TrimSpace(strings.ToLower(parts[4])) == "yes",
			Default:  strings.TrimSpace(parts[5]),
			Extra:    strings.TrimSpace(parts[6]),
			Comment:  strings.TrimSpace(parts[7]),
		})
	}
	return cols
}

// escapeString escapes single quotes and backslashes for safe inclusion in SQL queries.
func escapeString(val string) string {
	val = strings.ReplaceAll(val, "\\", "\\\\")
	val = strings.ReplaceAll(val, "'", "''")
	return val
}
