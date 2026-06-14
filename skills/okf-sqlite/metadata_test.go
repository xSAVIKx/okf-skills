package main

import "testing"

func TestParseCheckConstraints(t *testing.T) {
	sql := "CREATE TABLE t (id INTEGER, age INTEGER CHECK (age >= 0), score INT, CHECK (score <= 100))"
	cons := parseCheckConstraints(sql)
	if len(cons) != 2 {
		t.Fatalf("expected 2 check constraints, got %d: %+v", len(cons), cons)
	}
	if cons[0].Type != "CHECK" || cons[0].Definition != "age >= 0" {
		t.Fatalf("unexpected first check constraint: %+v", cons[0])
	}
	if cons[1].Definition != "score <= 100" {
		t.Fatalf("unexpected second check constraint: %+v", cons[1])
	}
}

func TestParseCheckConstraints_None(t *testing.T) {
	if cons := parseCheckConstraints("CREATE TABLE t (id INTEGER)"); len(cons) != 0 {
		t.Fatalf("expected no check constraints, got %+v", cons)
	}
}

func TestExtractBalanced_Nested(t *testing.T) {
	expr, end, ok := extractBalanced("(a AND (b OR c))x", 0)
	if !ok || expr != "a AND (b OR c)" {
		t.Fatalf("unexpected balanced extraction: %q ok=%v", expr, ok)
	}
	if end != 16 {
		t.Fatalf("unexpected end index %d", end)
	}
}

func TestDetectFreshnessColumn(t *testing.T) {
	cols := []Column{
		{Name: "id", Type: "INTEGER"},
		{Name: "created_at", Type: "TEXT"},
	}
	if got := detectFreshnessColumn(cols); got != "created_at" {
		t.Fatalf("expected created_at by name, got %q", got)
	}

	typed := []Column{
		{Name: "id", Type: "INTEGER"},
		{Name: "ts", Type: "TIMESTAMP"},
		{Name: "updated_at", Type: "TEXT"},
	}
	if got := detectFreshnessColumn(typed); got != "ts" {
		t.Fatalf("expected ts by type, got %q", got)
	}

	if got := detectFreshnessColumn([]Column{{Name: "id", Type: "INTEGER"}}); got != "" {
		t.Fatalf("expected no freshness column, got %q", got)
	}
}
