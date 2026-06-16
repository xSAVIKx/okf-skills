package main

import "testing"

func TestDetectFreshnessColumn(t *testing.T) {
	// Name-based detection when no date/time type is present.
	cols := []ColumnSpec{
		{Name: "id", Type: "int"},
		{Name: "created_at", Type: "varchar(32)"},
	}
	if got := detectFreshnessColumn(cols); got != "created_at" {
		t.Fatalf("expected created_at by name, got %q", got)
	}

	// Type-based detection takes precedence over a name-only match.
	typed := []ColumnSpec{
		{Name: "id", Type: "int"},
		{Name: "ts", Type: "timestamp"},
		{Name: "updated_at", Type: "varchar(32)"},
	}
	if got := detectFreshnessColumn(typed); got != "ts" {
		t.Fatalf("expected ts by type, got %q", got)
	}

	// A datetime type column is detected even with a non-date-ish name.
	dt := []ColumnSpec{
		{Name: "id", Type: "int"},
		{Name: "logged", Type: "datetime"},
	}
	if got := detectFreshnessColumn(dt); got != "logged" {
		t.Fatalf("expected logged by type, got %q", got)
	}

	// _on suffix is recognised by name.
	onCol := []ColumnSpec{
		{Name: "id", Type: "int"},
		{Name: "processed_on", Type: "varchar(32)"},
	}
	if got := detectFreshnessColumn(onCol); got != "processed_on" {
		t.Fatalf("expected processed_on by name, got %q", got)
	}

	// No timestamp-ish column at all.
	if got := detectFreshnessColumn([]ColumnSpec{{Name: "id", Type: "int"}}); got != "" {
		t.Fatalf("expected no freshness column, got %q", got)
	}
}
