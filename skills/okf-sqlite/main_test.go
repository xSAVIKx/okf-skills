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
