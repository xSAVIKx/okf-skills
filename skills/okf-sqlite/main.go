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

	_ "modernc.org/sqlite"
	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	Type        string    `yaml:"type"`
	Title       string    `yaml:"title,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Resource    string    `yaml:"resource,omitempty"`
	Tags        []string  `yaml:"tags,omitempty"`
	Timestamp   string    `yaml:"timestamp,omitempty"`
}

type ConceptDoc struct {
	Frontmatter Frontmatter
	Body        string
}

type Column struct {
	Name       string
	Type       string
	PrimaryKey bool
	Nullable   bool
	Default    string
}

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

func printUsage() {
	fmt.Println("Usage: okf-sqlite <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create OKF bundle from SQLite database")
	fmt.Println("  ingest   - Verify or sync OKF bundle schema into SQLite database")
	fmt.Println("\nRun 'okf-sqlite <command> -h' for command-specific options.")
}

func runProduce(args []string) {
	fs := flag.NewFlagSet("produce", flag.ExitOnError)
	dbPath := fs.String("db", "", "Path to SQLite database file (required)")
	outDir := fs.String("out", "", "Path to output OKF bundle directory (required)")
	tablesStr := fs.String("tables", "", "Comma-separated list of tables to extract (optional)")
	fs.Parse(args)

	if *dbPath == "" || *outDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Parse tables filter
	var filterTables map[string]bool
	if *tablesStr != "" {
		filterTables = make(map[string]bool)
		for _, t := range strings.Split(*tablesStr, ",") {
			filterTables[strings.TrimSpace(t)] = true
		}
	}

	// 1. Get all tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		log.Fatalf("Failed to query tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatalf("Scan table name error: %v", err)
		}
		if filterTables == nil || filterTables[name] {
			tables = append(tables, name)
		}
	}

	// Create output directories
	tablesDir := filepath.Join(*outDir, "tables")
	if err := os.MkdirAll(tablesDir, 0755); err != nil {
		log.Fatalf("Failed to create tables directory: %v", err)
	}

	absDbPath, _ := filepath.Abs(*dbPath)
	timestamp := time.Now().Format(time.RFC3339)

	// 2. Generate concept docs for each table
	for _, table := range tables {
		cols, err := getTableColumns(db, table)
		if err != nil {
			log.Fatalf("Failed to get columns for table %s: %v", table, err)
		}

		var body bytes.Buffer
		body.WriteString("# Columns\n\n")
		body.WriteString("| Name | Type | Primary Key | Nullable | Default |\n")
		body.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, col := range cols {
			pkStr := "No"
			if col.PrimaryKey {
				pkStr = "Yes"
			}
			nullStr := "No"
			if col.Nullable {
				nullStr = "Yes"
			}
			fmt.Fprintf(&body, "| %s | %s | %s | %s | %s |\n", col.Name, col.Type, pkStr, nullStr, col.Default)
		}

		doc := ConceptDoc{
			Frontmatter: Frontmatter{
				Type:        "SQLite Table",
				Title:       table,
				Description: fmt.Sprintf("SQLite table %s", table),
				Resource:    fmt.Sprintf("sqlite:///%s/%s", filepath.ToSlash(absDbPath), table),
				Tags:        []string{"sqlite", "table"},
				Timestamp:   timestamp,
			},
			Body: body.String(),
		}

		filePath := filepath.Join(tablesDir, table+".md")
		if err := writeConceptDoc(filePath, doc); err != nil {
			log.Fatalf("Failed to write concept doc for %s: %v", table, err)
		}
		fmt.Printf("Produced concept doc: %s\n", filePath)
	}

	// 3. Generate index.md at root
	var indexBody bytes.Buffer
	indexBody.WriteString("# E-commerce Database Schema (SQLite)\n\n")
	indexBody.WriteString("This OKF bundle represents the tables extracted from SQLite.\n\n")
	indexBody.WriteString("## Tables\n\n")
	for _, table := range tables {
		fmt.Fprintf(&indexBody, "- [%s](tables/%s.md) - SQLite table %s\n", table, table, table)
	}

	indexDoc := ConceptDoc{
		Frontmatter: Frontmatter{
			Type:        "Dataset",
			Title:       filepath.Base(*dbPath),
			Description: fmt.Sprintf("SQLite Dataset: %s", filepath.Base(*dbPath)),
			Timestamp:   timestamp,
		},
		Body: indexBody.String(),
	}
	if err := writeConceptDoc(filepath.Join(*outDir, "index.md"), indexDoc); err != nil {
		log.Fatalf("Failed to write index.md: %v", err)
	}
	fmt.Println("Produced index.md")
	fmt.Println("OKF bundle production completed successfully.")
}

func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	dbPath := fs.String("db", "", "Path to SQLite database file (required)")
	bundleDir := fs.String("bundle", "", "Path to OKF bundle directory (required)")
	sync := fs.Bool("sync", false, "Create missing tables/columns (optional)")
	fs.Parse(args)

	if *dbPath == "" || *bundleDir == "" {
		fs.Usage()
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	tablesDir := filepath.Join(*bundleDir, "tables")
	if _, err := os.Stat(tablesDir); os.IsNotExist(err) {
		log.Fatalf("Tables directory not found in bundle: %s", tablesDir)
	}

	// Read all files in tables/
	files, err := os.ReadDir(tablesDir)
	if err != nil {
		log.Fatalf("Failed to read tables directory: %v", err)
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

		parsedCols := parseColumnsFromMarkdown(doc.Body)
		if len(parsedCols) == 0 {
			fmt.Printf("Warning: No columns parsed from table body in %s\n", file.Name())
			continue
		}

		// Check if table exists in DB
		var name string
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&name)
		if err == sql.ErrNoRows {
			fmt.Printf("Table '%s' is missing in the database.\n", tableName)
			if *sync {
				// Recreate table
				var colDefs []string
				for _, col := range parsedCols {
					def := fmt.Sprintf("`%s` %s", col.Name, col.Type)
					if !col.Nullable {
						def += " NOT NULL"
					}
					if col.PrimaryKey {
						def += " PRIMARY KEY"
					}
					if col.Default != "" && col.Default != "NULL" {
						def += " DEFAULT " + col.Default
					}
					colDefs = append(colDefs, def)
				}
				query := fmt.Sprintf("CREATE TABLE `%s` (%s)", tableName, strings.Join(colDefs, ", "))
				_, err := db.Exec(query)
				if err != nil {
					log.Fatalf("Failed to create table %s: %v. Query: %s", tableName, err, query)
				}
				fmt.Printf("  -> Successfully created table '%s'.\n", tableName)
			}
		} else if err != nil {
			log.Fatalf("Query error checking table %s: %v", tableName, err)
		} else {
			// Table exists, verify columns
			existingCols, err := getTableColumns(db, tableName)
			if err != nil {
				log.Fatalf("Failed to query columns for table %s: %v", tableName, err)
			}

			// Map existing columns
			existingMap := make(map[string]Column)
			for _, col := range existingCols {
				existingMap[strings.ToLower(col.Name)] = col
			}

			for _, col := range parsedCols {
				existCol, found := existingMap[strings.ToLower(col.Name)]
				if !found {
					fmt.Printf("Table '%s' is missing column '%s'.\n", tableName, col.Name)
					if *sync {
						def := fmt.Sprintf("`%s` %s", col.Name, col.Type)
						if !col.Nullable {
							def += " NOT NULL"
						}
						if col.Default != "" && col.Default != "NULL" {
							def += " DEFAULT " + col.Default
						}
						query := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", tableName, def)
						_, err := db.Exec(query)
						if err != nil {
							log.Fatalf("Failed to add column %s to table %s: %v", col.Name, tableName, err)
						}
						fmt.Printf("  -> Successfully added column '%s' to table '%s'.\n", col.Name, tableName)
					}
				} else {
					// Validate types (case-insensitive compare)
					if !strings.EqualFold(existCol.Type, col.Type) {
						fmt.Printf("Table '%s' column '%s' type mismatch: DB has '%s', OKF has '%s'\n", tableName, col.Name, existCol.Type, col.Type)
					}
				}
			}
		}
	}
	fmt.Println("OKF bundle ingestion / verification finished.")
}

func getTableColumns(db *sql.DB, tableName string) ([]Column, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(`%s`)", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []Column
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notnull int
		var dfltVal sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltVal, &pk); err != nil {
			return nil, err
		}

		cols = append(cols, Column{
			Name:       name,
			Type:       colType,
			PrimaryKey: pk > 0,
			Nullable:   notnull == 0,
			Default:    dfltVal.String,
		})
	}
	return cols, nil
}

func parseColumnsFromMarkdown(body string) []Column {
	var cols []Column
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		// Skip header and dividers
		if strings.Contains(line, "---") || strings.Contains(strings.ToLower(line), "type") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			continue
		}

		// Format: | Name | Type | Primary Key | Nullable | Default |
		name := strings.TrimSpace(parts[1])
		colType := strings.TrimSpace(parts[2])
		pk := strings.TrimSpace(strings.ToLower(parts[3])) == "yes"
		nullable := strings.TrimSpace(strings.ToLower(parts[4])) == "yes"
		dflt := strings.TrimSpace(parts[5])

		cols = append(cols, Column{
			Name:       name,
			Type:       colType,
			PrimaryKey: pk,
			Nullable:   nullable,
			Default:    dflt,
		})
	}
	return cols
}

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

func readConceptDoc(filePath string) (*ConceptDoc, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	parts := bytes.SplitN(content, []byte("---\n"), 3)
	if len(parts) < 3 {
		// try with CRLF
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
