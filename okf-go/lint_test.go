package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBundleFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanBundle_Coverage(t *testing.T) {
	dir := t.TempDir()
	writeBundleFile(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	writeBundleFile(t, dir, "tables/index.md", "# Tables\n")
	// enriched concept, comment column 1/2 commented, one good + one broken link
	writeBundleFile(t, dir, "tables/orders.md",
		"---\ntype: SQLite Table\ntitle: orders\ndescription: One row per order.\n---\n# Columns\n\n| Name | Type | Comment |\n| --- | --- | --- |\n| id | INT | the id |\n| total | INT |  |\n\n[customers](/tables/customers.md) [ghost](/tables/ghost.md)\n")
	// placeholder concept, linked from orders (not an orphan)
	writeBundleFile(t, dir, "tables/customers.md",
		"---\ntype: SQLite Table\ntitle: customers\ndescription: SQLite table customers\n---\n# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INT |\n")
	// placeholder, missing type, no links -> orphan + missing-type
	writeBundleFile(t, dir, "tables/orphan.md", "---\ntype: \ntitle: orphan\ndescription: \n---\n# Columns\n")

	r, err := ScanBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.TotalConcepts != 3 {
		t.Errorf("TotalConcepts = %d, want 3", r.TotalConcepts)
	}
	if r.Placeholders != 2 {
		t.Errorf("Placeholders = %d, want 2", r.Placeholders)
	}
	if r.ColumnsTotal != 2 || r.ColumnsCommented != 1 {
		t.Errorf("columns = %d/%d, want 1/2", r.ColumnsCommented, r.ColumnsTotal)
	}
	if len(r.MissingType) != 1 || r.MissingType[0] != "tables/orphan" {
		t.Errorf("MissingType = %v", r.MissingType)
	}
	if len(r.BrokenLinks) != 1 || !strings.Contains(r.BrokenLinks[0], "ghost") {
		t.Errorf("BrokenLinks = %v", r.BrokenLinks)
	}
	if len(r.Orphans) != 1 || r.Orphans[0] != "tables/orphan" {
		t.Errorf("Orphans = %v (customers is linked, should not be orphan)", r.Orphans)
	}
	// placeholders ranked by degree desc: customers (1) before orphan (0)
	if len(r.EnrichFirst) != 2 || r.EnrichFirst[0] != "tables/customers" || r.EnrichFirst[1] != "tables/orphan" {
		t.Errorf("EnrichFirst = %v", r.EnrichFirst)
	}
	// only conformance issue: missing type on orphan
	if len(r.Conformance) != 1 || r.Conformance[0].Rule != RuleMissingType {
		t.Errorf("Conformance = %+v, want one missing-type", r.Conformance)
	}
	// report contains the substrings okf-viz's integration test relies on
	rep := r.TextReport()
	if !strings.Contains(rep, "enriched") || !strings.Contains(rep, "placeholders: 2") {
		t.Errorf("TextReport missing expected substrings:\n%s", rep)
	}
}

func TestScanBundle_Determinism(t *testing.T) {
	dir := t.TempDir()
	writeBundleFile(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	writeBundleFile(t, dir, "tables/a.md", "---\ntype: T\ndescription: SQLite table a\n---\n[b](/tables/b.md)\n")
	writeBundleFile(t, dir, "tables/b.md", "---\ntype: T\ndescription: SQLite table b\n---\n[a](/tables/a.md)\n")
	a, _ := ScanBundle(dir)
	b, _ := ScanBundle(dir)
	if a.TextReport() != b.TextReport() {
		t.Fatalf("non-deterministic report:\n%s\n---\n%s", a.TextReport(), b.TextReport())
	}
}

func TestScanBundle_Conformance(t *testing.T) {
	dir := t.TempDir()
	// root index with an extra key + still has okf_version
	writeBundleFile(t, dir, "index.md", "---\nokf_version: \"0.1\"\ntitle: oops\n---\n# Demo\n")
	// subdir index WITH frontmatter (violation)
	writeBundleFile(t, dir, "tables/index.md", "---\ntype: nope\n---\n# Tables\n")
	// a concept with no frontmatter at all (unparseable)
	writeBundleFile(t, dir, "tables/bad.md", "# Just a heading, no frontmatter\n")

	r, err := ScanBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]bool{}
	for _, f := range r.Conformance {
		rules[f.Rule] = true
	}
	for _, want := range []string{RuleRootIndexExtraKeys, RuleSubdirIndexFM, RuleConceptUnparseable} {
		if !rules[want] {
			t.Errorf("missing conformance rule %q in %+v", want, r.Conformance)
		}
	}
	// bad.md must not count as a valid concept
	if r.TotalConcepts != 0 {
		t.Errorf("TotalConcepts = %d, want 0 (bad.md is unparseable)", r.TotalConcepts)
	}
}

func TestScanBundle_RootIndexMissingVersion(t *testing.T) {
	dir := t.TempDir()
	writeBundleFile(t, dir, "index.md", "# Demo, no frontmatter at all\n")
	r, _ := ScanBundle(dir)
	found := false
	for _, f := range r.Conformance {
		if f.Rule == RuleRootIndexVersion {
			found = true
		}
	}
	if !found {
		t.Errorf("expected root-index-okf-version finding, got %+v", r.Conformance)
	}
}
