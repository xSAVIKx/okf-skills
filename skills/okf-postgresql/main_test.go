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
