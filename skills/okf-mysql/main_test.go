package main

import (
	"reflect"
	"testing"
)

func TestParseColumnsFromMarkdown(t *testing.T) {
	markdown := `
# Columns

| Name | Type | Key | Nullable | Default | Extra | Comment |
| --- | --- | --- | --- | --- | --- | --- |
| id | int | PRI | No |  | auto_increment | Unique identifier |
| name | varchar(100) |  | No |  |  | Customer name |
| discount | decimal(10,2) |  | Yes | 0.00 |  | Default discount rate |
`
	expected := []ColumnSpec{
		{
			Name:     "id",
			Type:     "int",
			Key:      "PRI",
			Nullable: false,
			Default:  "",
			Extra:    "auto_increment",
			Comment:  "Unique identifier",
		},
		{
			Name:     "name",
			Type:     "varchar(100)",
			Key:      "",
			Nullable: false,
			Default:  "",
			Extra:    "",
			Comment:  "Customer name",
		},
		{
			Name:     "discount",
			Type:     "decimal(10,2)",
			Key:      "",
			Nullable: true,
			Default:  "0.00",
			Extra:    "",
			Comment:  "Default discount rate",
		},
	}

	cols := parseColumnsFromMarkdown(markdown)
	if !reflect.DeepEqual(cols, expected) {
		t.Errorf("expected %+v, got %+v", expected, cols)
	}
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello World"},
		{"It's fine", "It''s fine"},
		{"Path\\To\\File", "Path\\\\To\\\\File"},
		{"Quote ' and Backslash \\ combined", "Quote '' and Backslash \\\\ combined"},
	}

	for _, test := range tests {
		actual := escapeString(test.input)
		if actual != test.expected {
			t.Errorf("escapeString(%q): expected %q, got %q", test.input, test.expected, actual)
		}
	}
}

// TestParseColumnsFromMarkdown_IgnoresProfileAndSample guards against the parser
// reading the "## Data Profile" / "## Sample" tables (present when a bundle is
// produced with --profile/--sample) as if they were schema columns.
func TestParseColumnsFromMarkdown_IgnoresProfileAndSample(t *testing.T) {
	markdown := `
# Columns

| Name | Type | Key | Nullable | Default | Extra | Comment |
| --- | --- | --- | --- | --- | --- | --- |
| id | int | PRI | No |  | auto_increment | Unique identifier |
| name | varchar(100) |  | Yes |  |  | Customer name |

## Data Profile

| Column | Non-Null | Null | Distinct | Min | Max |
| --- | --- | --- | --- | --- | --- |
| id | 5 | 0 | 5 | 1 | 5 |
| name | 4 | 1 | 4 | Ada | Zed |

## Sample

| id | name |
| --- | --- |
| 1 | Ada |
`
	expected := []ColumnSpec{
		{Name: "id", Type: "int", Key: "PRI", Nullable: false, Default: "", Extra: "auto_increment", Comment: "Unique identifier"},
		{Name: "name", Type: "varchar(100)", Key: "", Nullable: true, Default: "", Extra: "", Comment: "Customer name"},
	}
	cols := parseColumnsFromMarkdown(markdown)
	if !reflect.DeepEqual(cols, expected) {
		t.Errorf("profile/sample rows leaked into parsed schema columns.\nexpected %+v\ngot      %+v", expected, cols)
	}
}
