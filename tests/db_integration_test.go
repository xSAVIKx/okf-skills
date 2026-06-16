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
