package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOkfVizTypedEdges(t *testing.T) {
	bin := getBinaryPath("okf-viz")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-viz binary not built: %v", err)
	}
	bundle := t.TempDir()
	write := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(bundle, "index.md"), "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	write(filepath.Join(bundle, "tables", "orders.md"),
		"---\ntype: SQLite Table\ntitle: orders\ndescription: One row per order.\n---\n# Columns\n\n# Relationships\n\n- FK on customer_id [customers](/tables/customers.md)\n")
	write(filepath.Join(bundle, "tables", "customers.md"),
		"---\ntype: SQLite Table\ntitle: customers\ndescription: One row per customer.\n---\n# Columns\n")

	out := filepath.Join(bundle, "index.html")
	if err := exec.Command(bin, "render", "--bundle", bundle, "--out", out).Run(); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	html, _ := os.ReadFile(out)
	if !strings.Contains(string(html), "\"relation\":\"references\"") {
		t.Errorf("index.html missing typed FK relation in inlined JSON")
	}
	// navigation controls + permalink/fuzzy/trace hooks are present.
	s := string(html)
	for _, want := range []string{`id="hops"`, `id="trace"`, "function fuzzyScore", "function writeHash", "dijkstra"} {
		if !strings.Contains(s, want) {
			t.Errorf("index.html missing navigation hook %q", want)
		}
	}
}

func TestOkfVizERColumns(t *testing.T) {
	bin := getBinaryPath("okf-viz")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-viz binary not built: %v", err)
	}
	bundle := t.TempDir()
	write := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(bundle, "index.md"), "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	write(filepath.Join(bundle, "tables", "orders.md"),
		"---\ntype: SQLite Table\ntitle: orders\ndescription: One row per order.\n---\n# Columns\n\n| Name | Type | Primary Key | Nullable | Default |\n| --- | --- | --- | --- | --- |\n| id | INTEGER | Yes | No |  |\n| customer_id | INTEGER | No | No |  |\n\n# Relationships\n\n- FK on customer_id [customers](/tables/customers.md)\n")
	write(filepath.Join(bundle, "tables", "customers.md"),
		"---\ntype: SQLite Table\ntitle: customers\ndescription: One row per customer.\n---\n# Columns\n\n| Name | Type | Primary Key | Nullable | Default |\n| --- | --- | --- | --- | --- |\n| id | INTEGER | Yes | No |  |\n")

	out := filepath.Join(bundle, "index.html")
	if err := exec.Command(bin, "render", "--bundle", bundle, "--out", out).Run(); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	html, _ := os.ReadFile(out)
	s := string(html)
	if !strings.Contains(s, "\"columns\":") || !strings.Contains(s, "\"customer_id\"") {
		t.Errorf("index.html missing parsed columns payload")
	}
	if !strings.Contains(s, "\"fk\":true") {
		t.Errorf("index.html should mark customer_id as an FK column")
	}
	if !strings.Contains(s, "er-mode") {
		t.Errorf("index.html missing the ER mode control")
	}
}

func TestOkfVizLazyScale(t *testing.T) {
	bin := getBinaryPath("okf-viz")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-viz binary not built: %v", err)
	}
	bundle := t.TempDir()
	write := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(bundle, "index.md"), "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	write(filepath.Join(bundle, "tables", "a.md"), "---\ntype: SQLite Table\ntitle: a\n---\n# Columns\n\nbody of A here.\n")
	write(filepath.Join(bundle, "tables", "b.md"), "---\ntype: SQLite Table\ntitle: b\n---\n# Columns\n\nbody of B here.\n")

	out := filepath.Join(bundle, "index.html")
	// threshold 1 forces lazy with 2 concepts.
	if err := exec.Command(bin, "render", "--bundle", bundle, "--out", out, "--threshold", "1").Run(); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	html, _ := os.ReadFile(out)
	if !strings.Contains(string(html), "\"lazy\":true") {
		t.Errorf("expected lazy payload above threshold")
	}
	if strings.Contains(string(html), "body of A here") {
		t.Errorf("bodies must not be inlined in lazy mode")
	}
	// fragment written next to index.html and contains the body
	frag := filepath.Join(bundle, "_okf", "tables", "a.html")
	fb, err := os.ReadFile(frag)
	if err != nil || !strings.Contains(string(fb), "body of A here") {
		t.Errorf("body fragment missing or wrong: %v", err)
	}

	// --inline-all forces single file (no fragments) even above threshold.
	out2 := filepath.Join(bundle, "full.html")
	if err := exec.Command(bin, "render", "--bundle", bundle, "--out", out2, "--threshold", "1", "--inline-all").Run(); err != nil {
		t.Fatalf("render --inline-all failed: %v", err)
	}
	h2, _ := os.ReadFile(out2)
	if !strings.Contains(string(h2), "body of A here") {
		t.Errorf("--inline-all must inline bodies")
	}
}

func TestOkfVizCoverage(t *testing.T) {
	bin := getBinaryPath("okf-viz")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-viz binary not built: %v", err)
	}

	bundle := t.TempDir()
	write := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(bundle, "index.md"), "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	// one enriched, one placeholder.
	write(filepath.Join(bundle, "tables", "orders.md"),
		"---\ntype: SQLite Table\ntitle: orders\ndescription: One row per order.\n---\n# Columns\n\nFK to [customers](/tables/customers.md).\n")
	write(filepath.Join(bundle, "tables", "customers.md"),
		"---\ntype: SQLite Table\ntitle: customers\ndescription: SQLite table customers\n---\n# Columns\n")

	// Text report.
	out, err := exec.Command(bin, "coverage", "--bundle", bundle).Output()
	if err != nil {
		t.Fatalf("coverage failed: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "enriched") || !strings.Contains(s, "placeholders: 1") {
		t.Errorf("unexpected coverage report:\n%s", s)
	}

	// JSON report.
	jout, err := exec.Command(bin, "coverage", "--bundle", bundle, "--json").Output()
	if err != nil {
		t.Fatalf("coverage --json failed: %v", err)
	}
	if !strings.Contains(string(jout), "\"enriched_pct\"") {
		t.Errorf("json report missing enriched_pct:\n%s", string(jout))
	}

	// Gate: 50%% enriched, require 90%% -> must exit non-zero.
	cmd := exec.Command(bin, "coverage", "--bundle", bundle, "--min", "90")
	if err := cmd.Run(); err == nil {
		t.Errorf("coverage gate should have failed at --min 90 for a 50%% bundle")
	}
}

func TestOkfVizRender(t *testing.T) {
	bin := getBinaryPath("okf-viz")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-viz binary not built: %v", err)
	}

	bundle := t.TempDir()
	must := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(bundle, "index.md"), "---\nokf_version: \"0.1\"\n---\n# Demo\n")
	must(filepath.Join(bundle, "tables", "orders.md"),
		"---\ntype: SQLite Table\ntitle: orders\ndescription: One row per order.\n---\n# Columns\n\nFK to [customers](/tables/customers.md).\n")
	must(filepath.Join(bundle, "tables", "customers.md"),
		"---\ntype: SQLite Table\ntitle: customers\ndescription: One row per customer.\n---\n# Columns\n")

	cmd := exec.Command(bin, "render", "--bundle", bundle)
	var stderr bytesBuffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("render failed: %v\nstdout: %s\nstderr: %s", err, out, stderr.String())
	}

	htmlPath := filepath.Join(bundle, "index.html")
	data, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("index.html not written next to index.md: %v", err)
	}
	s := string(data)
	for _, want := range []string{`id="okf-data"`, "tables/orders", "tables/customers", `"kind":"crosslink"`} {
		if !strings.Contains(s, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
}
