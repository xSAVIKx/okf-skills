package main

import "testing"

func TestBuildModel_LinklessStaysConnected(t *testing.T) {
	m, err := BuildModel("testdata/linkless")
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	// Concept nodes for notes/a and notes/b; a directory node for notes; a root node.
	ids := map[string]Node{}
	for _, n := range m.Nodes {
		ids[n.ID] = n
	}
	for _, want := range []string{"notes/a", "notes/b"} {
		if ids[want].Kind != "concept" {
			t.Errorf("expected concept node %q", want)
		}
	}
	if ids["notes"].Kind != "directory" {
		t.Errorf("expected a directory node for notes")
	}
	// Every concept node is reachable (has at least one containment edge) — no orphans.
	hasParent := map[string]bool{}
	for _, e := range m.Edges {
		if e.Kind == "containment" {
			hasParent[e.Target] = true
		}
	}
	for _, want := range []string{"notes", "notes/a", "notes/b"} {
		if !hasParent[want] {
			t.Errorf("node %q has no containment parent (orphan)", want)
		}
	}
}
