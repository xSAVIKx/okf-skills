package main

import "testing"

func TestForeignKeyRelationships_MapsFKsToEdges(t *testing.T) {
	fks := []foreignKey{
		{FromColumn: "customer_id", ToTable: "customers", ToColumn: "id"},
		{FromColumn: "product_id", ToTable: "products", ToColumn: "id"},
	}
	rels := foreignKeyRelationships(fks)
	if len(rels) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(rels))
	}
	if rels[0].Label != "FK on customer_id" || rels[0].Target != "/tables/customers.md" || rels[0].Text != "customers" {
		t.Fatalf("unexpected first relationship: %+v", rels[0])
	}
}

func TestForeignKeyRelationships_SkipsEmptyTarget(t *testing.T) {
	fks := []foreignKey{{FromColumn: "orphan", ToTable: "", ToColumn: ""}}
	if rels := foreignKeyRelationships(fks); len(rels) != 0 {
		t.Fatalf("expected no relationships for empty target table, got %+v", rels)
	}
}
