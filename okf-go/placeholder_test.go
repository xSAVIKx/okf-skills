package okf

import "testing"

func TestIsPlaceholderDescription(t *testing.T) {
	placeholders := []string{
		"",
		"   ",
		"SQLite table orders",
		"SQLite view active_users",
		"MySQL table products",
		"PostgreSQL table users",
		"BigQuery table events",
		"MongoDB collection orders",
		"File config.yaml",
		"Directory src",
		"Git file main.go",
		"Git directory internal",
		"CSV file orders",
		"API endpoint GET /orders",
		"API schema Order",
		"No description available",
	}
	for _, p := range placeholders {
		if !IsPlaceholderDescription(p) {
			t.Errorf("expected placeholder for %q", p)
		}
	}

	real := []string{
		"One row per customer order with line-item totals and payment status.",
		"the SQLite table that stores orders",       // near-miss: not anchored at start
		"Stores files uploaded by users",            // starts with "Stores", not "File "
		"Directories of record — the org hierarchy", // "Directories" != "Directory "
		"Catalog of products available for purchase",
	}
	for _, r := range real {
		if IsPlaceholderDescription(r) {
			t.Errorf("real description misclassified as placeholder: %q", r)
		}
	}
}
