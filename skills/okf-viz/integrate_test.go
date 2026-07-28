package main

import (
	"testing"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func diffModel(docs map[string]*okf.ConceptDoc) *Model {
	m := &Model{RootID: rootNodeID, concepts: docs}
	for id := range docs {
		m.Nodes = append(m.Nodes, Node{ID: id, Kind: "concept", Title: id})
	}
	return m
}

func TestComputeDiff(t *testing.T) {
	older := diffModel(map[string]*okf.ConceptDoc{
		"a": {Frontmatter: okf.Frontmatter{ContentHash: "h1"}},
		"b": {Frontmatter: okf.Frontmatter{ContentHash: "h2"}}, // will change
		"c": {Frontmatter: okf.Frontmatter{ContentHash: "h3"}}, // removed
	})
	newer := diffModel(map[string]*okf.ConceptDoc{
		"a": {Frontmatter: okf.Frontmatter{ContentHash: "h1"}},  // unchanged
		"b": {Frontmatter: okf.Frontmatter{ContentHash: "h2x"}}, // changed
		"d": {Frontmatter: okf.Frontmatter{ContentHash: "h4"}},  // added
	})
	ComputeDiff(newer, older)

	diff := map[string]string{}
	for _, n := range newer.Nodes {
		diff[n.ID] = n.Diff
	}
	if diff["a"] != "" {
		t.Errorf("a should be unchanged, got %q", diff["a"])
	}
	if diff["b"] != "changed" {
		t.Errorf("b should be changed, got %q", diff["b"])
	}
	if diff["d"] != "added" {
		t.Errorf("d should be added, got %q", diff["d"])
	}
	if diff["c"] != "removed" {
		t.Errorf("c should be injected as removed, got %q", diff["c"])
	}
}

func TestConceptChanged_BodyFallback(t *testing.T) {
	// no content_hash on either side -> body comparison.
	a := &okf.ConceptDoc{Body: "# Columns\n\n| id |\n"}
	b := &okf.ConceptDoc{Body: "# Columns\n\n| id |\n| email |\n"}
	if !conceptChanged(a, b) {
		t.Errorf("differing bodies should be detected as changed without a hash")
	}
	if conceptChanged(a, a) {
		t.Errorf("identical body should not be changed")
	}
}

func TestFederate_NamespacesIDs(t *testing.T) {
	primary := diffModel(map[string]*okf.ConceptDoc{"orders": {}})
	other := diffModel(map[string]*okf.ConceptDoc{"orders": {}}) // same ID, different bundle
	Federate("shopA", primary, map[string]*Model{"shopB": other})

	ids := map[string]string{} // id -> bundle
	for _, n := range primary.Nodes {
		ids[n.ID] = n.Bundle
	}
	if ids["shopA:orders"] != "shopA" || ids["shopB:orders"] != "shopB" {
		t.Fatalf("federation must namespace colliding IDs by bundle, got %v", ids)
	}
}
