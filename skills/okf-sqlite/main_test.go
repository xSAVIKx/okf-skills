package main

import (
	"reflect"
	"testing"
)

func TestParseColumnsFromMarkdown(t *testing.T) {
	markdown := `
# Columns

| Name | Type | Primary Key | Nullable | Default |
| --- | --- | --- | --- | --- |
| id | INTEGER | Yes | No |  |
| username | VARCHAR(50) | No | No |  |
| active | BOOLEAN | No | Yes | 1 |
`
	expected := []Column{
		{
			Name:       "id",
			Type:       "INTEGER",
			PrimaryKey: true,
			Nullable:   false,
			Default:    "",
		},
		{
			Name:       "username",
			Type:       "VARCHAR(50)",
			PrimaryKey: false,
			Nullable:   false,
			Default:    "",
		},
		{
			Name:       "active",
			Type:       "BOOLEAN",
			PrimaryKey: false,
			Nullable:   true,
			Default:    "1",
		},
	}

	cols := parseColumnsFromMarkdown(markdown)
	if !reflect.DeepEqual(cols, expected) {
		t.Errorf("expected %+v, got %+v", expected, cols)
	}
}

// TestParseColumnsFromMarkdown_IgnoresProfileAndSample guards against the parser
// reading the "## Data Profile" / "## Sample" tables (present when a bundle is
// produced with --profile/--sample) as if they were schema columns.
func TestParseColumnsFromMarkdown_IgnoresProfileAndSample(t *testing.T) {
	markdown := `
# Columns

| Name | Type | Primary Key | Nullable | Default |
| --- | --- | --- | --- | --- |
| order_id | INTEGER | Yes | Yes |  |
| status | TEXT | No | No |  |

## Data Profile

| Column | Non-Null | Null | Distinct | Min | Max |
| --- | --- | --- | --- | --- | --- |
| order_id | 5 | 0 | 5 | 1001 | 1005 |
| status | 5 | 0 | 3 | cancelled | shipped |

## Sample

| order_id | status |
| --- | --- |
| 1001 | shipped |
`
	expected := []Column{
		{Name: "order_id", Type: "INTEGER", PrimaryKey: true, Nullable: true, Default: ""},
		{Name: "status", Type: "TEXT", PrimaryKey: false, Nullable: false, Default: ""},
	}
	cols := parseColumnsFromMarkdown(markdown)
	if !reflect.DeepEqual(cols, expected) {
		t.Errorf("profile/sample rows leaked into parsed schema columns.\nexpected %+v\ngot      %+v", expected, cols)
	}
}
