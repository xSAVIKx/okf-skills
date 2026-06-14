package main

import "testing"

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
