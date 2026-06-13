package tests

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// getBinaryPath returns the path to the compiled skill binary.
func getBinaryPath(skillName string) string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return filepath.Join("..", "skills", skillName, skillName+ext)
}

// isPortOpen returns true if a TCP port is open.
func isPortOpen(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

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

func TestFSIntegration(t *testing.T) {
	binaryPath := getBinaryPath("okf-fs")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("FS binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-fs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "my-src")
	bundleDir := filepath.Join(tempDir, "my-bundle")

	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatalf("failed to create src dir structure: %v", err)
	}

	// Create test files
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello file1"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "file2.txt"), []byte("hello file2"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ignored.txt"), []byte("should be ignored"), 0644); err != nil {
		t.Fatalf("failed to write ignored.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".okfignore"), []byte("ignored.txt\n"), 0644); err != nil {
		t.Fatalf("failed to write .okfignore: %v", err)
	}

	// Create metadata file
	metaContent := "file1.txt: \"The main configuration file\"\nsub/file2.txt: \"Helper function templates\"\n"
	if err := os.WriteFile(filepath.Join(srcDir, ".okf-metadata.yaml"), []byte(metaContent), 0644); err != nil {
		t.Fatalf("failed to write .okf-metadata.yaml: %v", err)
	}

	// 1. Run produce
	cmdProduce := exec.Command(binaryPath, "produce", "--dir", srcDir, "--out", bundleDir)
	var stderr bytesBuffer
	cmdProduce.Stderr = &stderr
	if err := cmdProduce.Run(); err != nil {
		t.Fatalf("fs produce command failed: %v. Stderr: %s", err, stderr.String())
	}

	// 2. Validate output bundle files
	indexFile := filepath.Join(bundleDir, "index.md")
	file1Doc := filepath.Join(bundleDir, "file1.txt.md")
	file2Doc := filepath.Join(bundleDir, "sub", "file2.txt.md")
	ignoredDoc := filepath.Join(bundleDir, "ignored.txt.md")

	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Errorf("index.md was not produced")
	}
	if _, err := os.Stat(file1Doc); os.IsNotExist(err) {
		t.Errorf("file1.txt.md was not produced")
	}
	if _, err := os.Stat(file2Doc); os.IsNotExist(err) {
		t.Errorf("sub/file2.txt.md was not produced")
	}
	if _, err := os.Stat(ignoredDoc); !os.IsNotExist(err) {
		t.Errorf("ignored.txt.md should NOT be produced")
	}

	// Check file1Doc description matches metadata
	file1Bytes, _ := os.ReadFile(file1Doc)
	if !strings.Contains(string(file1Bytes), "description: The main configuration file") && !strings.Contains(string(file1Bytes), "description: \"The main configuration file\"") {
		t.Errorf("file1Doc frontmatter description is missing or incorrect: %s", string(file1Bytes))
	}

	// Check index.md contains okf_version: "0.1"
	indexBytes, _ := os.ReadFile(indexFile)
	if !strings.Contains(string(indexBytes), "okf_version: \"0.1\"") {
		t.Errorf("index.md does not contain okf_version: \"0.1\"")
	}

	// 3. Modify description in file1.txt.md and sync back
	modifiedFile1 := strings.Replace(string(file1Bytes), "description: The main configuration file", "description: The main configuration file - updated via OKF", 1)
	modifiedFile1 = strings.Replace(modifiedFile1, "description: \"The main configuration file\"", "description: The main configuration file - updated via OKF", 1)
	if err := os.WriteFile(file1Doc, []byte(modifiedFile1), 0644); err != nil {
		t.Fatalf("failed to write modified file1.txt.md: %v", err)
	}

	stderr.buf = nil
	cmdIngest := exec.Command(binaryPath, "ingest", "--dir", srcDir, "--bundle", bundleDir, "--sync")
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("fs ingest command failed: %v. Stderr: %s", err, stderr.String())
	}

	// Verify .okf-metadata.yaml was updated
	newMetaBytes, err := os.ReadFile(filepath.Join(srcDir, ".okf-metadata.yaml"))
	if err != nil {
		t.Fatalf("failed to read updated .okf-metadata.yaml: %v", err)
	}
	if !strings.Contains(string(newMetaBytes), "The main configuration file - updated via OKF") {
		t.Errorf(".okf-metadata.yaml was not updated: %s", string(newMetaBytes))
	}
}

func TestGitIntegration(t *testing.T) {
	binaryPath := getBinaryPath("okf-git")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("Git binary not found at %s. Build it first.", binaryPath)
	}

	tempDir, err := os.MkdirTemp("", "okf-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "my-repo")
	bundleDir := filepath.Join(tempDir, "my-bundle")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize Git repo using os/exec
	runCmd := func(dir string, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command failed: %s %v: %v", name, args, err)
		}
	}

	runCmd(repoDir, "git", "init")
	runCmd(repoDir, "git", "config", "user.name", "Test Author")
	runCmd(repoDir, "git", "config", "user.email", "test@example.com")

	// Create a file and commit it
	gitFile := filepath.Join(repoDir, "file_git.txt")
	if err := os.WriteFile(gitFile, []byte("hello git"), 0644); err != nil {
		t.Fatalf("failed to write file_git.txt: %v", err)
	}

	runCmd(repoDir, "git", "add", "file_git.txt")
	runCmd(repoDir, "git", "commit", "-m", "First commit of git file")

	// Create .okf-metadata.yaml
	metaContent := "file_git.txt: \"Git tracked text file\"\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".okf-metadata.yaml"), []byte(metaContent), 0644); err != nil {
		t.Fatalf("failed to write .okf-metadata.yaml: %v", err)
	}

	// 1. Run produce
	cmdProduce := exec.Command(binaryPath, "produce", "--repo", repoDir, "--out", bundleDir)
	var stderr bytesBuffer
	cmdProduce.Stderr = &stderr
	if err := cmdProduce.Run(); err != nil {
		t.Fatalf("git produce command failed: %v. Stderr: %s", err, stderr.String())
	}

	// 2. Validate output bundle files
	indexFile := filepath.Join(bundleDir, "index.md")
	gitDocFile := filepath.Join(bundleDir, "file_git.txt.md")

	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Errorf("index.md was not produced")
	}
	if _, err := os.Stat(gitDocFile); os.IsNotExist(err) {
		t.Errorf("file_git.txt.md was not produced")
	}

	// Verify index.md contains okf_version: "0.1"
	indexBytes, _ := os.ReadFile(indexFile)
	if !strings.Contains(string(indexBytes), "okf_version: \"0.1\"") {
		t.Errorf("index.md does not contain okf_version: \"0.1\"")
	}

	// Check gitDocFile contains author and commit message provenance
	gitDocBytes, _ := os.ReadFile(gitDocFile)
	gitDocStr := string(gitDocBytes)
	if !strings.Contains(gitDocStr, "Last Committer**: Test Author") {
		t.Errorf("file_git.txt.md does not contain committer: %s", gitDocStr)
	}
	if !strings.Contains(gitDocStr, "First commit of git file") {
		t.Errorf("file_git.txt.md does not contain commit message: %s", gitDocStr)
	}

	// 3. Modify description in file_git.txt.md and sync back
	modifiedGitDoc := strings.Replace(gitDocStr, "description: Git tracked text file", "description: Git tracked text file - modified in bundle", 1)
	modifiedGitDoc = strings.Replace(modifiedGitDoc, "description: \"Git tracked text file\"", "description: Git tracked text file - modified in bundle", 1)
	if err := os.WriteFile(gitDocFile, []byte(modifiedGitDoc), 0644); err != nil {
		t.Fatalf("failed to write modified file_git.txt.md: %v", err)
	}

	stderr.buf = nil
	cmdIngest := exec.Command(binaryPath, "ingest", "--repo", repoDir, "--bundle", bundleDir, "--sync")
	cmdIngest.Stderr = &stderr
	if err := cmdIngest.Run(); err != nil {
		t.Fatalf("git ingest command failed: %v. Stderr: %s", err, stderr.String())
	}

	// Verify .okf-metadata.yaml was updated
	newMetaBytes, err := os.ReadFile(filepath.Join(repoDir, ".okf-metadata.yaml"))
	if err != nil {
		t.Fatalf("failed to read updated .okf-metadata.yaml: %v", err)
	}
	if !strings.Contains(string(newMetaBytes), "Git tracked text file - modified in bundle") {
		t.Errorf(".okf-metadata.yaml was not updated: %s", string(newMetaBytes))
	}
}

// bytesBuffer is a helper to implement a thread-safe bytes.Buffer for stdout/stderr capturing.
type bytesBuffer struct {
	buf []byte
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bytesBuffer) String() string {
	return string(b.buf)
}
