package okf

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LintReport is the deterministic, no-LLM result of scanning an OKF bundle: spec
// conformance findings plus enrichment-coverage signal. It never judges description
// quality and never mutates the bundle. Listings are sorted for byte-stable output.
//
// It is the single shared scanner used by both okf-lint (which gates on it) and
// okf-viz's `coverage` command (which formats it).
type LintReport struct {
	TotalConcepts       int       `json:"total_concepts"`
	Placeholders        int       `json:"placeholders"`
	EnrichedConcepts    int       `json:"enriched_concepts"`
	EnrichedPct         float64   `json:"enriched_pct"`
	ColumnsTotal        int       `json:"columns_total"`
	ColumnsCommented    int       `json:"columns_commented"`
	ColumnsCommentedPct float64   `json:"columns_commented_pct"`
	BrokenLinks         []string  `json:"broken_links"` // "source -> target", sorted
	MissingType         []string  `json:"missing_type"` // concept ids, sorted
	Orphans             []string  `json:"orphans"`      // cross-link-degree-0 concept ids, sorted
	EnrichFirst         []string  `json:"enrich_first"` // placeholder concept ids, most-referenced first
	Conformance         []Finding `json:"conformance"`  // spec-conformance violations, sorted
}

// Finding is a single spec-conformance violation.
type Finding struct {
	Rule   string `json:"rule"`   // machine-readable rule id
	Path   string `json:"path"`   // bundle-relative file
	Detail string `json:"detail"` // human-readable explanation
}

// Conformance rule ids.
const (
	RuleRootIndexExtraKeys = "root-index-extra-keys"
	RuleRootIndexVersion   = "root-index-okf-version"
	RuleSubdirIndexFM      = "subdir-index-frontmatter"
	RuleConceptUnparseable = "concept-unparseable"
	RuleMissingType        = "concept-missing-type"
)

var lintMarkdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// ScanBundle walks an OKF bundle directory and returns its deterministic LintReport.
func ScanBundle(dir string) (*LintReport, error) {
	r := &LintReport{}

	type concept struct {
		id          string
		body        string
		placeholder bool
	}
	var concepts []concept
	ids := map[string]bool{}

	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		base := filepath.Base(rel)

		switch base {
		case "index.md":
			r.checkIndex(rel, p)
			return nil
		case "log.md":
			return nil // reserved, not a concept
		}

		id := strings.TrimSuffix(rel, ".md")
		fm, body, ferr := readFrontmatter(p)
		if ferr != nil {
			r.Conformance = append(r.Conformance, Finding{
				Rule: RuleConceptUnparseable, Path: rel,
				Detail: "concept has no parseable YAML frontmatter: " + ferr.Error(),
			})
			return nil
		}
		r.TotalConcepts++
		if strings.TrimSpace(fm.Type) == "" {
			r.MissingType = append(r.MissingType, id)
			r.Conformance = append(r.Conformance, Finding{
				Rule: RuleMissingType, Path: rel, Detail: "concept frontmatter has an empty `type`",
			})
		}
		ph := IsPlaceholderDescription(fm.Description)
		if ph {
			r.Placeholders++
		} else {
			r.EnrichedConcepts++
		}
		total, commented := commentStats(body)
		r.ColumnsTotal += total
		r.ColumnsCommented += commented

		concepts = append(concepts, concept{id: id, body: body, placeholder: ph})
		ids[id] = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Second pass: resolve cross-links for broken-link detection + cross-link degree
	// (concepts sorted by id so dedup and output stay deterministic).
	sort.Slice(concepts, func(i, j int) bool { return concepts[i].id < concepts[j].id })
	degree := map[string]int{}
	for _, c := range concepts {
		for _, m := range lintMarkdownLink.FindAllStringSubmatch(c.body, -1) {
			target := resolveBundleLink(c.id, m[2])
			if target == "" || target == c.id {
				continue
			}
			if !ids[target] {
				r.BrokenLinks = append(r.BrokenLinks, c.id+" -> "+target)
				continue
			}
			degree[c.id]++
			degree[target]++
		}
	}

	// Orphans (no cross-links) and the degree-ranked "enrich first" placeholder list.
	type cand struct {
		id     string
		degree int
	}
	var cands []cand
	for _, c := range concepts {
		if degree[c.id] == 0 {
			r.Orphans = append(r.Orphans, c.id)
		}
		if c.placeholder {
			cands = append(cands, cand{id: c.id, degree: degree[c.id]})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].degree != cands[j].degree {
			return cands[i].degree > cands[j].degree
		}
		return cands[i].id < cands[j].id
	})
	for _, c := range cands {
		r.EnrichFirst = append(r.EnrichFirst, c.id)
	}

	if r.TotalConcepts > 0 {
		r.EnrichedPct = round1(100 * float64(r.EnrichedConcepts) / float64(r.TotalConcepts))
	}
	if r.ColumnsTotal > 0 {
		r.ColumnsCommentedPct = round1(100 * float64(r.ColumnsCommented) / float64(r.ColumnsTotal))
	}
	sort.Strings(r.BrokenLinks)
	sort.Strings(r.MissingType)
	sort.Strings(r.Orphans)
	sort.Slice(r.Conformance, func(i, j int) bool {
		if r.Conformance[i].Path != r.Conformance[j].Path {
			return r.Conformance[i].Path < r.Conformance[j].Path
		}
		return r.Conformance[i].Rule < r.Conformance[j].Rule
	})
	return r, nil
}

// TextReport renders a deterministic text summary suitable for CI logs.
func (r *LintReport) TextReport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "bundle is %.1f%% enriched (placeholders: %d, broken links: %d, orphans: %d, conformance: %d)\n",
		r.EnrichedPct, r.Placeholders, len(r.BrokenLinks), len(r.Orphans), len(r.Conformance))
	fmt.Fprintf(&b, "- concepts: %d (%d enriched, %d placeholder)\n", r.TotalConcepts, r.EnrichedConcepts, r.Placeholders)
	if r.ColumnsTotal > 0 {
		fmt.Fprintf(&b, "- columns commented: %d/%d (%.1f%%)\n", r.ColumnsCommented, r.ColumnsTotal, r.ColumnsCommentedPct)
	}
	fmt.Fprintf(&b, "- concepts missing type: %d\n", len(r.MissingType))
	fmt.Fprintf(&b, "- orphan nodes (no cross-links): %d\n", len(r.Orphans))
	fmt.Fprintf(&b, "- broken cross-links: %d\n", len(r.BrokenLinks))
	for _, bl := range r.BrokenLinks {
		fmt.Fprintf(&b, "    %s\n", bl)
	}
	if len(r.Conformance) > 0 {
		fmt.Fprintf(&b, "- conformance violations: %d\n", len(r.Conformance))
		for _, f := range r.Conformance {
			fmt.Fprintf(&b, "    [%s] %s: %s\n", f.Rule, f.Path, f.Detail)
		}
	}
	if len(r.EnrichFirst) > 0 {
		fmt.Fprintf(&b, "- enrich first (most-referenced placeholders): %d\n", len(r.EnrichFirst))
		for _, id := range r.EnrichFirst {
			fmt.Fprintf(&b, "    %s\n", id)
		}
	}
	return b.String()
}

// checkIndex applies spec-conformance rules to an index.md file.
func (r *LintReport) checkIndex(rel, fullPath string) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return
	}
	isRoot := rel == "index.md"
	keys, hasFM := frontmatterKeys(data)

	if isRoot {
		// Root index.md may declare only okf_version, and must declare it.
		if !hasFM {
			r.Conformance = append(r.Conformance, Finding{
				Rule: RuleRootIndexVersion, Path: rel, Detail: "root index.md must declare okf_version in frontmatter",
			})
			return
		}
		for _, k := range keys {
			if k != "okf_version" {
				r.Conformance = append(r.Conformance, Finding{
					Rule: RuleRootIndexExtraKeys, Path: rel,
					Detail: fmt.Sprintf("root index.md frontmatter must contain only okf_version (found %q)", k),
				})
			}
		}
		hasVersion := false
		for _, k := range keys {
			if k == "okf_version" {
				hasVersion = true
			}
		}
		if !hasVersion {
			r.Conformance = append(r.Conformance, Finding{
				Rule: RuleRootIndexVersion, Path: rel, Detail: "root index.md frontmatter is missing okf_version",
			})
		}
		return
	}
	// Subdirectory index.md must carry no frontmatter.
	if hasFM {
		r.Conformance = append(r.Conformance, Finding{
			Rule: RuleSubdirIndexFM, Path: rel, Detail: "subdirectory index.md must not carry frontmatter",
		})
	}
}

// readFrontmatter parses a concept file's frontmatter and returns it with the body.
func readFrontmatter(p string) (Frontmatter, string, error) {
	content, err := os.ReadFile(p)
	if err != nil {
		return Frontmatter{}, "", err
	}
	parts := bytes.SplitN(content, []byte("---\n"), 3)
	if len(parts) < 3 {
		parts = bytes.SplitN(content, []byte("---\r\n"), 3)
		if len(parts) < 3 {
			return Frontmatter{}, "", fmt.Errorf("missing frontmatter boundaries")
		}
	}
	var fm Frontmatter
	if err := yaml.Unmarshal(parts[1], &fm); err != nil {
		return Frontmatter{}, "", err
	}
	return fm, string(parts[2]), nil
}

// frontmatterKeys returns the top-level frontmatter keys of a file and whether it
// has a frontmatter block at all.
func frontmatterKeys(content []byte) (keys []string, hasFrontmatter bool) {
	sep := []byte("---\n")
	if !bytes.HasPrefix(content, sep) {
		sep = []byte("---\r\n")
		if !bytes.HasPrefix(content, sep) {
			return nil, false
		}
	}
	parts := bytes.SplitN(content, sep, 3)
	if len(parts) < 3 {
		return nil, false
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(parts[1], &m); err != nil {
		return nil, true // has a block but unparseable; treat as present
	}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, true
}

// resolveBundleLink turns a markdown link target into a concept id (path without
// .md), or "" if external/not a bundle link. Absolute targets begin with "/".
func resolveBundleLink(srcID, target string) string {
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return ""
	}
	if !strings.HasSuffix(target, ".md") {
		return ""
	}
	var p string
	if strings.HasPrefix(target, "/") {
		p = strings.TrimPrefix(target, "/")
	} else {
		p = path.Join(path.Dir(srcID), target)
	}
	p = path.Clean(p)
	return strings.TrimSuffix(p, ".md")
}

// round1 rounds to one decimal place deterministically.
func round1(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }

// commentStats parses the "# Columns" table and, when it carries a Comment or
// Description column, returns (total columns, columns with a non-empty comment).
func commentStats(body string) (total, commented int) {
	section, ok := GetSectionAny(body, "Columns")
	if !ok {
		return 0, 0
	}
	var header []string
	commentCol := -1
	for _, ln := range strings.Split(section, "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "|") {
			continue
		}
		cells := splitTableCells(ln)
		if header == nil {
			header = cells
			for i, h := range cells {
				h = strings.ToLower(strings.TrimSpace(h))
				if h == "comment" || h == "description" {
					commentCol = i
				}
			}
			continue
		}
		if isDividerRow(ln) {
			continue
		}
		if commentCol < 0 {
			return 0, 0
		}
		total++
		if commentCol < len(cells) && strings.TrimSpace(cells[commentCol]) != "" {
			commented++
		}
	}
	return total, commented
}

func splitTableCells(row string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

var dividerCell = regexp.MustCompile(`^:?-+:?$`)

// isDividerRow reports whether a table row is the |---|---| divider — every cell is
// a divider cell (so a data row whose cell legitimately contains "---" is not
// mistaken for the divider).
func isDividerRow(row string) bool {
	cells := splitTableCells(row)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if !dividerCell.MatchString(c) {
			return false
		}
	}
	return true
}
