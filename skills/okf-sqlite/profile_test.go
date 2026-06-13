package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, name) VALUES (1,'alice'),(2,'bob'),(3,NULL)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return db
}

func TestProfileTable(t *testing.T) {
	db := newTestDB(t)
	cols := []Column{{Name: "id"}, {Name: "name"}}
	profiles, err := profileTable(db, "users", cols)
	if err != nil {
		t.Fatalf("profileTable: %v", err)
	}
	byName := map[string]int64{}
	nullByName := map[string]int64{}
	for _, p := range profiles {
		byName[p.Column] = p.NonNull
		nullByName[p.Column] = p.Null
	}
	if byName["id"] != 3 {
		t.Fatalf("id non-null = %d, want 3", byName["id"])
	}
	if byName["name"] != 2 || nullByName["name"] != 1 {
		t.Fatalf("name non-null/null = %d/%d, want 2/1", byName["name"], nullByName["name"])
	}
}

func TestSampleTable(t *testing.T) {
	db := newTestDB(t)
	headers, rows, err := sampleTable(db, "users", 2)
	if err != nil {
		t.Fatalf("sampleTable: %v", err)
	}
	if len(headers) != 2 {
		t.Fatalf("headers = %v, want 2 columns", headers)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (LIMIT 2)", len(rows))
	}
}
