package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// rewriteTree adapts the copied reference tree in place for the new skill:
//   - replaces the reference slug (okf-<ref>) with the new slug everywhere, fixing
//     the module path, schema Name, the schema_test assertion, SKILL.md name, and
//     binary/install paths;
//   - rewrites SKILL.md frontmatter (description, tags, version) and the schema.go
//     skill-level Description to the new connector;
//   - inserts TODO(okf-scaffold) markers showing the concept type, bundle subdir,
//     and the source-specific produce/ingest seams to replace.
//
// It returns human-readable "file:what" locations of the TODO markers for the report.
func rewriteTree(skillDir, refSlug, skill, typ, subdir, desc, shape string) ([]string, error) {
	var todos []string
	err := filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s, crlf := toLF(string(data))
		base := d.Name()

		// 1. Slug replace everywhere.
		s = strings.ReplaceAll(s, refSlug, skill)

		// 2. File-specific edits.
		switch {
		case base == "SKILL.md":
			s = rewriteSkillMD(s, skill, desc)
			todos = append(todos, relPath(skillDir, path, skill)+": rewrite the SKILL.md body (still describes "+refSlug+")")
		case base == "schema.go":
			s = rewriteSchemaDesc(s, skill, desc)
		case base == "main.go":
			var marks []string
			s, marks = rewriteMainGo(s, refSlug, skill, typ, subdir, shape)
			for _, m := range marks {
				todos = append(todos, relPath(skillDir, path, skill)+": "+m)
			}
		}

		return os.WriteFile(path, []byte(fromLF(s, crlf)), 0o644)
	})
	return todos, err
}

func relPath(skillDir, path, skill string) string {
	rel, err := filepath.Rel(skillDir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	return filepath.ToSlash(filepath.Join("skills", skill, rel))
}

var (
	reDescription = regexp.MustCompile(`(?m)^description:.*$`)
	reTags        = regexp.MustCompile(`(?m)^(\s*)tags:.*$`)
	reVersion     = regexp.MustCompile(`(?m)^(\s*)version:.*$`)
	reFrontEnd    = regexp.MustCompile(`(?ms)\A(---\n.*?\n---\n)`)
)

// rewriteSkillMD updates the frontmatter description/tags/version and inserts a
// TODO marker after the frontmatter noting the body still describes the reference.
func rewriteSkillMD(s, skill, desc string) string {
	s = replaceLine(reDescription, s, "description: "+desc)
	name := strings.TrimPrefix(skill, "okf-")
	s = reTags.ReplaceAllStringFunc(s, func(m string) string {
		indent := reTags.FindStringSubmatch(m)[1]
		return indent + `tags: "okf, knowledge-catalog, ` + name + `"`
	})
	s = reVersion.ReplaceAllStringFunc(s, func(m string) string {
		indent := reVersion.FindStringSubmatch(m)[1]
		return indent + `version: "0.1.0"`
	})
	// Insert a TODO note right after the closing frontmatter fence.
	if loc := reFrontEnd.FindStringIndex(s); loc != nil {
		note := "\n<!-- TODO(okf-scaffold): rewrite the prose below for " + skill + "; it was copied from the reference connector. -->\n"
		s = s[:loc[1]] + note + s[loc[1]:]
	}
	return s
}

var reSchemaSkillDesc = regexp.MustCompile(`(Name:\s*"` + regexp.QuoteMeta("okf-") + `[a-z0-9-]+",\s*\n\s*Description:\s*)"[^"]*"`)

// rewriteSchemaDesc replaces the skill-level Description in schema.go (the one that
// immediately follows the skill Name), leaving per-command descriptions intact.
func rewriteSchemaDesc(s, skill, desc string) string {
	return reSchemaSkillDesc.ReplaceAllStringFunc(s, func(m string) string {
		sub := reSchemaSkillDesc.FindStringSubmatch(m)
		return sub[1] + goQuote(desc)
	})
}

var reRunProduce = regexp.MustCompile(`(?m)^func runProduce\(`)
var reRunIngest = regexp.MustCompile(`(?m)^func runIngest\(`)

// rewriteMainGo inserts a TODO banner after `package main` and inline TODO markers
// before runProduce/runIngest. Returns descriptions of the markers added.
func rewriteMainGo(s, refSlug, skill, typ, subdir, shape string) (string, []string) {
	var marks []string

	banner := fmt.Sprintf(`
// TODO(okf-scaffold): this skeleton was copied verbatim from %s (shape=%s) and
// still contains its source-specific extraction logic. To finish the connector:
//   - emit concept docs with Type: %q
//   - write concept docs under the %q subdirectory
//   - replace the produce/ingest source logic below (and drop the %s driver dep)
`, refSlug, shape, typ, subdir, refSlug)
	if i := strings.Index(s, "package main\n"); i >= 0 {
		end := i + len("package main\n")
		s = s[:end] + banner + s[end:]
		marks = append(marks, "TODO banner after `package main`")
	}

	if reRunProduce.MatchString(s) {
		s = reRunProduce.ReplaceAllString(s, "// TODO(okf-scaffold): replace with "+skill+" produce logic.\nfunc runProduce(")
		marks = append(marks, "marker before func runProduce")
	}
	if reRunIngest.MatchString(s) {
		s = reRunIngest.ReplaceAllString(s, "// TODO(okf-scaffold): replace with "+skill+" ingest logic.\nfunc runIngest(")
		marks = append(marks, "marker before func runIngest")
	}

	return s, marks
}

// replaceLine replaces the first line matching re with repl (no $-expansion).
func replaceLine(re *regexp.Regexp, s, repl string) string {
	done := false
	return re.ReplaceAllStringFunc(s, func(m string) string {
		if done {
			return m
		}
		done = true
		return repl
	})
}

// goQuote quotes a Go string literal for embedding in source.
func goQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
