package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSVIntegration(t *testing.T) {
	bin := getBinaryPath("okf-csv")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-csv binary not built: %v", err)
	}
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	csv := "id,price,active,status\n1,1.50,true,active\n2,2.00,false,pending\n3,3.25,true,active\n"
	if err := os.WriteFile(filepath.Join(dataDir, "orders.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "bundle")
	if o, err := exec.Command(bin, "produce", "--dir", dataDir, "--out", out, "--profile", "--sample", "2").CombinedOutput(); err != nil {
		t.Fatalf("produce failed: %v\n%s", err, o)
	}

	body, err := os.ReadFile(filepath.Join(out, "tables", "orders.md"))
	if err != nil {
		t.Fatalf("orders.md not produced: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"type: CSV File",
		"# Columns",
		"| id | integer |",
		"| price | number |",
		"| active | boolean |",
		"## Data Profile",            // --profile section header (rendered by okf-go)
		"status ∈ {active, pending}", // low-cardinality value set
	} {
		if !strings.Contains(s, want) {
			t.Errorf("orders.md missing %q:\n%s", want, s)
		}
	}

	// Root index.md carries only okf_version.
	idx, _ := os.ReadFile(filepath.Join(out, "index.md"))
	if !strings.Contains(string(idx), "okf_version: \"0.1\"") {
		t.Errorf("index.md missing okf_version")
	}

	// Enrich the description, then ingest --sync writes it to .okf-metadata.yaml.
	enriched := strings.Replace(s, "description: CSV file orders", "description: One row per order.", 1)
	if enriched == s {
		t.Fatal("test setup: placeholder description not found to enrich")
	}
	if err := os.WriteFile(filepath.Join(out, "tables", "orders.md"), []byte(enriched), 0o644); err != nil {
		t.Fatal(err)
	}
	if o, err := exec.Command(bin, "ingest", "--dir", dataDir, "--bundle", out, "--sync").CombinedOutput(); err != nil {
		t.Fatalf("ingest --sync failed: %v\n%s", err, o)
	}
	meta, err := os.ReadFile(filepath.Join(dataDir, ".okf-metadata.yaml"))
	if err != nil {
		t.Fatalf(".okf-metadata.yaml not written: %v", err)
	}
	if !strings.Contains(string(meta), "One row per order.") {
		t.Errorf(".okf-metadata.yaml missing synced description:\n%s", meta)
	}

	// Re-produce preserves the enriched description (incremental produce).
	if o, err := exec.Command(bin, "produce", "--dir", dataDir, "--out", out, "--profile", "--sample", "2").CombinedOutput(); err != nil {
		t.Fatalf("second produce failed: %v\n%s", err, o)
	}
	body2, _ := os.ReadFile(filepath.Join(out, "tables", "orders.md"))
	if !strings.Contains(string(body2), "description: One row per order.") {
		t.Errorf("re-produce clobbered the enriched description:\n%s", body2)
	}
}
