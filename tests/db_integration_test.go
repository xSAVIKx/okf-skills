package tests

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func TestSQLiteIntegration(t *testing.T) {
	binaryPath := getBinaryPath("okf-sqlite")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("SQLite binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-sqlite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL, email TEXT)")
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	// 1. Run produce
	outDir := filepath.Join(tempDir, "bundle")
	cmdProduce := exec.Command(binaryPath, "produce", "--db", dbPath, "--out", outDir)
	var stderr bytesBuffer
	cmdProduce.Stderr = &stderr
	if err := cmdProduce.Run(); err != nil {
		t.Fatalf("sqlite produce command failed: %v. Stderr: %s", err, stderr.String())
	}

	// 2. Validate output bundle files
	indexFile := filepath.Join(outDir, "index.md")
	userFile := filepath.Join(outDir, "tables", "users.md")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Errorf("index.md was not produced")
	}
	if _, err := os.Stat(userFile); os.IsNotExist(err) {
		t.Errorf("users.md was not produced")
	}

	// Verify index.md contains okf_version: "0.1"
	indexContent, _ := os.ReadFile(indexFile)
	if !strings.Contains(string(indexContent), "okf_version: \"0.1\"") {
		t.Errorf("index.md does not contain okf_version: \"0.1\"")
	}

	// 3. Run ingest and sync a new table
	// Write a new table markdown file to the bundle
	productFile := filepath.Join(outDir, "tables", "products.md")
	productMD := `---
type: SQLite Table
title: products
description: Store products
---
# Columns

| Name | Type | Primary Key | Nullable | Default |
| --- | --- | --- | --- | --- |
| id | INTEGER | Yes | No |  |
| name | TEXT | No | No |  |
`
	if err := os.WriteFile(productFile, []byte(productMD), 0644); err != nil {
		t.Fatalf("failed to write products test file: %v", err)
	}

	cmdIngest := exec.Command(binaryPath, "ingest", "--db", dbPath, "--bundle", outDir, "--sync")
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("sqlite ingest command failed: %v. Stderr: %s", err, stderr.String())
	}

	// Verify products table exists in SQLite
	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='products'").Scan(&name)
	if err != nil {
		t.Errorf("products table was not created in SQLite: %v", err)
	}
}

func TestSQLiteRelationships(t *testing.T) {
	binaryPath := getBinaryPath("okf-sqlite")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("SQLite binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-sqlite-rel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create customers table: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER, FOREIGN KEY(customer_id) REFERENCES customers(id))"); err != nil {
		t.Fatalf("failed to create orders table: %v", err)
	}

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "--db", dbPath, "--out", outDir, "--relationships")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("sqlite produce --relationships failed: %v. Stderr: %s", err, stderr.String())
	}

	ordersContent, err := os.ReadFile(filepath.Join(outDir, "tables", "orders.md"))
	if err != nil {
		t.Fatalf("failed to read orders.md: %v", err)
	}
	body := string(ordersContent)
	if !strings.Contains(body, "# Relationships") {
		t.Errorf("orders.md missing # Relationships section:\n%s", body)
	}
	if !strings.Contains(body, "[customers](/tables/customers.md)") {
		t.Errorf("orders.md missing bundle-relative FK link to customers:\n%s", body)
	}
	// The link target must resolve to an actually produced concept.
	if _, err := os.Stat(filepath.Join(outDir, "tables", "customers.md")); err != nil {
		t.Errorf("FK target customers.md does not exist: %v", err)
	}

	// customers has no FK -> must NOT emit a Relationships section.
	customersContent, _ := os.ReadFile(filepath.Join(outDir, "tables", "customers.md"))
	if strings.Contains(string(customersContent), "# Relationships") {
		t.Errorf("customers.md should not have a Relationships section:\n%s", string(customersContent))
	}
}

func TestSQLiteIncrementalProduce(t *testing.T) {
	binaryPath := getBinaryPath("okf-sqlite")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("SQLite binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-sqlite-incr-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	outDir := filepath.Join(tempDir, "bundle")
	produce := func() {
		t.Helper()
		cmd := exec.Command(binaryPath, "produce", "--db", dbPath, "--out", outDir)
		var stderr bytesBuffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("produce failed: %v. Stderr: %s", err, stderr.String())
		}
	}

	usersFile := filepath.Join(outDir, "tables", "users.md")

	// 1. First produce stamps a content_hash and a log.md Creation entry.
	produce()
	first, err := os.ReadFile(usersFile)
	if err != nil {
		t.Fatalf("failed to read users.md: %v", err)
	}
	if !strings.Contains(string(first), "content_hash:") {
		t.Errorf("users.md missing content_hash after first produce:\n%s", string(first))
	}
	logFile := filepath.Join(outDir, "log.md")
	if logBytes, _ := os.ReadFile(logFile); !strings.Contains(string(logBytes), "**Creation**") {
		t.Errorf("log.md missing Creation entry:\n%s", string(logBytes))
	}

	// 2. Hand-edit the description (simulating okf-enrich), then re-produce the
	//    UNCHANGED source: the edited description must survive byte-for-byte.
	enriched := strings.Replace(string(first), "description: SQLite table users",
		"description: The application's user accounts.", 1)
	if enriched == string(first) {
		t.Fatalf("test setup: description line not found to edit:\n%s", string(first))
	}
	if err := os.WriteFile(usersFile, []byte(enriched), 0644); err != nil {
		t.Fatalf("failed to write enriched users.md: %v", err)
	}

	produce()
	afterUnchanged, _ := os.ReadFile(usersFile)
	if string(afterUnchanged) != enriched {
		t.Errorf("re-produce over unchanged source must preserve the file byte-for-byte.\nwant:\n%s\ngot:\n%s", enriched, string(afterUnchanged))
	}

	// 3. Alter the structure (add a column): only this concept is rewritten, the
	//    enriched description still survives, and a log.md Update entry is appended.
	if _, err := db.Exec("ALTER TABLE users ADD COLUMN email TEXT"); err != nil {
		t.Fatalf("failed to alter users table: %v", err)
	}
	produce()
	afterChange, _ := os.ReadFile(usersFile)
	body := string(afterChange)
	if !strings.Contains(body, "The application's user accounts.") {
		t.Errorf("enriched description must survive a structural change:\n%s", body)
	}
	if !strings.Contains(body, "email") {
		t.Errorf("new column must appear after structural change:\n%s", body)
	}
	if logBytes, _ := os.ReadFile(logFile); !strings.Contains(string(logBytes), "**Update**") {
		t.Errorf("log.md missing Update entry after structural change:\n%s", string(logBytes))
	}
}

// assertCompositeFKEdges verifies a composite (multi-column) foreign key to
// product_variants renders as exactly one edge per referencing column with no
// duplicates. It is the regression guard for the information_schema cross-join
// bug, where joining KEY_COLUMN_USAGE to CONSTRAINT_COLUMN_USAGE on the
// constraint name alone cross-products the N referencing columns with the N
// referenced columns, emitting N*N duplicate edges.
func assertCompositeFKEdges(t *testing.T, body string) {
	t.Helper()
	if got := strings.Count(body, "(/tables/product_variants.md)"); got != 2 {
		t.Fatalf("expected exactly 2 composite-FK edges to product_variants, got %d:\n%s", got, body)
	}
	if got := strings.Count(body, "FK on product_id ["); got != 1 {
		t.Errorf("expected exactly one 'FK on product_id' edge, got %d:\n%s", got, body)
	}
	if got := strings.Count(body, "FK on sku ["); got != 1 {
		t.Errorf("expected exactly one 'FK on sku' edge, got %d:\n%s", got, body)
	}
}

func TestSQLiteCompositeFK(t *testing.T) {
	binaryPath := getBinaryPath("okf-sqlite")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("SQLite binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-sqlite-compositefk-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE product_variants (product_id INTEGER NOT NULL, sku TEXT NOT NULL, PRIMARY KEY (product_id, sku))"); err != nil {
		t.Fatalf("failed to create product_variants table: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE shipments (id INTEGER PRIMARY KEY, product_id INTEGER NOT NULL, sku TEXT NOT NULL, FOREIGN KEY (product_id, sku) REFERENCES product_variants(product_id, sku))"); err != nil {
		t.Fatalf("failed to create shipments table: %v", err)
	}

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "--db", dbPath, "--out", outDir, "--relationships")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("sqlite produce --relationships failed: %v. Stderr: %s", err, stderr.String())
	}

	body, err := os.ReadFile(filepath.Join(outDir, "tables", "shipments.md"))
	if err != nil {
		t.Fatalf("failed to read shipments.md: %v", err)
	}
	assertCompositeFKEdges(t, string(body))
}

func TestSQLiteRicherGrounding(t *testing.T) {
	binaryPath := getBinaryPath("okf-sqlite")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("SQLite binary not found at %s.", binaryPath)
	}

	tempDir, _ := os.MkdirTemp("", "okf-sqlite-grounding-*")
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	db.Exec("CREATE TABLE accounts (id TEXT PRIMARY KEY, email TEXT, status TEXT)")
	db.Exec("INSERT INTO accounts VALUES ('123e4567-e89b-12d3-a456-426614174000','a@x.com','active'),('00000000-0000-0000-0000-000000000002','b@y.com','pending'),('00000000-0000-0000-0000-000000000003','c@z.com','active')")

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "--db", dbPath, "--out", outDir, "--profile")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("produce --profile failed: %v. Stderr: %s", err, stderr.String())
	}

	body, _ := os.ReadFile(filepath.Join(outDir, "tables", "accounts.md"))
	s := string(body)
	for _, want := range []string{"| Semantic |", "email", "uuid", "## Data Profile", "status ∈ {active, pending}"} {
		if !strings.Contains(s, want) {
			t.Errorf("accounts.md missing %q:\n%s", want, s)
		}
	}

	// Re-produce on the unchanged DB: the profile and its derived semantic tags
	// must be byte-identical, so the value-set sampling stays deterministic and the
	// incremental-produce hash holds under --profile.
	if err := exec.Command(binaryPath, "produce", "--db", dbPath, "--out", outDir, "--profile").Run(); err != nil {
		t.Fatalf("second produce --profile failed: %v", err)
	}
	body2, _ := os.ReadFile(filepath.Join(outDir, "tables", "accounts.md"))
	if string(body2) != s {
		t.Errorf("accounts.md not byte-stable across a --profile re-produce:\nfirst:\n%s\nsecond:\n%s", s, string(body2))
	}
}

func TestMySQLIntegration(t *testing.T) {
	binaryPath := getBinaryPath("okf-mysql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("MySQL binary not found at %s. Build it first.", binaryPath)
	}

	if !isPortOpen("localhost", 3306) {
		t.Skip("MySQL container is not running on localhost:3306. Skipping integration test.")
	}

	dsn := "root:secret@tcp(localhost:3306)/ecommerce?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to connect to mysql: %v", err)
	}
	defer db.Close()

	// Reset product comments to initial values
	_, _ = db.Exec("ALTER TABLE products COMMENT = 'Catalog of products available for purchase'")
	_, _ = db.Exec("ALTER TABLE products MODIFY COLUMN price decimal(10,2) NOT NULL COMMENT 'Unit price in USD'")

	tempDir, err := os.MkdirTemp("", "okf-mysql-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "bundle")

	// 1. Run produce
	cmdProduce := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "3306", "-user", "root", "-password", "secret", "-db", "ecommerce", "-out", outDir)
	var stderr bytesBuffer
	cmdProduce.Stderr = &stderr
	if err := cmdProduce.Run(); err != nil {
		t.Fatalf("mysql produce failed: %v. Stderr: %s", err, stderr.String())
	}

	// 2. Validate index.md
	indexFile := filepath.Join(outDir, "index.md")
	indexContent, _ := os.ReadFile(indexFile)
	if !strings.Contains(string(indexContent), "okf_version: \"0.1\"") {
		t.Errorf("mysql index.md does not contain okf_version: \"0.1\"")
	}

	// 3. Modify table and column comments in products.md
	productFile := filepath.Join(outDir, "tables", "products.md")
	productContent, err := os.ReadFile(productFile)
	if err != nil {
		t.Fatalf("failed to read products.md: %v", err)
	}
	modifiedContent := strings.Replace(string(productContent), "Catalog of products available for purchase", "Catalog of products - INTEGRATION TEST", 1)
	modifiedContent = strings.Replace(modifiedContent, "Unit price in USD", "Unit price in USD - INTEGRATION TEST", 1)
	if err := os.WriteFile(productFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to write back products.md: %v", err)
	}

	// 4. Ingest and sync comments
	cmdIngest := exec.Command(binaryPath, "ingest", "-host", "localhost", "-port", "3306", "-user", "root", "-password", "secret", "-db", "ecommerce", "-bundle", outDir, "-sync")
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("mysql ingest failed: %v. Stderr: %s", err, stderr.String())
	}

	// 5. Query DB and verify comments are updated
	var tblComment string
	err = db.QueryRow("SELECT COALESCE(TABLE_COMMENT, '') FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = 'ecommerce' AND TABLE_NAME = 'products'").Scan(&tblComment)
	if err != nil || tblComment != "Catalog of products - INTEGRATION TEST" {
		t.Errorf("table comment was not synced: expected %q, got %q (err: %v)", "Catalog of products - INTEGRATION TEST", tblComment, err)
	}

	var colComment string
	err = db.QueryRow("SELECT COALESCE(COLUMN_COMMENT, '') FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = 'ecommerce' AND TABLE_NAME = 'products' AND COLUMN_NAME = 'price'").Scan(&colComment)
	if err != nil || colComment != "Unit price in USD - INTEGRATION TEST" {
		t.Errorf("column comment was not synced: expected %q, got %q (err: %v)", "Unit price in USD - INTEGRATION TEST", colComment, err)
	}
}

func TestMySQLRelationships(t *testing.T) {
	binaryPath := getBinaryPath("okf-mysql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("MySQL binary not found at %s. Build it first.", binaryPath)
	}
	if !isPortOpen("localhost", 3306) {
		t.Skip("MySQL container is not running on localhost:3306. Skipping integration test.")
	}

	tempDir, err := os.MkdirTemp("", "okf-mysql-rel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "3306", "-user", "root", "-password", "secret", "-db", "ecommerce", "-out", outDir, "-relationships")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("mysql produce -relationships failed: %v. Stderr: %s", err, stderr.String())
	}

	orders, err := os.ReadFile(filepath.Join(outDir, "tables", "orders.md"))
	if err != nil {
		t.Fatalf("failed to read orders.md: %v", err)
	}
	body := string(orders)
	if !strings.Contains(body, "# Relationships") || !strings.Contains(body, "[users](/tables/users.md)") {
		t.Errorf("orders.md missing FK relationship to users:\n%s", body)
	}
}

func TestMySQLCompositeFK(t *testing.T) {
	binaryPath := getBinaryPath("okf-mysql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("MySQL binary not found at %s. Build it first.", binaryPath)
	}
	if !isPortOpen("localhost", 3306) {
		t.Skip("MySQL container is not running on localhost:3306. Skipping integration test.")
	}

	tempDir, err := os.MkdirTemp("", "okf-mysql-compositefk-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "3306", "-user", "root", "-password", "secret", "-db", "ecommerce", "-out", outDir, "-relationships")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("mysql produce -relationships failed: %v. Stderr: %s", err, stderr.String())
	}

	body, err := os.ReadFile(filepath.Join(outDir, "tables", "shipments.md"))
	if err != nil {
		t.Fatalf("failed to read shipments.md: %v", err)
	}
	assertCompositeFKEdges(t, string(body))
}

func TestPostgreSQLRelationships(t *testing.T) {
	binaryPath := getBinaryPath("okf-postgresql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("PostgreSQL binary not found at %s. Build it first.", binaryPath)
	}
	if !isPortOpen("localhost", 5432) {
		t.Skip("PostgreSQL container is not running on localhost:5432. Skipping integration test.")
	}

	tempDir, err := os.MkdirTemp("", "okf-postgres-rel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "5432", "-user", "postgres", "-password", "secret", "-db", "ecommerce", "-out", outDir, "-relationships")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("postgres produce -relationships failed: %v. Stderr: %s", err, stderr.String())
	}

	orders, err := os.ReadFile(filepath.Join(outDir, "tables", "orders.md"))
	if err != nil {
		t.Fatalf("failed to read orders.md: %v", err)
	}
	body := string(orders)
	if !strings.Contains(body, "# Relationships") || !strings.Contains(body, "[users](/tables/users.md)") {
		t.Errorf("orders.md missing FK relationship to users:\n%s", body)
	}
}

func TestPostgreSQLCompositeFK(t *testing.T) {
	binaryPath := getBinaryPath("okf-postgresql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("PostgreSQL binary not found at %s. Build it first.", binaryPath)
	}
	if !isPortOpen("localhost", 5432) {
		t.Skip("PostgreSQL container is not running on localhost:5432. Skipping integration test.")
	}

	tempDir, err := os.MkdirTemp("", "okf-postgres-compositefk-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "5432", "-user", "postgres", "-password", "secret", "-db", "ecommerce", "-out", outDir, "-relationships")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("postgres produce -relationships failed: %v. Stderr: %s", err, stderr.String())
	}

	body, err := os.ReadFile(filepath.Join(outDir, "tables", "shipments.md"))
	if err != nil {
		t.Fatalf("failed to read shipments.md: %v", err)
	}
	assertCompositeFKEdges(t, string(body))
}

func TestPostgreSQLIntegration(t *testing.T) {
	binaryPath := getBinaryPath("okf-postgresql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("PostgreSQL binary not found at %s. Build it first.", binaryPath)
	}

	if !isPortOpen("localhost", 5432) {
		t.Skip("PostgreSQL container is not running on localhost:5432. Skipping integration test.")
	}

	connStr := "host=localhost port=5432 user=postgres password=secret dbname=ecommerce sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer db.Close()

	// Reset table and column comments
	_, _ = db.Exec("COMMENT ON TABLE products IS 'Catalog of products available for purchase'")
	_, _ = db.Exec("COMMENT ON COLUMN products.price IS 'Unit price in USD'")

	tempDir, err := os.MkdirTemp("", "okf-postgres-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "bundle")

	// 1. Run produce
	cmdProduce := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "5432", "-user", "postgres", "-password", "secret", "-db", "ecommerce", "-out", outDir)
	var stderr bytesBuffer
	cmdProduce.Stderr = &stderr
	if err := cmdProduce.Run(); err != nil {
		t.Fatalf("postgres produce failed: %v. Stderr: %s", err, stderr.String())
	}

	// 2. Validate index.md
	indexFile := filepath.Join(outDir, "index.md")
	indexContent, _ := os.ReadFile(indexFile)
	if !strings.Contains(string(indexContent), "okf_version: \"0.1\"") {
		t.Errorf("postgres index.md does not contain okf_version: \"0.1\"")
	}

	// 3. Modify table and column comments in products.md
	productFile := filepath.Join(outDir, "tables", "products.md")
	productContent, err := os.ReadFile(productFile)
	if err != nil {
		t.Fatalf("failed to read products.md: %v", err)
	}
	modifiedContent := strings.Replace(string(productContent), "Catalog of products available for purchase", "Catalog of products - PG INTEGRATION", 1)
	modifiedContent = strings.Replace(modifiedContent, "Unit price in USD", "Unit price in USD - PG INTEGRATION", 1)
	if err := os.WriteFile(productFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to write back products.md: %v", err)
	}

	// 4. Ingest and sync comments
	cmdIngest := exec.Command(binaryPath, "ingest", "-host", "localhost", "-port", "5432", "-user", "postgres", "-password", "secret", "-db", "ecommerce", "-bundle", outDir, "-sync")
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("postgres ingest failed: %v. Stderr: %s", err, stderr.String())
	}

	// 5. Query DB and verify comments are updated
	var tblComment string
	err = db.QueryRow(`
		SELECT COALESCE(obj_description(c.oid, 'pg_class'), '')
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r' AND n.nspname = 'public' AND c.relname = 'products'`).Scan(&tblComment)
	if err != nil || tblComment != "Catalog of products - PG INTEGRATION" {
		t.Errorf("table comment was not synced: expected %q, got %q (err: %v)", "Catalog of products - PG INTEGRATION", tblComment, err)
	}

	var colComment string
	err = db.QueryRow(`
		SELECT COALESCE(col_description(c.oid, a.attnum), '')
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = 'products' AND n.nspname = 'public' AND a.attname = 'price' AND a.attnum > 0 AND NOT a.attisdropped`).Scan(&colComment)
	if err != nil || colComment != "Unit price in USD - PG INTEGRATION" {
		t.Errorf("column comment was not synced: expected %q, got %q (err: %v)", "Unit price in USD - PG INTEGRATION", colComment, err)
	}
}

func TestSQLiteMetadataUpgrades(t *testing.T) {
	binaryPath := getBinaryPath("okf-sqlite")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("SQLite binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-sqlite-meta-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite db: %v", err)
	}
	defer db.Close()

	stmts := []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT UNIQUE, age INTEGER CHECK (age >= 0), created_at TEXT)",
		"CREATE INDEX idx_users_age ON users(age)",
		"INSERT INTO users (email, age, created_at) VALUES ('a@x.com', 30, '2019-01-01'), ('b@x.com', 40, '2026-06-14')",
		"CREATE VIEW adult_users AS SELECT id, email FROM users WHERE age >= 18",
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("setup failed (%s): %v", s, err)
		}
	}

	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "--db", dbPath, "--out", outDir, "--stats")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("produce --stats failed: %v. Stderr: %s", err, stderr.String())
	}

	// Table concept: constraints, indexes, and stats.
	usersBody, err := os.ReadFile(filepath.Join(outDir, "tables", "users.md"))
	if err != nil {
		t.Fatalf("failed to read users.md: %v", err)
	}
	u := string(usersBody)
	for _, want := range []string{"## Constraints", "UNIQUE", "CHECK", "age >= 0", "## Indexes", "idx_users_age", "## Stats", "**Row Count**: 2", "**Freshness** (`created_at`)"} {
		if !strings.Contains(u, want) {
			t.Errorf("users.md missing %q:\n%s", want, u)
		}
	}

	// View concept: type ends in View, defining SQL captured.
	viewBody, err := os.ReadFile(filepath.Join(outDir, "tables", "adult_users.md"))
	if err != nil {
		t.Fatalf("failed to read adult_users.md: %v", err)
	}
	v := string(viewBody)
	if !strings.Contains(v, "type: SQLite View") {
		t.Errorf("adult_users.md type should be SQLite View:\n%s", v)
	}
	if !strings.Contains(v, "## View Definition") || !strings.Contains(v, "CREATE VIEW") {
		t.Errorf("adult_users.md missing captured view definition:\n%s", v)
	}

	// ingest must still parse columns despite the new sections.
	cmdIngest := exec.Command(binaryPath, "ingest", "--db", dbPath, "--bundle", outDir)
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("ingest after metadata upgrades failed: %v. Stderr: %s", err, stderr.String())
	}
}

func TestMySQLMetadataUpgrades(t *testing.T) {
	binaryPath := getBinaryPath("okf-mysql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("MySQL binary not found at %s.", binaryPath)
	}
	if !isPortOpen("localhost", 3306) {
		t.Skip("MySQL container not running on localhost:3306.")
	}

	dsn := "root:secret@tcp(localhost:3306)/ecommerce?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("connect mysql: %v", err)
	}
	defer db.Close()

	db.Exec("DROP VIEW IF EXISTS meta_active")
	db.Exec("DROP TABLE IF EXISTS meta_widgets")
	if _, err := db.Exec("CREATE TABLE meta_widgets (id INT PRIMARY KEY, sku VARCHAR(32) UNIQUE, qty INT CHECK (qty >= 0), created_at DATETIME)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	db.Exec("INSERT INTO meta_widgets VALUES (1,'a',5,'2019-01-01 00:00:00'),(2,'b',9,'2026-06-14 00:00:00')")
	if _, err := db.Exec("CREATE VIEW meta_active AS SELECT id, sku FROM meta_widgets WHERE qty > 0"); err != nil {
		t.Fatalf("create view: %v", err)
	}
	defer func() {
		db.Exec("DROP VIEW IF EXISTS meta_active")
		db.Exec("DROP TABLE IF EXISTS meta_widgets")
	}()

	tempDir, _ := os.MkdirTemp("", "okf-mysql-meta-*")
	defer os.RemoveAll(tempDir)
	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "3306", "-user", "root", "-password", "secret", "-db", "ecommerce", "-out", outDir, "-stats", "-tables", "meta_widgets,meta_active")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("produce failed: %v. Stderr: %s", err, stderr.String())
	}

	w, _ := os.ReadFile(filepath.Join(outDir, "tables", "meta_widgets.md"))
	for _, want := range []string{"## Constraints", "UNIQUE", "CHECK", "## Indexes", "## Stats", "**Row Count**: 2", "**Freshness**"} {
		if !strings.Contains(string(w), want) {
			t.Errorf("meta_widgets.md missing %q:\n%s", want, string(w))
		}
	}
	v, _ := os.ReadFile(filepath.Join(outDir, "tables", "meta_active.md"))
	if !strings.Contains(string(v), "type: MySQL View") || !strings.Contains(string(v), "## View Definition") {
		t.Errorf("meta_active.md missing view type/definition:\n%s", string(v))
	}
}

func TestPostgreSQLMetadataUpgrades(t *testing.T) {
	binaryPath := getBinaryPath("okf-postgresql")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("PostgreSQL binary not found at %s.", binaryPath)
	}
	if !isPortOpen("localhost", 5432) {
		t.Skip("PostgreSQL container not running on localhost:5432.")
	}

	connStr := "host=localhost port=5432 user=postgres password=secret dbname=ecommerce sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	db.Exec("DROP VIEW IF EXISTS meta_active")
	db.Exec("DROP TABLE IF EXISTS meta_widgets")
	if _, err := db.Exec("CREATE TABLE meta_widgets (id INT PRIMARY KEY, sku VARCHAR(32) UNIQUE, qty INT CHECK (qty >= 0), created_at TIMESTAMP)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	db.Exec("INSERT INTO meta_widgets VALUES (1,'a',5,'2019-01-01'),(2,'b',9,'2026-06-14')")
	if _, err := db.Exec("CREATE VIEW meta_active AS SELECT id, sku FROM meta_widgets WHERE qty > 0"); err != nil {
		t.Fatalf("create view: %v", err)
	}
	defer func() {
		db.Exec("DROP VIEW IF EXISTS meta_active")
		db.Exec("DROP TABLE IF EXISTS meta_widgets")
	}()

	tempDir, _ := os.MkdirTemp("", "okf-pg-meta-*")
	defer os.RemoveAll(tempDir)
	outDir := filepath.Join(tempDir, "bundle")
	cmd := exec.Command(binaryPath, "produce", "-host", "localhost", "-port", "5432", "-user", "postgres", "-password", "secret", "-db", "ecommerce", "-out", outDir, "-stats", "-tables", "meta_widgets,meta_active")
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("produce failed: %v. Stderr: %s", err, stderr.String())
	}

	w, _ := os.ReadFile(filepath.Join(outDir, "tables", "meta_widgets.md"))
	for _, want := range []string{"## Constraints", "UNIQUE", "CHECK", "## Indexes", "## Stats", "**Row Count**: 2", "**Freshness**"} {
		if !strings.Contains(string(w), want) {
			t.Errorf("meta_widgets.md missing %q:\n%s", want, string(w))
		}
	}
	v, _ := os.ReadFile(filepath.Join(outDir, "tables", "meta_active.md"))
	if !strings.Contains(string(v), "type: PostgreSQL View") || !strings.Contains(string(v), "## View Definition") {
		t.Errorf("meta_active.md missing view type/definition:\n%s", string(v))
	}
}
