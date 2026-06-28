package main

import (
	"strings"
	"testing"
)

func TestWireGoWork(t *testing.T) {
	in := "go 1.26\n\nuse (\n\t./okf-go\n\t./skills/okf-bigquery\n\t./skills/okf-sqlite\n\t./tests\n)\n"
	out, err := wireGoWork(in, "okf-csv")
	if err != nil {
		t.Fatal(err)
	}
	// inserted alphabetically: bigquery < csv < sqlite
	want := "\t./skills/okf-bigquery\n\t./skills/okf-csv\n\t./skills/okf-sqlite\n"
	if !strings.Contains(out, want) {
		t.Fatalf("not inserted in sorted position:\n%s", out)
	}
	// idempotent
	again, err := wireGoWork(out, "okf-csv")
	if err != nil {
		t.Fatal(err)
	}
	if again != out {
		t.Fatalf("not idempotent:\n%s", again)
	}
	// missing anchor
	if _, err := wireGoWork("use (\n\t./okf-go\n)\n", "okf-csv"); err == nil {
		t.Fatal("expected error when no ./skills/ entries")
	}
}

func TestWireMakefile(t *testing.T) {
	in := "SKILLS := okf-sqlite okf-viz\n\nbuild:\n"
	out, err := wireMakefile(in, "okf-csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "SKILLS := okf-sqlite okf-viz okf-csv\n") {
		t.Fatalf("skill not appended:\n%s", out)
	}
	if again, _ := wireMakefile(out, "okf-csv"); again != out {
		t.Fatal("not idempotent")
	}
	if _, err := wireMakefile("no skills line\n", "okf-csv"); err == nil {
		t.Fatal("expected error when SKILLS line missing")
	}
}

func TestWireInstallSh(t *testing.T) {
	in := "#!/bin/sh\nSKILLS=\"okf-sqlite okf-viz\"\n"
	out, err := wireInstallSh(in, "okf-csv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `SKILLS="okf-sqlite okf-viz okf-csv"`) {
		t.Fatalf("skill not appended:\n%s", out)
	}
	if again, _ := wireInstallSh(out, "okf-csv"); again != out {
		t.Fatal("not idempotent")
	}
}

func TestWireSkillsJSON(t *testing.T) {
	in := `{
  "$schema": "x",
  "notGrouped": "bottom",
  "groupings": [
    { "title": "Database Connectors", "description": "d", "skills": ["okf-sqlite"] }
  ]
}`
	out, err := wireSkillsJSON(in, "okf-csv", "Database Connectors")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"okf-csv"`) {
		t.Fatalf("skill not added:\n%s", out)
	}
	if !strings.Contains(out, `"$schema": "x"`) {
		t.Fatalf("schema key not preserved:\n%s", out)
	}
	if again, _ := wireSkillsJSON(out, "okf-csv", "Database Connectors"); again != out {
		t.Fatal("not idempotent")
	}
	if _, err := wireSkillsJSON(in, "okf-csv", "Nonexistent"); err == nil {
		t.Fatal("expected error for unknown grouping")
	}
}

func TestWireReadme(t *testing.T) {
	in := "### Available Connectors\n\n| Skill | Data Source | Key Feature |\n|---|---|---|\n" +
		"| `okf-sqlite` | SQLite databases | CGO-free |\n| `okf-fs` | Local filesystem | sidecar |\n\n" +
		"### Commands\n\n" +
		"| Group | Skills |\n|---|---|\n| Database Connectors | `okf-sqlite` |\n"
	out, err := wireReadme(in, "okf-csv", "CSV File", "Database Connectors")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "| `okf-csv` | CSV File | — |") {
		t.Fatalf("connector row not added:\n%s", out)
	}
	if !strings.Contains(out, "| Database Connectors | `okf-sqlite`, `okf-csv` |") {
		t.Fatalf("group row not updated:\n%s", out)
	}
	if again, _ := wireReadme(out, "okf-csv", "CSV File", "Database Connectors"); again != out {
		t.Fatal("not idempotent")
	}
}

func TestWireAgents(t *testing.T) {
	in := "│   ├── okf-sqlite/                # SQLite connector\n│   └── okf-viz/                   # Visualizer\n└── tests/\n"
	out, err := wireAgents(in, "okf-csv", "CSV File")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "├── okf-csv/") {
		t.Fatalf("tree line not added:\n%s", out)
	}
	if again, _ := wireAgents(out, "okf-csv", "CSV File"); again != out {
		t.Fatal("not idempotent")
	}
	if _, err := wireAgents("no tree here\n", "okf-csv", "CSV File"); err == nil {
		t.Fatal("expected error when tree entries missing")
	}
}
