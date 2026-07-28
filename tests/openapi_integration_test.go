package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const openapiSpec = `openapi: 3.0.0
info:
  title: Orders API
  version: 1.0.0
paths:
  /orders/{id}:
    get:
      operationId: getOrder
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Order'
components:
  schemas:
    Order:
      type: object
      required: [id]
      properties:
        id:
          type: integer
        customer:
          $ref: '#/components/schemas/Customer'
    Customer:
      type: object
      properties:
        name:
          type: string
`

func TestOpenAPIIntegration(t *testing.T) {
	bin := getBinaryPath("okf-openapi")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("okf-openapi binary not built: %v", err)
	}
	dir := t.TempDir()
	spec := filepath.Join(dir, "api.yaml")
	if err := os.WriteFile(spec, []byte(openapiSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "bundle")
	if o, err := exec.Command(bin, "produce", "--spec", spec, "--out", out).CombinedOutput(); err != nil {
		t.Fatalf("produce failed: %v\n%s", err, o)
	}

	// Schema concept with a typed property table + cross-link to Customer.
	orderBytes, err := os.ReadFile(filepath.Join(out, "schemas", "Order.md"))
	if err != nil {
		t.Fatalf("schemas/Order.md not produced: %v", err)
	}
	order := string(orderBytes)
	for _, want := range []string{
		"type: Schema",
		"| id | integer | yes |",
		"| customer | Customer | no |",
		"# Relationships",
		"[Customer](/schemas/Customer.md)",
	} {
		if !strings.Contains(order, want) {
			t.Errorf("Order.md missing %q:\n%s", want, order)
		}
	}

	// Endpoint concept (operationId getOrder -> endpoints/getorder.md).
	epBytes, err := os.ReadFile(filepath.Join(out, "endpoints", "getorder.md"))
	if err != nil {
		t.Fatalf("endpoints/getorder.md not produced: %v", err)
	}
	ep := string(epBytes)
	for _, want := range []string{"type: API Endpoint", "## Parameters", "## Responses", "[Order](/schemas/Order.md)"} {
		if !strings.Contains(ep, want) {
			t.Errorf("endpoint doc missing %q:\n%s", want, ep)
		}
	}

	// Root index.md carries only okf_version.
	idx, _ := os.ReadFile(filepath.Join(out, "index.md"))
	if !strings.Contains(string(idx), "okf_version: \"0.1\"") {
		t.Errorf("index.md missing okf_version")
	}

	// Enrich + ingest --sync writes .okf-metadata.yaml.
	enriched := strings.Replace(order, "description: API schema Order", "description: One order.", 1)
	if enriched == order {
		t.Fatal("test setup: placeholder description not found to enrich")
	}
	if err := os.WriteFile(filepath.Join(out, "schemas", "Order.md"), []byte(enriched), 0o644); err != nil {
		t.Fatal(err)
	}
	if o, err := exec.Command(bin, "ingest", "--spec", spec, "--bundle", out, "--sync").CombinedOutput(); err != nil {
		t.Fatalf("ingest --sync failed: %v\n%s", err, o)
	}
	meta, err := os.ReadFile(filepath.Join(dir, ".okf-metadata.yaml"))
	if err != nil {
		t.Fatalf(".okf-metadata.yaml not written: %v", err)
	}
	if !strings.Contains(string(meta), "One order.") {
		t.Errorf(".okf-metadata.yaml missing synced description:\n%s", meta)
	}
}
