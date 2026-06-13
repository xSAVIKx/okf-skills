package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/savikne/okf-skills-registry/okf-go"
)

type fakeGen struct {
	calls int
	last  string
}

func (f *fakeGen) Describe(ctx context.Context, prompt string) (string, error) {
	f.calls++
	f.last = prompt
	return "Generated description.", nil
}

func TestNeedsDescription(t *testing.T) {
	if !needsDescription("", false) {
		t.Fatal("empty should need description")
	}
	if needsDescription("x", false) {
		t.Fatal("non-empty without overwrite should not")
	}
	if !needsDescription("x", true) {
		t.Fatal("overwrite should force")
	}
}

func TestEnrichBundle_FillsEmptyDescriptions(t *testing.T) {
	dir := t.TempDir()
	tablesDir := filepath.Join(dir, "tables")
	if err := os.MkdirAll(tablesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := okf.WriteConceptDoc(filepath.Join(tablesDir, "users.md"), okf.ConceptDoc{
		Frontmatter: okf.Frontmatter{Type: "SQLite Table", Title: "users"},
		Body:        "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INTEGER |\n",
	}); err != nil {
		t.Fatal(err)
	}
	if err := okf.WriteConceptDoc(filepath.Join(dir, "index.md"), okf.ConceptDoc{
		Frontmatter: okf.Frontmatter{OKFVersion: "0.1"}, Body: "# Index\n",
	}); err != nil {
		t.Fatal(err)
	}
	g := &fakeGen{}
	n, err := enrichBundle(context.Background(), dir, g, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("changed=%d, want 1", n)
	}
	if g.calls != 1 {
		t.Fatalf("gen calls=%d, want 1 (index.md must be skipped)", g.calls)
	}
	doc, err := okf.ReadConceptDoc(filepath.Join(tablesDir, "users.md"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter.Description != "Generated description." {
		t.Fatalf("description=%q", doc.Frontmatter.Description)
	}
	if !strings.Contains(g.last, "users") {
		t.Fatalf("prompt missing title: %q", g.last)
	}
}

func TestEnrichBundle_SkipsExistingUnlessOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tables"), 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "tables", "x.md")
	if err := okf.WriteConceptDoc(p, okf.ConceptDoc{
		Frontmatter: okf.Frontmatter{Title: "x", Description: "existing"}, Body: "b",
	}); err != nil {
		t.Fatal(err)
	}
	g := &fakeGen{}
	n, _ := enrichBundle(context.Background(), dir, g, false)
	if n != 0 || g.calls != 0 {
		t.Fatalf("should skip existing: n=%d calls=%d", n, g.calls)
	}
	n, _ = enrichBundle(context.Background(), dir, g, true)
	if n != 1 || g.calls != 1 {
		t.Fatalf("overwrite should enrich: n=%d calls=%d", n, g.calls)
	}
}
