package okf

import (
	"strings"
	"testing"
)

func TestRenderConstraintsSection(t *testing.T) {
	if RenderConstraintsSection(nil) != "" {
		t.Fatal("empty constraints must render nothing")
	}
	got := RenderConstraintsSection([]Constraint{
		{Name: "uq_email", Type: "UNIQUE", Definition: "email"},
		{Name: "ck_age", Type: "CHECK", Definition: "age >= 0"},
	})
	if !strings.Contains(got, "| Name | Type | Definition |") {
		t.Fatalf("missing header:\n%s", got)
	}
	// Sorted by name: ck_age before uq_email.
	if strings.Index(got, "ck_age") > strings.Index(got, "uq_email") {
		t.Fatalf("constraints not sorted by name:\n%s", got)
	}
}

func TestRenderIndexesSection(t *testing.T) {
	if RenderIndexesSection(nil) != "" {
		t.Fatal("empty indexes must render nothing")
	}
	got := RenderIndexesSection([]Index{
		{Name: "idx_users_email", Columns: []string{"email"}, Unique: true},
	})
	if !strings.Contains(got, "| idx_users_email | email | Yes |") {
		t.Fatalf("unexpected index render:\n%s", got)
	}
}

func TestRenderStatsSection(t *testing.T) {
	if RenderStatsSection(TableStats{}) != "" {
		t.Fatal("empty stats must render nothing")
	}
	got := RenderStatsSection(TableStats{
		RowCount: 42, HasRowCount: true,
		FreshnessColumn: "created_at", Earliest: "2019-01-01", Latest: "2026-06-14",
	})
	if !strings.Contains(got, "**Row Count**: 42") {
		t.Fatalf("missing row count:\n%s", got)
	}
	if !strings.Contains(got, "**Freshness** (`created_at`): 2019-01-01 … 2026-06-14") {
		t.Fatalf("missing freshness:\n%s", got)
	}
}

func TestRenderStatsSection_RowCountOnly(t *testing.T) {
	got := RenderStatsSection(TableStats{RowCount: 0, HasRowCount: true})
	if !strings.Contains(got, "**Row Count**: 0") || strings.Contains(got, "Freshness") {
		t.Fatalf("expected only a row count line:\n%s", got)
	}
}

func TestRenderViewDefinition(t *testing.T) {
	if RenderViewDefinition("  ") != "" {
		t.Fatal("blank view SQL must render nothing")
	}
	got := RenderViewDefinition("SELECT 1")
	if got != "```sql\nSELECT 1\n```\n" {
		t.Fatalf("unexpected view definition render:\n%q", got)
	}
}

func TestMetadataSections_IsolatedFromColumns(t *testing.T) {
	// Assemble a body the way a connector would, then ensure column parsing via
	// GetSectionAny is not polluted by the new sections.
	body := "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INT |\n"
	body = UpsertSection(body, "Constraints", RenderConstraintsSection([]Constraint{{Name: "uq", Type: "UNIQUE", Definition: "id"}}))
	body = UpsertSection(body, "Indexes", RenderIndexesSection([]Index{{Name: "ix", Columns: []string{"id"}, Unique: true}}))
	body = UpsertSection(body, "Stats", RenderStatsSection(TableStats{RowCount: 1, HasRowCount: true}))

	cols, ok := GetSectionAny(body, "Columns")
	if !ok {
		t.Fatal("Columns section not found")
	}
	if strings.Contains(cols, "UNIQUE") || strings.Contains(cols, "Row Count") {
		t.Fatalf("metadata sections leaked into Columns:\n%s", cols)
	}
}
