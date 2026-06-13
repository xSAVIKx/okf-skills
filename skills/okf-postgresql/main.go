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
	"gopkg.in/yaml.v3"
)

// Frontmatter represents the YAML metadata block at the top of an OKF concept document.
type Frontmatter struct {
	Type        string    `yaml:"type"`                  // Concept kind (e.g. PostgreSQL Table)
	Title       string    `yaml:"title,omitempty"`       // Table name
	Description string    `yaml:"description,omitempty"` // Table comment description
	Resource    string    `yaml:"resource,omitempty"`    // Canonical database URI for the table
	Tags        []string  `yaml:"tags,omitempty"`        // Tags for classification
	Timestamp   string    `yaml:"timestamp,omitempty"`   // Timestamp of extraction
}

// ConceptDoc represents a parsed or constructed OKF markdown document.
type ConceptDoc struct {
	Frontmatter Frontmatter
	Body        string
}

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
	fs.Parse(args)

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

	// 1. Get base tables in PostgreSQL schema and their comments
	rows, err := db.Query(`
		SELECT
			c.relname AS table_name,
			COALESCE(obj_description(c.oid, 'pg_class'), '') AS table_comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r' AND n.nspname = $1`, *schemaName)
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
	}
	var tables []TableInfo

	timestamp := time.Now().Format(time.RFC3339)

	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			log.Fatalf("Failed to scan table info: %v", err)
		}
		if filterTables == nil || filterTables[name] {
			tables = append(tables, TableInfo{Name: name, Comment: strings.TrimSpace(comment)})
		}
	}

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

		doc := ConceptDoc{
			Frontmatter: Frontmatter{
				Type:        "PostgreSQL Table",
				Title:       tInfo.Name,
				Description: tInfo.Comment,
				Resource:    fmt.Sprintf("postgres://%s:%d/%s/%s/%s", *host, *port, *dbName, *schemaName, tInfo.Name),
				Tags:        []string{"postgres", "table"},
				Timestamp:   timestamp,
			},
			Body: body.String(),
		}

		filePath := filepath.Join(tablesDir, tInfo.Name+".md")
		if err := writeConceptDoc(filePath, doc); err != nil {
			log.Fatalf("Failed to write table %s document: %v", tInfo.Name, err)
		}
		fmt.Printf("Produced concept doc: %s\n", filePath)
	}

	// Produce index.md listing all tables
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# Database Schema: %s.%s\n\n", *dbName, *schemaName)
	indexBody.WriteString("This OKF bundle represents the tables and comments extracted from PostgreSQL.\n\n")
	indexBody.WriteString("## Tables\n\n")
	for _, tInfo := range tables {
		desc := tInfo.Comment
		if desc == "" {
			desc = "No description available"
		}
		fmt.Fprintf(&indexBody, "- [%s](tables/%s.md) - %s\n", tInfo.Name, tInfo.Name, desc)
	}

	indexDoc := ConceptDoc{
		Frontmatter: Frontmatter{
			Type:        "Dataset",
			Title:       *dbName + "." + *schemaName,
			Description: fmt.Sprintf("PostgreSQL Dataset schema for %s.%s", *dbName, *schemaName),
			Timestamp:   timestamp,
		},
		Body: indexBody.String(),
	}
	if err := writeConceptDoc(filepath.Join(*outDir, "index.md"), indexDoc); err != nil {
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
	sync := flag.Bool("sync", false, "Write modifications back to PostgreSQL (optional)")
	fs.Parse(args)

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
		doc, err := readConceptDoc(filePath)
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
				query := fmt.Sprintf("COMMENT ON TABLE %s.%s IS $1", *schemaName, tableName)
				_, err := db.Exec(query, okfTableComment)
				if err != nil {
					log.Fatalf("Failed to update comment on table %s: %v", tableName, err)
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
					query := fmt.Sprintf("COMMENT ON COLUMN %s.%s.%s IS $1", *schemaName, tableName, col.Name)
					_, err := db.Exec(query, okfComment)
					if err != nil {
						log.Fatalf("Failed to update column comment for %s.%s: %v", tableName, col.Name, err)
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

// writeConceptDoc writes the ConceptDoc structure as a markdown file with YAML frontmatter.
func writeConceptDoc(filePath string, doc ConceptDoc) error {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	fmBytes, err := yaml.Marshal(doc.Frontmatter)
	if err != nil {
		return err
	}
	buf.Write(fmBytes)
	buf.WriteString("---\n")
	buf.WriteString(doc.Body)
	return os.WriteFile(filePath, buf.Bytes(), 0644)
}

// readConceptDoc parses an OKF Markdown file.
func readConceptDoc(filePath string) (*ConceptDoc, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	parts := bytes.SplitN(content, []byte("---\n"), 3)
	if len(parts) < 3 {
		parts = bytes.SplitN(content, []byte("---\r\n"), 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid OKF concept file format: missing frontmatter boundaries")
		}
	}

	var fm Frontmatter
	if err := yaml.Unmarshal(parts[1], &fm); err != nil {
		return nil, err
	}

	return &ConceptDoc{
		Frontmatter: fm,
		Body:        string(parts[2]),
	}, nil
}
