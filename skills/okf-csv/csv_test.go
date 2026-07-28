package main

import (
	"strings"
	"testing"
)

func TestInferType(t *testing.T) {
	cases := []struct {
		name    string
		samples []string
		want    string
	}{
		{"integer", []string{"1", "2", "300"}, "integer"},
		{"number", []string{"1.5", "2", "3.14"}, "number"},
		{"boolean", []string{"true", "false", "yes", "no"}, "boolean"},
		{"date", []string{"2024-01-02", "2025-12-31"}, "date"},
		{"string", []string{"a", "b", "c"}, "string"},
		{"mixed -> string", []string{"1", "two", "3"}, "string"},
		{"empty -> string", []string{"", "  ", ""}, "string"},
		{"ints with blanks ignored", []string{"1", "", "2"}, "integer"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inferType(c.samples); got != c.want {
				t.Errorf("inferType(%v) = %q, want %q", c.samples, got, c.want)
			}
		})
	}
}

func TestColumnTypes(t *testing.T) {
	header := []string{"id", "price", "active", "name"}
	rows := [][]string{
		{"1", "1.50", "true", "alice"},
		{"2", "2.00", "false", "bob"},
	}
	got := columnTypes(header, rows)
	want := []string{"integer", "number", "boolean", "string"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %q type = %q, want %q", header[i], got[i], want[i])
		}
	}
}

func TestColumnProfiles(t *testing.T) {
	header := []string{"id", "status"}
	rows := [][]string{
		{"10", "active"},
		{"2", "pending"},
		{"30", ""}, // null in status
	}
	types := []string{"integer", "string"}
	profs := columnProfiles(header, rows, types)

	// id: numeric min/max should be numeric, not lexical ("10" < "2" lexically)
	if profs[0].Min != "2" || profs[0].Max != "30" {
		t.Errorf("id min/max = %q/%q, want 2/30 (numeric)", profs[0].Min, profs[0].Max)
	}
	if profs[0].NonNull != 3 || profs[0].Null != 0 {
		t.Errorf("id nonnull/null = %d/%d, want 3/0", profs[0].NonNull, profs[0].Null)
	}
	// status: 1 null, 2 distinct, low cardinality -> Values populated
	if profs[1].Null != 1 || profs[1].Distinct != 2 {
		t.Errorf("status null/distinct = %d/%d, want 1/2", profs[1].Null, profs[1].Distinct)
	}
	if strings.Join(profs[1].Values, ",") != "active,pending" {
		t.Errorf("status values = %v, want [active pending]", profs[1].Values)
	}
}

func TestRenderColumns(t *testing.T) {
	out := renderColumns([]string{"id", "name"}, []string{"integer", "string"})
	for _, want := range []string{"# Columns", "| Name | Type |", "| id | integer |", "| name | string |"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderColumns missing %q:\n%s", want, out)
		}
	}
}

func TestConceptPathRoundTrip(t *testing.T) {
	cases := []string{"orders.csv", "sub/customers.csv", "a/b/c.csv"}
	for _, asset := range cases {
		concept := csvConceptPath(asset)
		if !strings.HasPrefix(concept, "tables/") || !strings.HasSuffix(concept, ".md") {
			t.Errorf("csvConceptPath(%q) = %q, want tables/...md", asset, concept)
		}
		if back := csvAssetPath(concept); back != asset {
			t.Errorf("round-trip %q -> %q -> %q", asset, concept, back)
		}
	}
}

func TestColumnsMatch(t *testing.T) {
	body := "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | integer |\n| name | string |\n"
	if !columnsMatch(body, []string{"id", "name"}) {
		t.Error("columnsMatch should be true for matching header")
	}
	if columnsMatch(body, []string{"id", "email"}) {
		t.Error("columnsMatch should be false on header drift")
	}
	if columnsMatch(body, []string{"id"}) {
		t.Error("columnsMatch should be false on column count drift")
	}
}
