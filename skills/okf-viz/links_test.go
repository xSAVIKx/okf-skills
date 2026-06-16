package main

import (
	"testing"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func TestRelationExtraction(t *testing.T) {
	m := &Model{concepts: map[string]*okf.ConceptDoc{}}
	add := func(id, body string) {
		m.Nodes = append(m.Nodes, Node{ID: id, Kind: "concept"})
		m.concepts[id] = &okf.ConceptDoc{Body: body}
	}
	add("a", "# Relationships\n\n- FK on x [b](b.md)\n\n# Joins\n\n[c](c.md)\n\n# Columns\n\n[d](d.md) and [fk](e.md)\n\n# Related Files\n\n[f](f.md)\n\n# See Also\n\n[g](g.md)\n")
	add("b", "")
	add("c", "")
	add("d", "")
	add("e", "")
	add("f", "")
	add("g", "")
	addCrossLinks(m)

	rel := map[string]string{}
	for _, e := range m.Edges {
		if e.Kind == "crosslink" {
			rel[e.Target] = e.Relation
		}
	}
	if rel["b"] != "references" {
		t.Errorf("link under # Relationships should be references, got %q", rel["b"])
	}
	if rel["c"] != "joins-with" {
		t.Errorf("link under # Joins should be joins-with, got %q", rel["c"])
	}
	if rel["d"] != "" {
		t.Errorf("link under # Columns should be a generic crosslink, got %q", rel["d"])
	}
	if rel["e"] != "references" {
		t.Errorf("[fk] annotation should override to references, got %q", rel["e"])
	}
	if rel["f"] != "co-changes" {
		t.Errorf("link under # Related Files should be co-changes, got %q", rel["f"])
	}
	if rel["g"] != "see-also" {
		t.Errorf("link under # See Also should be see-also, got %q", rel["g"])
	}
}

func TestRelationSortDeterministic(t *testing.T) {
	m := &Model{concepts: map[string]*okf.ConceptDoc{}}
	for _, id := range []string{"a", "b", "c"} {
		m.Nodes = append(m.Nodes, Node{ID: id, Kind: "concept"})
		m.concepts[id] = &okf.ConceptDoc{}
	}
	m.concepts["a"] = &okf.ConceptDoc{Body: "# Joins\n\n[b](b.md)\n\n# Relationships\n\n[c](c.md)\n"}
	addCrossLinks(m)
	// edges sorted by (Kind, Relation, Source, Target): joins-with < references.
	var rels []string
	for _, e := range m.Edges {
		if e.Kind == "crosslink" {
			rels = append(rels, e.Relation)
		}
	}
	if len(rels) != 2 || rels[0] != "joins-with" || rels[1] != "references" {
		t.Fatalf("edges not sorted by relation: %v", rels)
	}
}

func TestCrossLinks(t *testing.T) {
	m, err := BuildModel("testdata/linked")
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	addCrossLinks(m)

	var found, broken int
	for _, e := range m.Edges {
		if e.Kind != "crosslink" {
			continue
		}
		if e.Source == "tables/orders" && e.Target == "tables/customers" {
			found++
		}
		if e.Target == "tables/ghost" {
			broken++
		}
	}
	if found != 1 {
		t.Errorf("expected 1 orders->customers crosslink, got %d", found)
	}
	if broken != 0 {
		t.Errorf("broken link must produce no edge, got %d", broken)
	}
	// degree: customers has 1 containment (from tables) + 1 crosslink in = 2.
	deg := map[string]int{}
	for _, n := range m.Nodes {
		deg[n.ID] = n.Degree
	}
	if deg["tables/customers"] < 2 {
		t.Errorf("customers degree = %d, want >= 2", deg["tables/customers"])
	}
}

// TestEdgeOrderDeterministic verifies that addCrossLinks leaves m.Edges sorted
// by (Kind, Relation, Source, Target).  The fixture has two link-source concepts
// (orders→customers and customers→orders), so map iteration order in
// addCrossLinks would produce different orderings across runs without the sort.
// This test fails deterministically without the sort.Slice call in addCrossLinks.
func TestEdgeOrderDeterministic(t *testing.T) {
	m, err := BuildModel("testdata/linked")
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	addCrossLinks(m)

	for i := 1; i < len(m.Edges); i++ {
		prev, cur := m.Edges[i-1], m.Edges[i]
		// Mirror the comparator in addCrossLinks exactly: (Kind, Relation, Source,
		// Target). Omitting Relation here would let a Relation-ordering regression
		// pass unnoticed.
		less := func(a, b Edge) bool {
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
			}
			if a.Relation != b.Relation {
				return a.Relation < b.Relation
			}
			if a.Source != b.Source {
				return a.Source < b.Source
			}
			return a.Target < b.Target
		}
		if less(cur, prev) {
			t.Errorf("edges not sorted at index %d: %+v > %+v", i, prev, cur)
		}
	}
}
