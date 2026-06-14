package main

import (
	"testing"
)

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
// by (Kind, Source, Target).  The fixture has two link-source concepts
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
		// Compare (Kind, Source, Target) lexicographically.
		less := func(a, b Edge) bool {
			if a.Kind != b.Kind {
				return a.Kind < b.Kind
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
