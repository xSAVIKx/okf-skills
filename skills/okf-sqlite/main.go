// Package main implements the SQLite OKF (Open Knowledge Format) connector.
// It provides commands to produce OKF bundles from local SQLite databases
// and ingest/verify existing databases against OKF specifications.
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
	"github.com/savikne/okf-skills-registry/okf-go"
)

// Column represents the properties of a database table column.
type Column struct {
	Name       string // Name of the column
	Type       string // Column database type (e.g., INTEGER, TEXT)
	PrimaryKey bool   // True if the column is part of the primary key
	Nullable   bool   // True if the column is nullable
	Default    string // Default value of the column
}

// main is the CLI entrypoint. It routes commands to produce or ingest subcommands.
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

// printUsage outputs the available CLI commands and formatting options.
func printUsage() {
	fmt.Println("Usage: okf-sqlite <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  produce  - Create OKF bundle from SQLite database")
	fmt.Println("  ingest   - Verify or sync OKF bundle schema into SQLite database")
	fmt.Println("\nRun 'okf-sqlite <command> -h' for command-specific options.")
}

// runProduce implements the 'produce' subcommand, extracting SQLite tables and schemas
// into a set of OKF Markdown concept documents.
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

	// 1. Get all tables from the sqlite schema
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

	// Create output directory for tables
	tablesDir := filepath.Join(*outDir, "tables")
	if err := os.MkdirAll(tablesDir, 0755); err != nil {
		log.Fatalf("Failed to create tables directory: %v", err)
	}

	absDbPath, _ := filepath.Abs(*dbPath)
	timestamp := time.Now().Format(time.RFC3339)

	// 2. Generate concept documents for each table
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

		doc := okf.ConceptDoc{
			Frontmatter: okf.Frontmatter{
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
		if err := okf.WriteConceptDoc(filePath, doc); err != nil {
			log.Fatalf("Failed to write concept doc for %s: %v", table, err)
		}
		fmt.Printf("Produced concept doc: %s\n", filePath)
	}

	// 3. Generate index.md at the bundle root listing all extracted tables
	var indexBody bytes.Buffer
	fmt.Fprintf(&indexBody, "# Database Schema: %s (SQLite)\n\n", filepath.Base(*dbPath))
	indexBody.WriteString("This OKF bundle represents the tables extracted from SQLite.\n\n")
	indexBody.WriteString("## Tables\n\n")
	for _, table := range tables {
		fmt.Fprintf(&indexBody, "- [%s](tables/%s.md) - SQLite table %s\n", table, table, table)
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
	fmt.Println("Produced index.md")
	fmt.Println("OKF bundle production completed successfully.")
}

// runIngest implements the 'ingest' subcommand, parsing OKF bundles,
// validating existing SQLite tables, or optionally syncing schema DDL.
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

	// Read all table files inside the OKF bundle
	files, err := os.ReadDir(tablesDir)
	if err != nil {
		log.Fatalf("Failed to read tables directory: %v", err)
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
				// Recreate the table from parsed columns definition
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

			// Map existing columns for comparison
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
					// Validate types (case-insensitive comparison)
					if !strings.EqualFold(existCol.Type, col.Type) {
						fmt.Printf("Table '%s' column '%s' type mismatch: DB has '%s', OKF has '%s'\n", tableName, col.Name, existCol.Type, col.Type)
					}
				}
			}
		}
	}
	fmt.Println("OKF bundle ingestion / verification finished.")
}

// getTableColumns queries PRAGMA table_info to retrieve SQLite column definitions.
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

// parseColumnsFromMarkdown extracts column information from the OKF markdown body table.
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

		// Parse row: | Name | Type | Primary Key | Nullable | Default |
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

// writeConceptDoc and readConceptDoc deleted because they are now part of the okf-go library.
