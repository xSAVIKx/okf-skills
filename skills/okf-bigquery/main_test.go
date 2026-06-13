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
