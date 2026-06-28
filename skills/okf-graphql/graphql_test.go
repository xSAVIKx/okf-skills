package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSDL = `
type Query {
  orders(status: String): [Order!]!
  customer(id: ID!): Customer
}

type Mutation {
  createOrder(input: OrderInput!): Order
}

type Order {
  id: ID!
  total: Float
  customer: Customer
  status: Status
}

input OrderInput {
  total: Float!
}

type Customer {
  id: ID!
  name: String
}

enum Status {
  ACTIVE
  PENDING
}
`

func loadSample(t *testing.T) []concept {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "schema.graphql")
	if err := os.WriteFile(p, []byte(sampleSDL), 0o644); err != nil {
		t.Fatal(err)
	}
	schema, err := loadSchema(p)
	if err != nil {
		t.Fatalf("loadSchema: %v", err)
	}
	return extractConcepts(schema)
}

func TestExtractConcepts(t *testing.T) {
	concepts := loadSample(t)
	by := map[string]concept{}
	for _, c := range concepts {
		by[c.rel] = c
	}

	for _, want := range []string{
		"types/Order", "types/Customer", "types/OrderInput", "types/Status",
		"queries/orders", "queries/customer", "mutations/createorder",
	} {
		if _, ok := by[want]; !ok {
			t.Errorf("missing concept %q", want)
		}
	}
	// Root operation types are not emitted as plain types.
	if _, ok := by["types/Query"]; ok {
		t.Error("Query root type should not be a types/ concept")
	}
	// Order references Customer and Status (field types).
	if !contains(by["types/Order"].links, "types/Customer") || !contains(by["types/Order"].links, "types/Status") {
		t.Errorf("Order links = %v, want Customer + Status", by["types/Order"].links)
	}
	// Order field table includes the rendered GraphQL types.
	ob := by["types/Order"].body
	for _, want := range []string{"# Columns", "| id | ID! | yes |", "| total | Float | no |", "| customer | Customer | no |"} {
		if !strings.Contains(ob, want) {
			t.Errorf("Order body missing %q:\n%s", want, ob)
		}
	}
	// Enum renders its values.
	sb := by["types/Status"].body
	if !strings.Contains(sb, "# Values") || !strings.Contains(sb, "- ACTIVE") {
		t.Errorf("Status enum body missing values:\n%s", sb)
	}
	// Query.orders returns [Order!]! and references Order; has a status argument.
	q := by["queries/orders"]
	if q.typ != "GraphQL Query" {
		t.Errorf("orders typ = %q", q.typ)
	}
	if !contains(q.links, "types/Order") {
		t.Errorf("orders links = %v, want types/Order", q.links)
	}
	if !strings.Contains(q.body, "Returns `[Order!]!`") || !strings.Contains(q.body, "## Arguments") {
		t.Errorf("orders body unexpected:\n%s", q.body)
	}
	// Mutation concept typed correctly.
	if by["mutations/createorder"].typ != "GraphQL Mutation" {
		t.Errorf("createorder typ = %q", by["mutations/createorder"].typ)
	}
}

func TestUnderlying(t *testing.T) {
	concepts := loadSample(t)
	_ = concepts // underlying is exercised via extraction above; smoke-test slugify here
	if slugify("createOrder") != "createorder" {
		t.Errorf("slugify(createOrder) = %q", slugify("createOrder"))
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
