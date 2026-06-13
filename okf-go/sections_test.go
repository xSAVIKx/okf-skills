package okf

import "testing"

func TestUpsertSection_AppendsWhenAbsent(t *testing.T) {
	body := "# Columns\n\n| Name |\n| --- |\n| id |\n"
	got := UpsertSection(body, "Data Profile", "| Column |\n| --- |\n| id |")
	if !contains(got, "# Columns") {
		t.Fatalf("original content lost:\n%s", got)
	}
	if !contains(got, "## Data Profile") {
		t.Fatalf("section not appended:\n%s", got)
	}
}

func TestUpsertSection_ReplacesWhenPresent(t *testing.T) {
	body := "# Columns\n\nfoo\n\n## Data Profile\n\nOLD\n"
	got := UpsertSection(body, "Data Profile", "NEW")
	if contains(got, "OLD") {
		t.Fatalf("old section content not replaced:\n%s", got)
	}
	if !contains(got, "NEW") {
		t.Fatalf("new content missing:\n%s", got)
	}
	if !contains(got, "foo") {
		t.Fatalf("preceding content lost:\n%s", got)
	}
}

func TestUpsertSection_PreservesTrailingSection(t *testing.T) {
	body := "## Data Profile\n\nOLD\n\n## Sample\n\nkeepme\n"
	got := UpsertSection(body, "Data Profile", "NEW")
	if !contains(got, "## Sample") || !contains(got, "keepme") {
		t.Fatalf("trailing section clobbered:\n%s", got)
	}
	if contains(got, "OLD") {
		t.Fatalf("old content not replaced:\n%s", got)
	}
}

func TestGetSection(t *testing.T) {
	body := "# Columns\n\nfoo\n\n## Data Profile\n\nrow1\nrow2\n"
	content, ok := GetSection(body, "Data Profile")
	if !ok {
		t.Fatal("expected section to be found")
	}
	if !contains(content, "row1") || !contains(content, "row2") {
		t.Fatalf("section content incomplete: %q", content)
	}
	if contains(content, "foo") {
		t.Fatalf("section content leaked preceding text: %q", content)
	}
	if _, ok := GetSection(body, "Nope"); ok {
		t.Fatal("expected missing section to report not found")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
