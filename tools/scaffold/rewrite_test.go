package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteSkillMD(t *testing.T) {
	in := "---\nname: okf-sqlite\ndescription: old desc\nlicense: Apache-2.0\nmetadata:\n  version: \"0.7.0\"\n  tags: \"okf, sqlite\"\n---\n\n# SQLite\n\nbody about okf-sqlite.\n"
	// slug replace happens in rewriteTree; here test the frontmatter rewrite directly
	out := rewriteSkillMD(strings.ReplaceAll(in, "okf-sqlite", "okf-csv"), "okf-csv", "new desc here")
	for _, want := range []string{
		"name: okf-csv",
		"description: new desc here",
		`version: "0.1.0"`,
		`tags: "okf, knowledge-catalog, csv"`,
		"TODO(okf-scaffold): rewrite the prose",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRewriteSchemaDesc(t *testing.T) {
	in := "Name:        \"okf-csv\",\n\t\tDescription: \"old\",\n\t\tCommands: []okf.CommandSchema{\n\t\t\t{Description: \"produce desc\"},\n"
	out := rewriteSchemaDesc(in, "okf-csv", "new skill desc")
	if !strings.Contains(out, `Description: "new skill desc"`) {
		t.Fatalf("skill description not rewritten:\n%s", out)
	}
	if !strings.Contains(out, `Description: "produce desc"`) {
		t.Fatalf("per-command description must be preserved:\n%s", out)
	}
}

func TestRewriteMainGo(t *testing.T) {
	in := "package main\n\nimport \"fmt\"\n\nfunc runProduce(args []string) {}\nfunc runIngest(args []string) {}\n"
	out, marks := rewriteMainGo(in, "okf-sqlite", "okf-csv", "CSV File", "files", "file")
	for _, want := range []string{
		"TODO(okf-scaffold): this skeleton was copied verbatim from okf-sqlite",
		`Type: "CSV File"`,
		`"files" subdirectory`,
		"TODO(okf-scaffold): replace with okf-csv produce logic.",
		"TODO(okf-scaffold): replace with okf-csv ingest logic.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if len(marks) < 3 {
		t.Errorf("expected >=3 markers, got %d: %v", len(marks), marks)
	}
}

// TestRewriteTreeSlugReplace verifies the end-to-end slug replacement on a tiny tree.
func TestRewriteTreeSlugReplace(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module github.com/xSAVIKx/okf-skills/skills/okf-sqlite\n")
	write("schema.go", "Name:        \"okf-sqlite\",\n\t\tDescription: \"old\",\n")
	write("schema_test.go", "if s.Name != \"okf-sqlite\" {\n")
	write("main.go", "package main\n\nfunc runProduce(args []string) {}\nfunc runIngest(args []string) {}\n")
	write("SKILL.md", "---\nname: okf-sqlite\ndescription: d\nmetadata:\n  version: \"0.7.0\"\n  tags: \"okf, sqlite\"\n---\n# x\n")

	if _, err := rewriteTree(dir, "okf-sqlite", "okf-csv", "CSV File", "files", "desc", "file"); err != nil {
		t.Fatal(err)
	}
	gomod, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	if !strings.Contains(string(gomod), "skills/okf-csv") {
		t.Errorf("go.mod module path not rewritten: %s", gomod)
	}
	st, _ := os.ReadFile(filepath.Join(dir, "schema_test.go"))
	if !strings.Contains(string(st), `"okf-csv"`) {
		t.Errorf("schema_test assertion not rewritten: %s", st)
	}
}
