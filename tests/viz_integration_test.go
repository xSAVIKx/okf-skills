package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
