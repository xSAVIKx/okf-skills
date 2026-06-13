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
