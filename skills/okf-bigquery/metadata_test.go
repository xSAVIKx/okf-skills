package main

import (
	"testing"

	"cloud.google.com/go/bigquery"
)

func TestDetectFreshnessColumn(t *testing.T) {
	// Picks a date/time-ish column by name when no typed timestamp exists.
	byName := bigquery.Schema{
		{Name: "id", Type: bigquery.IntegerFieldType},
		{Name: "created_at", Type: bigquery.StringFieldType},
	}
	if got := detectFreshnessColumn(byName); got != "created_at" {
		t.Fatalf("expected created_at by name, got %q", got)
	}

	// Prefers a typed timestamp column over a name-only match.
	byType := bigquery.Schema{
		{Name: "id", Type: bigquery.IntegerFieldType},
		{Name: "ts", Type: bigquery.TimestampFieldType},
		{Name: "updated_at", Type: bigquery.StringFieldType},
	}
	if got := detectFreshnessColumn(byType); got != "ts" {
		t.Fatalf("expected ts by type, got %q", got)
	}

	// DATE / DATETIME types are also recognized by type.
	dateType := bigquery.Schema{
		{Name: "id", Type: bigquery.IntegerFieldType},
		{Name: "d", Type: bigquery.DateFieldType},
	}
	if got := detectFreshnessColumn(dateType); got != "d" {
		t.Fatalf("expected d by date type, got %q", got)
	}

	// No timestamp-ish column at all -> empty string.
	none := bigquery.Schema{
		{Name: "id", Type: bigquery.IntegerFieldType},
		{Name: "label", Type: bigquery.StringFieldType},
	}
	if got := detectFreshnessColumn(none); got != "" {
		t.Fatalf("expected no freshness column, got %q", got)
	}
}
