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

func TestGetSectionAny_MatchesLevel1Heading(t *testing.T) {
	// Connectors write the schema as a level-1 "# Columns" heading and then append
	// "## Data Profile" / "## Sample" subsections. GetSectionAny must isolate the
	// Columns content and stop at the next heading.
	body := "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INTEGER |\n\n## Data Profile\n\n| Stat |\n| --- |\n| x |\n"
	content, ok := GetSectionAny(body, "Columns")
	if !ok {
		t.Fatal("expected to find the level-1 Columns heading")
	}
	if !contains(content, "| id | INTEGER |") {
		t.Fatalf("schema row missing from section:\n%s", content)
	}
	if contains(content, "Data Profile") || contains(content, "Stat") {
		t.Fatalf("section leaked past the next heading:\n%s", content)
	}
}

func TestGetSectionAny_MatchesLevel2Heading(t *testing.T) {
	body := "## Columns\n\nrow\n\n## Next\n\nkeepout\n"
	content, ok := GetSectionAny(body, "Columns")
	if !ok || !contains(content, "row") {
		t.Fatalf("expected level-2 heading to match: ok=%v content=%q", ok, content)
	}
	if contains(content, "keepout") {
		t.Fatalf("section leaked into the next section:\n%s", content)
	}
}

func TestGetSectionAny_NotFound(t *testing.T) {
	if _, ok := GetSectionAny("# Other\n\ntext\n", "Columns"); ok {
		t.Fatal("expected a missing heading to report not found")
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
