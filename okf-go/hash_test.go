package okf

import (
	"strings"
	"testing"
)

func TestConceptStructuralHash_StableAcrossRuns(t *testing.T) {
	doc := ConceptDoc{Body: "# Columns\n\n| Name |\n| --- |\n| id |\n"}
	if ConceptStructuralHash(doc) != ConceptStructuralHash(doc) {
		t.Fatal("hash not stable for identical input")
	}
}

func TestConceptStructuralHash_IgnoresDescriptionAndTimestamp(t *testing.T) {
	body := "# Columns\n\n| Name |\n| --- |\n| id |\n"
	a := ConceptDoc{Frontmatter: Frontmatter{Description: "one", Timestamp: "2020-01-01"}, Body: body}
	b := ConceptDoc{Frontmatter: Frontmatter{Description: "two", Timestamp: "2026-06-14"}, Body: body}
	if ConceptStructuralHash(a) != ConceptStructuralHash(b) {
		t.Fatal("description/timestamp-only change must not alter the structural hash")
	}
}

func TestConceptStructuralHash_ChangesWithBody(t *testing.T) {
	a := ConceptDoc{Body: "# Columns\n\n| Name |\n| --- |\n| id |\n"}
	b := ConceptDoc{Body: "# Columns\n\n| Name |\n| --- |\n| id |\n| email |\n"}
	if ConceptStructuralHash(a) == ConceptStructuralHash(b) {
		t.Fatal("a column change must alter the structural hash")
	}
}

func TestConceptStructuralHash_NormalizesLineEndings(t *testing.T) {
	a := ConceptDoc{Body: "# Columns\n\n| id |\n"}
	b := ConceptDoc{Body: "# Columns\r\n\r\n| id |\r\n"}
	if ConceptStructuralHash(a) != ConceptStructuralHash(b) {
		t.Fatal("CRLF vs LF must not change the hash")
	}
}

func TestMergeConcept_NewConceptStampsHash(t *testing.T) {
	fresh := ConceptDoc{Frontmatter: Frontmatter{Description: "x"}, Body: "# Columns\n\n| id |\n"}
	merged, changed := MergeConcept(nil, fresh)
	if !changed {
		t.Fatal("new concept must report changed")
	}
	if merged.Frontmatter.ContentHash == "" {
		t.Fatal("new concept must have ContentHash stamped")
	}
}

func TestMergeConcept_UnchangedStructurePreservesByteForByte(t *testing.T) {
	body := "# Columns\n\n| id |\n"
	h := ConceptStructuralHash(ConceptDoc{Body: body})
	existing := ConceptDoc{
		Frontmatter: Frontmatter{Description: "agent wrote this", Timestamp: "2020-01-01", ContentHash: h, Tags: []string{"pii"}},
		Body:        body + "\n## Agent Notes\n\nimportant\n",
	}
	fresh := ConceptDoc{Frontmatter: Frontmatter{Description: "SQLite table x", Timestamp: "2026-06-14"}, Body: body}

	merged, changed := MergeConcept(&existing, fresh)
	if changed {
		t.Fatal("unchanged structure must report changed == false")
	}
	if merged.Frontmatter.Description != "agent wrote this" || merged.Frontmatter.Timestamp != "2020-01-01" {
		t.Fatalf("preserve path must return existing verbatim, got %+v", merged.Frontmatter)
	}
	if !strings.Contains(merged.Body, "Agent Notes") {
		t.Fatal("preserve path must keep agent body prose")
	}
}

func TestMergeConcept_CarriesEnrichedAgainstOnChange(t *testing.T) {
	oldBody := "# Columns\n\n| id |\n"
	existing := ConceptDoc{
		Frontmatter: Frontmatter{
			Description:     "agent description",
			ContentHash:     ConceptStructuralHash(ConceptDoc{Body: oldBody}),
			EnrichedAgainst: "stale-hash-value",
		},
		Body: oldBody,
	}
	fresh := ConceptDoc{Body: "# Columns\n\n| id |\n| email |\n"}
	merged, changed := MergeConcept(&existing, fresh)
	if !changed {
		t.Fatal("structural change expected")
	}
	if merged.Frontmatter.EnrichedAgainst != "stale-hash-value" {
		t.Fatalf("enriched_against must be carried over, got %q", merged.Frontmatter.EnrichedAgainst)
	}
	// And it must now differ from the new content hash → concept reads as stale.
	if merged.Frontmatter.EnrichedAgainst == merged.Frontmatter.ContentHash {
		t.Fatal("after a structural change the marker must not match the new hash")
	}
}

func TestMergeConcept_StructuralChangeCarriesAgentContent(t *testing.T) {
	oldBody := "# Columns\n\n| id |\n"
	existing := ConceptDoc{
		Frontmatter: Frontmatter{Description: "agent description", ContentHash: ConceptStructuralHash(ConceptDoc{Body: oldBody}), Tags: []string{"pii"}},
		Body:        oldBody + "\n## Agent Notes\n\nkeep me\n",
	}
	// New structure: a column was added.
	fresh := ConceptDoc{
		Frontmatter: Frontmatter{Description: "SQLite table x", Tags: []string{"sqlite", "table"}},
		Body:        "# Columns\n\n| id |\n| email |\n",
	}

	merged, changed := MergeConcept(&existing, fresh)
	if !changed {
		t.Fatal("structural change must report changed == true")
	}
	if merged.Frontmatter.Description != "agent description" {
		t.Fatalf("agent description must be carried over, got %q", merged.Frontmatter.Description)
	}
	if !strings.Contains(merged.Body, "email") {
		t.Fatal("fresh structural body must be used")
	}
	if !strings.Contains(merged.Body, "Agent Notes") {
		t.Fatal("agent-added section must be preserved on structural change")
	}
	// Tags union includes both source and agent tags.
	joined := strings.Join(merged.Frontmatter.Tags, ",")
	if !strings.Contains(joined, "pii") || !strings.Contains(joined, "sqlite") {
		t.Fatalf("tags must be unioned, got %v", merged.Frontmatter.Tags)
	}
	if merged.Frontmatter.ContentHash != ConceptStructuralHash(fresh) {
		t.Fatal("merged doc must carry the fresh structural hash")
	}
}
