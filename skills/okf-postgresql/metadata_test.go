package main

import "testing"

func TestDetectFreshnessColumn(t *testing.T) {
	// Falls back to a date/time-ish *name* when no type matches.
	cols := []ColumnSpec{
		{Name: "id", Type: "integer"},
		{Name: "created_at", Type: "text"},
	}
	if got := detectFreshnessColumn(cols); got != "created_at" {
		t.Fatalf("expected created_at by name, got %q", got)
	}

	// Prefers a date/time *type* over a name match.
	typed := []ColumnSpec{
		{Name: "id", Type: "integer"},
		{Name: "ts", Type: "timestamp without time zone"},
		{Name: "updated_at", Type: "text"},
	}
	if got := detectFreshnessColumn(typed); got != "ts" {
		t.Fatalf("expected ts by type, got %q", got)
	}

	// No timestamp-ish column at all.
	if got := detectFreshnessColumn([]ColumnSpec{{Name: "id", Type: "integer"}}); got != "" {
		t.Fatalf("expected no freshness column, got %q", got)
	}
}

func TestQuoteIdent(t *testing.T) {
	if got := quoteIdent("public"); got != `"public"` {
		t.Fatalf("unexpected quoteIdent: %q", got)
	}
	if got := quoteIdent(`we"ird`); got != `"we""ird"` {
		t.Fatalf("expected embedded quote to be doubled, got %q", got)
	}
}
