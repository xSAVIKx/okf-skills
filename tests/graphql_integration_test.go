package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const graphqlSDL = `
type Query {
  orders(status: String): [Order!]!
}
type Order {
  id: ID!
  total: Float
  customer: Customer
}
type Customer {
  id: ID!
  name: String
}
`

func TestGraphQLIntegration(t *testing.T) {
	bin := getBinaryPath("okf-graphql")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-graphql binary not built: %v", err)
	}
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.graphql")
	if err := os.WriteFile(schema, []byte(graphqlSDL), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "bundle")
	if o, err := exec.Command(bin, "produce", "--schema", schema, "--out", out).CombinedOutput(); err != nil {
		t.Fatalf("produce failed: %v\n%s", err, o)
	}

	// Type concept with a field table + cross-link to Customer.
	orderBytes, err := os.ReadFile(filepath.Join(out, "types", "Order.md"))
	if err != nil {
		t.Fatalf("types/Order.md not produced: %v", err)
	}
	order := string(orderBytes)
	for _, want := range []string{
		"type: GraphQL Type",
		"| id | ID! | yes |",
		"| customer | Customer | no |",
		"# Relationships",
		"[Customer](/types/Customer.md)",
	} {
		if !strings.Contains(order, want) {
			t.Errorf("Order.md missing %q:\n%s", want, order)
		}
	}

	// Query operation concept references Order.
	q, err := os.ReadFile(filepath.Join(out, "queries", "orders.md"))
	if err != nil {
		t.Fatalf("queries/orders.md not produced: %v", err)
	}
	qs := string(q)
	for _, want := range []string{"type: GraphQL Query", "Returns `[Order!]!`", "[Order](/types/Order.md)"} {
		if !strings.Contains(qs, want) {
			t.Errorf("orders.md missing %q:\n%s", want, qs)
		}
	}

	// Root index.md carries only okf_version.
	idx, _ := os.ReadFile(filepath.Join(out, "index.md"))
	if !strings.Contains(string(idx), "okf_version: \"0.1\"") {
		t.Errorf("index.md missing okf_version")
	}

	// Enrich + ingest --sync writes .okf-metadata.yaml.
	enriched := strings.Replace(order, "description: GraphQL type Order", "description: An order.", 1)
	if enriched == order {
		t.Fatal("test setup: placeholder description not found")
	}
	if err := os.WriteFile(filepath.Join(out, "types", "Order.md"), []byte(enriched), 0o644); err != nil {
		t.Fatal(err)
	}
	if o, err := exec.Command(bin, "ingest", "--schema", schema, "--bundle", out, "--sync").CombinedOutput(); err != nil {
		t.Fatalf("ingest --sync failed: %v\n%s", err, o)
	}
	meta, err := os.ReadFile(filepath.Join(dir, ".okf-metadata.yaml"))
	if err != nil {
		t.Fatalf(".okf-metadata.yaml not written: %v", err)
	}
	if !strings.Contains(string(meta), "An order.") {
		t.Errorf(".okf-metadata.yaml missing synced description:\n%s", meta)
	}
}
