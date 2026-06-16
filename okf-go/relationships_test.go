package okf

import (
	"strings"
	"testing"
)

func TestRenderRelationshipsSection_RendersResolvableLinks(t *testing.T) {
	rels := []Relationship{
		{Label: "FK on customer_id", Target: "/tables/customers.md", Text: "customers"},
	}
	got := RenderRelationshipsSection(rels)
	want := "- FK on customer_id [customers](/tables/customers.md)\n"
	if got != want {
		t.Fatalf("unexpected render:\n got: %q\nwant: %q", got, want)
	}
}

func TestRenderRelationshipsSection_SortedForByteStability(t *testing.T) {
	a := []Relationship{
		{Label: "FK on b", Target: "/tables/b.md", Text: "b"},
		{Label: "FK on a", Target: "/tables/a.md", Text: "a"},
		{Label: "FK on c", Target: "/tables/c.md", Text: "c"},
	}
	// Same set, different input order.
	b := []Relationship{
		{Label: "FK on c", Target: "/tables/c.md", Text: "c"},
		{Label: "FK on a", Target: "/tables/a.md", Text: "a"},
		{Label: "FK on b", Target: "/tables/b.md", Text: "b"},
	}
	if RenderRelationshipsSection(a) != RenderRelationshipsSection(b) {
		t.Fatalf("render is not order-independent:\n%s\n---\n%s",
			RenderRelationshipsSection(a), RenderRelationshipsSection(b))
	}
	// Verify the deterministic order is by Target ascending.
	got := RenderRelationshipsSection(a)
	if idxA, idxB := strings.Index(got, "/tables/a.md"), strings.Index(got, "/tables/b.md"); idxA > idxB {
		t.Fatalf("expected a.md before b.md, got:\n%s", got)
	}
}

func TestRenderRelationshipsSection_Empty(t *testing.T) {
	if got := RenderRelationshipsSection(nil); got != "" {
		t.Fatalf("expected empty render for no relationships, got %q", got)
	}
}

func TestAppendRelationshipsSection_AppendsLevel1Heading(t *testing.T) {
	body := "# Columns\n\n| Name |\n| --- |\n| id |\n"
	rels := []Relationship{{Label: "FK on customer_id", Target: "/tables/customers.md", Text: "customers"}}
	got := AppendRelationshipsSection(body, "Relationships", rels)

	if !strings.Contains(got, "# Columns") {
		t.Fatalf("preceding content lost:\n%s", got)
	}
	if !strings.Contains(got, "# Relationships\n\n- FK on customer_id [customers](/tables/customers.md)") {
		t.Fatalf("relationships section missing or malformed:\n%s", got)
	}
	// GetSectionAny must isolate it so a column parser does not see the link rows.
	cols, ok := GetSectionAny(got, "Columns")
	if !ok || strings.Contains(cols, "customers.md") {
		t.Fatalf("Columns section leaked into relationships:\n%q", cols)
	}
	rel, ok := GetSectionAny(got, "Relationships")
	if !ok || !strings.Contains(rel, "/tables/customers.md") {
		t.Fatalf("Relationships section not retrievable via GetSectionAny:\n%q", rel)
	}
}

func TestAppendRelationshipsSection_EmptyEmitsNothing(t *testing.T) {
	body := "# Columns\n\n| Name |\n"
	if got := AppendRelationshipsSection(body, "Relationships", nil); got != body {
		t.Fatalf("empty relationships should leave body unchanged, got:\n%s", got)
	}
}
