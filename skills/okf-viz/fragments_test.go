package main

import (
	"strings"
	"testing"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func TestFragmentPaths_NoHiddenSegments(t *testing.T) {
	docs := map[string]Doc{
		"tables/orders":            {},
		".github":                  {},
		".github/workflows/ci.yml": {},
		"a/.hidden/b":              {},
		"_.github":                 {}, // would collide with the mapped ".github"
	}
	paths := fragmentPaths(docs)
	want := map[string]string{
		"tables/orders":            "_okf/tables/orders.html",
		".github":                  "_okf/_.github.html",
		".github/workflows/ci.yml": "_okf/_.github/workflows/ci.yml.html",
		"a/.hidden/b":              "_okf/a/_.hidden/b.html",
		"_.github":                 "_okf/_.github-2.html",
	}
	for id, w := range want {
		if paths[id] != w {
			t.Errorf("fragment path for %q = %q, want %q", id, paths[id], w)
		}
	}
	seen := map[string]string{}
	for id, p := range paths {
		if other, dup := seen[p]; dup {
			t.Errorf("concepts %q and %q share fragment path %q", id, other, p)
		}
		seen[p] = id
		for _, seg := range strings.Split(p, "/") {
			if strings.HasPrefix(seg, ".") {
				t.Errorf("fragment path %q has a hidden segment", p)
			}
		}
	}
}

func TestEmit_LazyFragmentsAvoidHiddenPaths(t *testing.T) {
	m := &Model{RootID: rootNodeID, concepts: map[string]*okf.ConceptDoc{
		".github/workflows/ci.yml": {Body: "ci body"},
		"README.md":                {Body: "readme body"},
	}}
	m.Nodes = []Node{
		{ID: ".github/workflows/ci.yml", Kind: "concept"},
		{ID: "README.md", Kind: "concept"},
		{ID: rootNodeID, Kind: "root"},
	}
	html, frags, err := Emit(m, EmitOptions{Title: "t", Threshold: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frags["_okf/_.github/workflows/ci.yml.html"], "ci body") {
		t.Errorf("hidden-path concept must get a published fragment path, got %v", frags)
	}
	if !strings.Contains(html, `".github/workflows/ci.yml":"_okf/_.github/workflows/ci.yml.html"`) {
		t.Errorf("manifest must map the concept ID to the mapped fragment path")
	}
	if _, ok := frags["_okf/README.md.html"]; !ok {
		t.Errorf("ordinary concept fragment path must not change, got %v", frags)
	}
}
