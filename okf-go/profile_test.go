package okf

import "testing"

func TestRenderProfileSection(t *testing.T) {
	profiles := []ColumnProfile{
		{Column: "id", NonNull: 10, Null: 0, Distinct: 10, Min: "1", Max: "10"},
		{Column: "name", NonNull: 8, Null: 2, Distinct: 7, Min: "alice", Max: "zed"},
	}
	got := RenderProfileSection(profiles)
	for _, want := range []string{"| Column |", "| id |", "| name |", "| 10 |", "alice"} {
		if !contains(got, want) {
			t.Fatalf("rendered profile missing %q:\n%s", want, got)
		}
	}
}

func TestRenderProfileSection_LegacyWhenNoSemantic(t *testing.T) {
	profiles := []ColumnProfile{{Column: "id", NonNull: 10, Null: 0, Distinct: 10, Min: "1", Max: "10"}}
	got := RenderProfileSection(profiles)
	if contains(got, "Semantic") {
		t.Fatalf("semantic-free profile must render the legacy 6-column table:\n%s", got)
	}
	if !contains(got, "| Column | Non-Null | Null | Distinct | Min | Max |") {
		t.Fatalf("legacy header missing:\n%s", got)
	}
}

func TestRenderProfileSection_WithSemanticAndValues(t *testing.T) {
	profiles := []ColumnProfile{
		{Column: "email", NonNull: 5, Null: 0, Distinct: 5, Min: "a@x.com", Max: "z@x.com", Semantic: "email"},
		{Column: "status", NonNull: 5, Null: 0, Distinct: 3, Min: "cancelled", Max: "shipped", Semantic: "enum", Values: []string{"shipped", "pending", "cancelled"}},
	}
	got := RenderProfileSection(profiles)
	if !contains(got, "| Column | Non-Null | Null | Distinct | Min | Max | Semantic |") {
		t.Fatalf("semantic header missing:\n%s", got)
	}
	if !contains(got, "| email | 5 | 0 | 5 | a@x.com | z@x.com | email |") {
		t.Fatalf("semantic row missing:\n%s", got)
	}
	// Values rendered sorted.
	if !contains(got, "- status ∈ {cancelled, pending, shipped}") {
		t.Fatalf("sorted value set missing:\n%s", got)
	}
}

func TestRenderProfileSection_SanitizesValues(t *testing.T) {
	profiles := []ColumnProfile{
		{Column: "kind", Distinct: 1, Semantic: "enum", Values: []string{"a|b"}},
	}
	got := RenderProfileSection(profiles)
	if !contains(got, `a\|b`) {
		t.Fatalf("pipe in value not sanitized:\n%s", got)
	}
}

func TestRenderSampleSection(t *testing.T) {
	headers := []string{"id", "name"}
	rows := [][]string{{"1", "alice"}, {"2", "bob"}}
	got := RenderSampleSection(headers, rows)
	for _, want := range []string{"| id | name |", "| 1 | alice |", "| 2 | bob |"} {
		if !contains(got, want) {
			t.Fatalf("rendered sample missing %q:\n%s", want, got)
		}
	}
}

func TestSanitizeCell(t *testing.T) {
	if got := SanitizeCell("a|b\nc"); got != "a\\|b c" {
		t.Fatalf("SanitizeCell = %q, want %q", got, "a\\|b c")
	}
}

func TestRenderSampleSection_RowShorterThanHeaders(t *testing.T) {
	headers := []string{"a", "b", "c"}
	rows := [][]string{{"1", "2"}}
	got := RenderSampleSection(headers, rows)
	if !contains(got, "| 1 | 2 |  |") {
		t.Fatalf("short row not padded to header width:\n%s", got)
	}
}

func TestRenderSampleSection_EmptyHeaders(t *testing.T) {
	if got := RenderSampleSection(nil, nil); got != "" {
		t.Fatalf("empty headers should render empty string, got %q", got)
	}
}

func TestSanitizeCell_CRLF(t *testing.T) {
	if got := SanitizeCell("a\r\nb"); got != "a b" {
		t.Fatalf("SanitizeCell CRLF = %q, want %q", got, "a b")
	}
}
