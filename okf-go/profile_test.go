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
