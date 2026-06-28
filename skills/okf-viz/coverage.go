package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// Coverage is the deterministic, no-LLM enrichment coverage of a bundle. It counts
// signal; it never judges description quality (that is the eval harness) and never
// mutates the bundle.
type Coverage struct {
	TotalConcepts       int      `json:"total_concepts"`
	Placeholders        int      `json:"placeholders"`
	EnrichedConcepts    int      `json:"enriched_concepts"`
	EnrichedPct         float64  `json:"enriched_pct"`
	ColumnsTotal        int      `json:"columns_total"`
	ColumnsCommented    int      `json:"columns_commented"`
	ColumnsCommentedPct float64  `json:"columns_commented_pct"`
	BrokenLinks         []string `json:"broken_links"` // "source -> target", sorted
	MissingType         []string `json:"missing_type"` // concept ids, sorted
	Orphans             []string `json:"orphans"`      // degree-0 concept ids, sorted
	EnrichFirst         []string `json:"enrich_first"` // placeholder concept ids, most-referenced (highest degree) first
}

// ComputeCoverage scans a built model (after addCrossLinks has set Degree) and
// returns its coverage metrics. Listings are sorted by path for byte-stable output.
func ComputeCoverage(m *Model) Coverage {
	var c Coverage
	exists := map[string]bool{}
	degree := map[string]int{}
	for _, n := range m.Nodes {
		exists[n.ID] = true
		degree[n.ID] = n.Degree
	}

	// candidate is an unenriched concept plus the signal we rank it by: a concept
	// many others link to (or FK at) is read most, so it deserves a description
	// first. This is the deterministic "enrich these first" list okf-enrich triages.
	type candidate struct {
		id     string
		degree int
	}
	var candidates []candidate

	for _, n := range m.Nodes {
		if n.Kind != "concept" {
			continue
		}
		c.TotalConcepts++
		if okf.IsPlaceholderDescription(n.Description) {
			c.Placeholders++
			candidates = append(candidates, candidate{id: n.ID, degree: n.Degree})
		} else {
			c.EnrichedConcepts++
		}
		if strings.TrimSpace(n.Type) == "" {
			c.MissingType = append(c.MissingType, n.ID)
		}
		if degree[n.ID] == 0 {
			c.Orphans = append(c.Orphans, n.ID)
		}

		doc := m.concepts[n.ID]
		if doc != nil {
			total, commented := commentStats(doc.Body)
			c.ColumnsTotal += total
			c.ColumnsCommented += commented
			c.BrokenLinks = append(c.BrokenLinks, brokenLinks(n.ID, doc.Body, exists)...)
		}
	}

	if c.TotalConcepts > 0 {
		c.EnrichedPct = round1(100 * float64(c.EnrichedConcepts) / float64(c.TotalConcepts))
	}
	if c.ColumnsTotal > 0 {
		c.ColumnsCommentedPct = round1(100 * float64(c.ColumnsCommented) / float64(c.ColumnsTotal))
	}
	sort.Strings(c.BrokenLinks)
	sort.Strings(c.MissingType)
	sort.Strings(c.Orphans)

	// Rank unenriched concepts: highest degree first, ties broken by id for a
	// byte-stable order.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].degree != candidates[j].degree {
			return candidates[i].degree > candidates[j].degree
		}
		return candidates[i].id < candidates[j].id
	})
	for _, cand := range candidates {
		c.EnrichFirst = append(c.EnrichFirst, cand.id)
	}
	return c
}

// round1 rounds to one decimal place deterministically.
func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}

// commentStats parses the "# Columns" table and, when it carries a Comment or
// Description column, returns (total columns, columns with a non-empty comment).
// Connectors without a comment column contribute (0, 0).
func commentStats(body string) (total, commented int) {
	section, ok := okf.GetSectionAny(body, "Columns")
	if !ok {
		return 0, 0
	}
	lines := strings.Split(section, "\n")
	var header []string
	commentCol := -1
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "|") {
			continue
		}
		cells := splitRow(ln)
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
			return 0, 0 // no comment column → connector doesn't carry comments
		}
		total++
		if commentCol < len(cells) && strings.TrimSpace(cells[commentCol]) != "" {
			commented++
		}
	}
	return total, commented
}

// brokenLinks returns "src -> target" for each body markdown link whose resolved
// bundle target is not a known node (external/self links are skipped).
func brokenLinks(srcID, body string, exists map[string]bool) []string {
	var out []string
	for _, match := range markdownLink.FindAllStringSubmatch(body, -1) {
		target := resolveLink(srcID, match[2]) // match[1]=text, match[2]=href
		if target == "" || target == srcID {
			continue
		}
		if !exists[target] {
			out = append(out, srcID+" -> "+target)
		}
	}
	return out
}

// splitRow splits a markdown table row into trimmed cells, dropping the leading
// and trailing empty cells produced by the surrounding pipes.
func splitRow(row string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// dividerCell matches a GFM table divider cell: dashes with optional alignment
// colons (e.g. "---", ":--", "--:", ":-:").
var dividerCell = regexp.MustCompile(`^:?-+:?$`)

// isDividerRow reports whether a table row is the |---|---| divider — every cell
// is a divider cell. A plain strings.Contains(row, "---") would also match a data
// row whose cells legitimately contain "---" (a default value or comment),
// silently dropping that column from the count.
func isDividerRow(row string) bool {
	cells := splitRow(row)
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

// Report renders a deterministic text summary suitable for CI logs.
func (c Coverage) Report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "bundle is %.1f%% enriched (placeholders: %d, broken links: %d, orphans: %d)\n",
		c.EnrichedPct, c.Placeholders, len(c.BrokenLinks), len(c.Orphans))
	fmt.Fprintf(&b, "- concepts: %d (%d enriched, %d placeholder)\n", c.TotalConcepts, c.EnrichedConcepts, c.Placeholders)
	if c.ColumnsTotal > 0 {
		fmt.Fprintf(&b, "- columns commented: %d/%d (%.1f%%)\n", c.ColumnsCommented, c.ColumnsTotal, c.ColumnsCommentedPct)
	}
	fmt.Fprintf(&b, "- concepts missing type: %d\n", len(c.MissingType))
	fmt.Fprintf(&b, "- orphan nodes (degree 0): %d\n", len(c.Orphans))
	fmt.Fprintf(&b, "- broken cross-links: %d\n", len(c.BrokenLinks))
	for _, bl := range c.BrokenLinks {
		fmt.Fprintf(&b, "    %s\n", bl)
	}
	if len(c.EnrichFirst) > 0 {
		fmt.Fprintf(&b, "- enrich first (most-referenced placeholders): %d\n", len(c.EnrichFirst))
		for _, id := range c.EnrichFirst {
			fmt.Fprintf(&b, "    %s\n", id)
		}
	}
	return b.String()
}
