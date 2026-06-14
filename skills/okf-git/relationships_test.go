package main

import (
	"strings"
	"testing"
)

func newIndex(pairs map[string]map[string]int) *coChangeIndex {
	return &coChangeIndex{counts: pairs}
}

func TestRelationshipsFor_RanksByFrequencyAndAppliesMin(t *testing.T) {
	idx := newIndex(map[string]map[string]int{
		"a.go": {"b.go": 5, "c.go": 2, "d.go": 1},
	})
	rels := idx.relationshipsFor("a.go", 2, 0, nil)
	if len(rels) != 2 {
		t.Fatalf("expected 2 partners above min, got %d: %+v", len(rels), rels)
	}
	// d.go (count 1) is below min and must be excluded.
	for _, r := range rels {
		if r.Text == "d.go" {
			t.Fatalf("d.go should be below min threshold: %+v", rels)
		}
	}
	// Targets are bundle-relative concept links.
	if rels[0].Target != "/b.go.md" && rels[1].Target != "/b.go.md" {
		t.Fatalf("expected a /b.go.md target, got %+v", rels)
	}
}

func TestRelationshipsFor_TopNCap(t *testing.T) {
	idx := newIndex(map[string]map[string]int{
		"a.go": {"b.go": 5, "c.go": 4, "d.go": 3},
	})
	rels := idx.relationshipsFor("a.go", 1, 2, nil)
	if len(rels) != 2 {
		t.Fatalf("expected topN=2 partners, got %d: %+v", len(rels), rels)
	}
	// The two highest-frequency partners (b.go=5, c.go=4) survive; d.go is cut.
	joined := rels[0].Text + " " + rels[1].Text
	if strings.Contains(joined, "d.go") {
		t.Fatalf("lowest-frequency partner should be cut by topN: %+v", rels)
	}
}

func TestRelationshipsFor_ExistsFilter(t *testing.T) {
	idx := newIndex(map[string]map[string]int{
		"a.go": {"b.go": 5, "gone.go": 5},
	})
	exists := func(c string) bool { return c != "gone.go" }
	rels := idx.relationshipsFor("a.go", 1, 0, exists)
	if len(rels) != 1 || rels[0].Text != "b.go" {
		t.Fatalf("exists filter should drop gone.go, got %+v", rels)
	}
}

func TestRelationshipsFor_NoPartners(t *testing.T) {
	idx := newIndex(map[string]map[string]int{})
	if rels := idx.relationshipsFor("a.go", 1, 0, nil); rels != nil {
		t.Fatalf("expected nil for file with no partners, got %+v", rels)
	}
}

func TestAdd_SymmetricCounting(t *testing.T) {
	idx := &coChangeIndex{counts: map[string]map[string]int{}}
	idx.add("a", "b")
	idx.add("b", "a")
	if idx.counts["a"]["b"] != 1 || idx.counts["b"]["a"] != 1 {
		t.Fatalf("expected symmetric counts, got %+v", idx.counts)
	}
}
