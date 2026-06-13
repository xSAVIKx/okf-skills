package main

import (
	"reflect"
	"testing"
)

func TestParseColumnsFromMarkdown(t *testing.T) {
	markdown := `
# Columns

| Name | Type | Required | Description |
| --- | --- | --- | --- |
| event_id | STRING | Required | Unique event UUID |
| user_id | INT64 | Nullable | Event sender ID |
| tags | ARRAY<STRING> | Repeated | Category tags |
`
	expected := []ColumnSpec{
		{
			Name:        "event_id",
			Type:        "STRING",
			Required:    "Required",
			Description: "Unique event UUID",
		},
		{
			Name:        "user_id",
			Type:        "INT64",
			Required:    "Nullable",
			Description: "Event sender ID",
		},
		{
			Name:        "tags",
			Type:        "ARRAY<STRING>",
			Required:    "Repeated",
			Description: "Category tags",
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

| Name | Type | Required | Description |
| --- | --- | --- | --- |
| event_id | STRING | Required | Unique event UUID |
| user_id | INT64 | Nullable | Event sender ID |

## Data Profile

| Column | Non-Null | Null | Distinct | Min | Max |
| --- | --- | --- | --- | --- | --- |
| event_id | 5 | 0 | 5 | a | z |
| user_id | 4 | 1 | 4 | 1 | 9 |

## Sample

| event_id | user_id |
| --- | --- |
| a | 1 |
`
	expected := []ColumnSpec{
		{Name: "event_id", Type: "STRING", Required: "Required", Description: "Unique event UUID"},
		{Name: "user_id", Type: "INT64", Required: "Nullable", Description: "Event sender ID"},
	}
	cols := parseColumnsFromMarkdown(markdown)
	if !reflect.DeepEqual(cols, expected) {
		t.Errorf("profile/sample rows leaked into parsed schema columns.\nexpected %+v\ngot      %+v", expected, cols)
	}
}
