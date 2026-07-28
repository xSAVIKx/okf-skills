package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeLintBundle writes a file into a bundle under dir.
func writeLintBundle(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOkfLintCleanBundle(t *testing.T) {
	bin := getBinaryPath("okf-lint")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-lint binary not built: %v", err)
	}
	dir := t.TempDir()
	writeLintBundle(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	writeLintBundle(t, dir, "tables/orders.md",
		"---\ntype: SQLite Table\ntitle: orders\ndescription: One row per order.\n---\n# Columns\n\nFK to [customers](/tables/customers.md).\n")
	writeLintBundle(t, dir, "tables/customers.md",
		"---\ntype: SQLite Table\ntitle: customers\ndescription: One row per customer.\n---\n# Columns\n")

	// Clean bundle: lint passes (exit 0). --min 100 because both concepts are enriched.
	if out, err := exec.Command(bin, "lint", "--bundle", dir, "--min", "100").CombinedOutput(); err != nil {
		t.Fatalf("clean bundle should pass, got error %v:\n%s", err, out)
	}
}

func TestOkfLintGatesAndConformance(t *testing.T) {
	bin := getBinaryPath("okf-lint")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-lint binary not built: %v", err)
	}
	dir := t.TempDir()
	writeLintBundle(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	// subdir index WITH frontmatter (spec violation)
	writeLintBundle(t, dir, "tables/index.md", "---\ntype: bad\n---\n# Tables\n")
	// concept missing type + a broken link
	writeLintBundle(t, dir, "tables/orders.md",
		"---\ntype: \ntitle: orders\ndescription: SQLite table orders\n---\n# Columns\n\n[ghost](/tables/ghost.md)\n")

	// Spec violation + missing type + broken link → exit non-zero.
	out, err := exec.Command(bin, "lint", "--bundle", dir).CombinedOutput()
	if err == nil {
		t.Fatalf("bundle with violations should fail, but lint passed:\n%s", out)
	}
	s := string(out)
	for _, want := range []string{"spec-conformance", "missing a type", "broken cross-link"} {
		if !strings.Contains(s, want) {
			t.Errorf("lint output missing %q:\n%s", want, s)
		}
	}

	// --require-types=false drops the missing-type gate, but structural + broken-link
	// gates still fail.
	if _, err := exec.Command(bin, "lint", "--bundle", dir, "--require-types=false").CombinedOutput(); err == nil {
		t.Errorf("structural + broken-link violations should still fail")
	}

	// JSON output is well-formed and includes the conformance findings.
	jout, _ := exec.Command(bin, "lint", "--bundle", dir, "--json", "--max-broken-links", "99", "--require-types=false").CombinedOutput()
	if !strings.Contains(string(jout), "\"conformance\"") {
		t.Errorf("json report missing conformance key:\n%s", jout)
	}
}
