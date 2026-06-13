package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/savikne/okf-skills-registry/okf-go"
)

// needsDescription reports whether a concept's description should be generated.
// By default only empty descriptions are filled; overwrite forces regeneration.
func needsDescription(desc string, overwrite bool) bool {
	return overwrite || strings.TrimSpace(desc) == ""
}

// buildPrompt constructs the LLM user prompt from a concept's title and body.
func buildPrompt(title, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Concept title: %s\n\n", title)
	b.WriteString("Concept details (OKF markdown):\n")
	b.WriteString(strings.TrimSpace(body))
	return b.String()
}

// enrichBundle walks the concept docs under bundleDir, generates descriptions
// for those that need one, writes them back to frontmatter, and returns the
// number of documents changed. index.md and log.md are skipped.
func enrichBundle(ctx context.Context, bundleDir string, gen Generator, overwrite bool) (int, error) {
	var changed int
	err := filepath.WalkDir(bundleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		base := filepath.Base(path)
		if base == "index.md" || base == "log.md" {
			return nil
		}
		doc, err := okf.ReadConceptDoc(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if !needsDescription(doc.Frontmatter.Description, overwrite) {
			return nil
		}
		title := doc.Frontmatter.Title
		if title == "" {
			title = strings.TrimSuffix(base, ".md")
		}
		desc, err := gen.Describe(ctx, buildPrompt(title, doc.Body))
		if err != nil {
			return fmt.Errorf("describe %s: %w", path, err)
		}
		if desc = strings.TrimSpace(desc); desc == "" {
			return nil
		}
		doc.Frontmatter.Description = desc
		if err := okf.WriteConceptDoc(path, *doc); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		changed++
		return nil
	})
	return changed, err
}
