// Command scaffold generates a green-building, fully-registered skeleton for a new
// OKF connector skill under skills/okf-<name>/, copied from the nearest reference
// connector and wired into every registration file (go.work, Makefile, install.sh,
// skills.sh.json, README.md, AGENTS.md).
//
// It codifies the okf-producer-generator registration checklist into one command so
// the author can `make build` immediately and then replace only the source-specific
// extraction logic (marked with TODO(okf-scaffold) comments in the generated tree).
//
// Usage:
//
//	go run ./tools/scaffold -name csv -type "CSV File" -shape file [-subdir files] [-desc "..."]
//
// It is a development tool, not an okf-* skill: it has no produce/ingest/schema
// surface and never appears in skills.sh.json.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// reference describes a shape's source connector and its registration defaults.
type reference struct {
	slug          string // reference skill slug, e.g. "okf-sqlite"
	defaultSubdir string // bundle subdirectory for concept docs
	group         string // skills.sh.json grouping title + README §8 group row
}

// shapes maps the -shape value to the reference connector it copies and adapts.
var shapes = map[string]reference{
	"tabular":     {slug: "okf-sqlite", defaultSubdir: "tables", group: "Database Connectors"},
	"db-comments": {slug: "okf-mysql", defaultSubdir: "tables", group: "Database Connectors"},
	"file":        {slug: "okf-fs", defaultSubdir: "files", group: "Filesystem & Git"},
}

var nameRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func main() {
	fs := flag.NewFlagSet("scaffold", flag.ExitOnError)
	name := fs.String("name", "", "connector slug (lowercase), e.g. csv -> skill okf-csv (required)")
	typ := fs.String("type", "", "concept `type` frontmatter string, e.g. \"CSV File\" (required)")
	shape := fs.String("shape", "", "source shape: tabular | db-comments | file (required)")
	subdir := fs.String("subdir", "", "bundle subdirectory for concept docs (default: by shape)")
	desc := fs.String("desc", "", "one-line description for SKILL.md and the doc tables (default: derived from -type)")
	fs.Parse(os.Args[1:])

	if err := run(*name, *typ, *shape, *subdir, *desc); err != nil {
		fmt.Fprintf(os.Stderr, "scaffold: %v\n", err)
		os.Exit(1)
	}
}

func run(name, typ, shape, subdir, desc string) error {
	// --- validate inputs (write nothing on failure) ---
	if name == "" || typ == "" || shape == "" {
		return fmt.Errorf("-name, -type and -shape are all required")
	}
	if !nameRE.MatchString(name) {
		return fmt.Errorf("-name %q must be lowercase letters/digits with single hyphens (e.g. csv, openapi, my-source)", name)
	}
	ref, ok := shapes[shape]
	if !ok {
		return fmt.Errorf("-shape %q is not one of: tabular, db-comments, file", shape)
	}
	if subdir == "" {
		subdir = ref.defaultSubdir
	}
	if desc == "" {
		desc = fmt.Sprintf("Produce and ingest OKF bundles from %s sources, and sync descriptions back.", typ)
	}
	skill := "okf-" + name

	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	skillDir := filepath.Join(root, "skills", skill)
	if _, err := os.Stat(skillDir); err == nil {
		return fmt.Errorf("%s already exists — refusing to overwrite", filepath.ToSlash(filepath.Join("skills", skill)))
	}
	refDir := filepath.Join(root, "skills", ref.slug)
	if _, err := os.Stat(refDir); err != nil {
		return fmt.Errorf("reference skill %s not found at %s: %w", ref.slug, refDir, err)
	}

	// --- compute all wiring edits in memory FIRST, so a missing anchor aborts
	// before anything touches disk (all-or-nothing). ---
	plan, err := planWiring(root, skill, typ, desc, ref.group)
	if err != nil {
		return err
	}

	// --- copy + rewrite the reference tree ---
	if err := copyTree(refDir, skillDir); err != nil {
		return fmt.Errorf("copy reference tree: %w", err)
	}
	todos, err := rewriteTree(skillDir, ref.slug, skill, typ, subdir, desc, shape)
	if err != nil {
		// best-effort cleanup of the partial copy so a failed run leaves no dir
		os.RemoveAll(skillDir)
		return fmt.Errorf("rewrite tree: %w", err)
	}

	// --- apply the (already validated) wiring edits ---
	if err := plan.write(); err != nil {
		os.RemoveAll(skillDir)
		return fmt.Errorf("write wiring: %w", err)
	}

	report(skill, ref.slug, shape, todos)
	return nil
}

// report prints the next-steps summary.
func report(skill, refSlug, shape string, todos []string) {
	fmt.Printf("✓ scaffolded %s (from %s, shape=%s)\n\n", skill, refSlug, shape)
	fmt.Println("Wired into: go.work, Makefile, install.sh, skills.sh.json, README.md, AGENTS.md")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. make build                    # the skeleton already builds green")
	fmt.Println("  2. replace the source logic at the TODO(okf-scaffold) markers:")
	for _, t := range todos {
		fmt.Printf("       %s\n", t)
	}
	fmt.Printf("  3. cd skills/%s && go mod tidy   # drop the reference's now-unused source driver\n", skill)
	fmt.Printf("  4. skills-ref validate skills/%s\n", skill)
}
