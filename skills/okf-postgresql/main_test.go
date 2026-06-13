package main

import (
	"reflect"
	"testing"
)

func TestParseColumnsFromMarkdown(t *testing.T) {
	markdown := `
# Columns

| Name | Type | Nullable | Comment |
| --- | --- | --- | --- |
| id | integer | No | Unique identifier |
| email | character varying(255) | No | User email address |
| bio | text | Yes | Brief bio |
`
	expected := []ColumnSpec{
		{
			Name:     "id",
			Type:     "integer",
			Nullable: false,
			Comment:  "Unique identifier",
		},
		{
			Name:     "email",
			Type:     "character varying(255)",
			Nullable: false,
			Comment:  "User email address",
		},
		{
			Name:     "bio",
			Type:     "text",
			Nullable: true,
			Comment:  "Brief bio",
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

| Name | Type | Nullable | Comment |
| --- | --- | --- | --- |
| id | integer | No | Unique identifier |
| email | text | No | User email |

## Data Profile

| Column | Non-Null | Null | Distinct | Min | Max |
| --- | --- | --- | --- | --- | --- |
| id | 5 | 0 | 5 | 1 | 5 |
| email | 5 | 0 | 5 | a@x | z@x |

## Sample

| id | email |
| --- | --- |
| 1 | a@x |
`
	expected := []ColumnSpec{
		{Name: "id", Type: "integer", Nullable: false, Comment: "Unique identifier"},
		{Name: "email", Type: "text", Nullable: false, Comment: "User email"},
	}
	cols := parseColumnsFromMarkdown(markdown)
	if !reflect.DeepEqual(cols, expected) {
		t.Errorf("profile/sample rows leaked into parsed schema columns.\nexpected %+v\ngot      %+v", expected, cols)
	}
}
