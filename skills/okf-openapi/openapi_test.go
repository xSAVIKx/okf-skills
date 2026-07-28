package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSpec = `openapi: 3.0.0
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

func loadSample(t *testing.T) []concept {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "api.yaml")
	if err := os.WriteFile(p, []byte(sampleSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := loadSpec(p)
	if err != nil {
		t.Fatalf("loadSpec: %v", err)
	}
	return extractConcepts(doc)
}

func TestExtractConcepts(t *testing.T) {
	concepts := loadSample(t)
	byRel := map[string]concept{}
	for _, c := range concepts {
		byRel[c.rel] = c
	}
	for _, want := range []string{"schemas/Order", "schemas/Customer", "endpoints/getorder"} {
		if _, ok := byRel[want]; !ok {
			t.Errorf("missing concept %q (got %v)", want, keysOf(byRel))
		}
	}
	if byRel["schemas/Order"].typ != "Schema" {
		t.Errorf("Order type = %q", byRel["schemas/Order"].typ)
	}
	if byRel["endpoints/getorder"].typ != "API Endpoint" {
		t.Errorf("getorder type = %q", byRel["endpoints/getorder"].typ)
	}
	// Order references Customer (property $ref).
	if !contains(byRel["schemas/Order"].links, "schemas/Customer") {
		t.Errorf("Order links = %v, want schemas/Customer", byRel["schemas/Order"].links)
	}
	// Endpoint references Order (response $ref).
	if !contains(byRel["endpoints/getorder"].links, "schemas/Order") {
		t.Errorf("getorder links = %v, want schemas/Order", byRel["endpoints/getorder"].links)
	}
	// Schema body has a typed property table.
	ob := byRel["schemas/Order"].body
	for _, want := range []string{"| id | integer | yes |", "| customer | Customer | no |"} {
		if !strings.Contains(ob, want) {
			t.Errorf("Order body missing %q:\n%s", want, ob)
		}
	}
	// Endpoint body has parameters + responses.
	eb := byRel["endpoints/getorder"].body
	for _, want := range []string{"## Parameters", "| id | path | integer | yes |", "## Responses", "| 200 | Order |"} {
		if !strings.Contains(eb, want) {
			t.Errorf("endpoint body missing %q:\n%s", want, eb)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"getOrder":     "getorder",
		"GET /orders":  "get-orders",
		"a//b":         "a-b",
		"  trim-me!  ": "trim-me",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRefName(t *testing.T) {
	if refName("#/components/schemas/Order") != "Order" {
		t.Error("refName components")
	}
	if refName("#/definitions/Order") != "Order" {
		t.Error("refName definitions")
	}
	if refName("") != "" {
		t.Error("refName empty")
	}
}

func keysOf(m map[string]concept) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	return k
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
