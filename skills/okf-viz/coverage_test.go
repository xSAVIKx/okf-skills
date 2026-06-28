package main

import (
	"strings"
	"testing"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func coverageModel() *Model {
	m := &Model{RootID: rootNodeID, concepts: map[string]*okf.ConceptDoc{}}
	// enriched concept with a comment column (1 of 2 commented) and a good link
	m.Nodes = append(m.Nodes, Node{ID: "tables/orders", Kind: "concept", Type: "SQLite Table",
		Title: "orders", Description: "One row per order.", Degree: 2})
	m.concepts["tables/orders"] = &okf.ConceptDoc{Body: "# Columns\n\n| Name | Type | Comment |\n| --- | --- | --- |\n| id | INT | the id |\n| total | INT |  |\n\n[customers](/tables/customers.md) [ghost](/tables/ghost.md)\n"}
	// placeholder concept, orphan, missing type
	m.Nodes = append(m.Nodes, Node{ID: "tables/customers", Kind: "concept", Type: "SQLite Table",
		Title: "customers", Description: "SQLite table customers", Degree: 1})
	m.concepts["tables/customers"] = &okf.ConceptDoc{Body: "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INT |\n"}
	m.Nodes = append(m.Nodes, Node{ID: "tables/orphan", Kind: "concept", Type: "",
		Title: "orphan", Description: "", Degree: 0})
	m.concepts["tables/orphan"] = &okf.ConceptDoc{Body: "# Columns\n"}
	// non-concept nodes are ignored
	m.Nodes = append(m.Nodes, Node{ID: "tables", Kind: "directory", Title: "tables"})
	m.Nodes = append(m.Nodes, Node{ID: rootNodeID, Kind: "root", Title: "root"})
	return m
}

func TestComputeCoverage(t *testing.T) {
	c := ComputeCoverage(coverageModel())

	if c.TotalConcepts != 3 {
		t.Fatalf("TotalConcepts = %d, want 3", c.TotalConcepts)
	}
	if c.Placeholders != 2 { // "SQLite table customers" + empty orphan
		t.Fatalf("Placeholders = %d, want 2", c.Placeholders)
	}
	if c.EnrichedPct != round1(100.0/3.0) {
		t.Fatalf("EnrichedPct = %v, want %v", c.EnrichedPct, round1(100.0/3.0))
	}
	if c.ColumnsTotal != 2 || c.ColumnsCommented != 1 {
		t.Fatalf("columns = %d/%d, want 1/2", c.ColumnsCommented, c.ColumnsTotal)
	}
	if len(c.MissingType) != 1 || c.MissingType[0] != "tables/orphan" {
		t.Fatalf("MissingType = %v", c.MissingType)
	}
	if len(c.Orphans) != 1 || c.Orphans[0] != "tables/orphan" {
		t.Fatalf("Orphans = %v", c.Orphans)
	}
	if len(c.BrokenLinks) != 1 || !strings.Contains(c.BrokenLinks[0], "ghost") {
		t.Fatalf("BrokenLinks = %v (want the /tables/ghost.md link)", c.BrokenLinks)
	}
	// Unenriched concepts ranked by degree (desc): customers (1) before orphan (0).
	if len(c.EnrichFirst) != 2 || c.EnrichFirst[0] != "tables/customers" || c.EnrichFirst[1] != "tables/orphan" {
		t.Fatalf("EnrichFirst = %v, want [tables/customers tables/orphan]", c.EnrichFirst)
	}
}

func TestCommentStats_DataCellWithDashes(t *testing.T) {
	// A data cell whose value is "---" must NOT be mistaken for the table divider
	// and silently dropped from the column count.
	body := "# Columns\n\n| Name | Type | Comment |\n| --- | --- | --- |\n" +
		"| id | INT | the id |\n| status | TEXT | --- |\n"
	total, commented := commentStats(body)
	if total != 2 || commented != 2 {
		t.Fatalf("commentStats = %d/%d, want 2/2 (the '---' comment row must be counted)", commented, total)
	}
}

func TestComputeCoverage_Deterministic(t *testing.T) {
	a := ComputeCoverage(coverageModel()).Report()
	b := ComputeCoverage(coverageModel()).Report()
	if a != b {
		t.Fatalf("coverage report not deterministic:\n%s\n---\n%s", a, b)
	}
}
