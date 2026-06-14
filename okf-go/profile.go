package okf

import (
	"fmt"
	"sort"
	"strings"
)

// ColumnProfile holds basic per-column statistics computed from a table's data.
// It is the spec representation embedded in a concept doc's "Data Profile" section.
type ColumnProfile struct {
	Column   string // Column name
	NonNull  int64  // Count of non-NULL values
	Null     int64  // Count of NULL values
	Distinct int64  // Count of distinct non-NULL values
	Min      string // Minimum value rendered as text
	Max      string // Maximum value rendered as text
}

// Constraint is a non-primary-key table constraint surfaced from the catalog:
// a UNIQUE or CHECK constraint. A unique constraint is a strong grounding signal
// ("this column identifies a row"); a check constraint documents an invariant.
type Constraint struct {
	Name       string // constraint name
	Type       string // "UNIQUE" or "CHECK"
	Definition string // columns (unique) or expression (check)
}

// Index describes a table index — a hint at access patterns.
type Index struct {
	Name    string   // index name
	Columns []string // indexed columns, in order
	Unique  bool     // whether the index enforces uniqueness
}

// TableStats holds cheap, opt-in table-level statistics: an approximate or exact
// row count and a freshness window over a detected timestamp column. Populated
// behind the connectors' --stats flag.
type TableStats struct {
	RowCount        int64  // number of rows (exact or dialect-cheap estimate)
	HasRowCount     bool   // whether RowCount was computed
	FreshnessColumn string // detected timestamp column, "" if none
	Earliest        string // min value of FreshnessColumn, rendered as text
	Latest          string // max value of FreshnessColumn, rendered as text
}

// RenderConstraintsSection renders constraints as a markdown table suitable for
// UpsertSection(body, "Constraints", ...). Output is sorted by (Name, Type) for
// byte-stability; an empty slice renders "" so no section is emitted.
func RenderConstraintsSection(cons []Constraint) string {
	if len(cons) == 0 {
		return ""
	}
	sorted := make([]Constraint, len(cons))
	copy(sorted, cons)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Type < sorted[j].Type
	})
	var b strings.Builder
	b.WriteString("| Name | Type | Definition |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, c := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", SanitizeCell(c.Name), SanitizeCell(c.Type), SanitizeCell(c.Definition))
	}
	return b.String()
}

// RenderIndexesSection renders indexes as a markdown table suitable for
// UpsertSection(body, "Indexes", ...). Sorted by Name; empty renders "".
func RenderIndexesSection(idx []Index) string {
	if len(idx) == 0 {
		return ""
	}
	sorted := make([]Index, len(idx))
	copy(sorted, idx)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	var b strings.Builder
	b.WriteString("| Name | Columns | Unique |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, ix := range sorted {
		unique := "No"
		if ix.Unique {
			unique = "Yes"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", SanitizeCell(ix.Name), SanitizeCell(strings.Join(ix.Columns, ", ")), unique)
	}
	return b.String()
}

// RenderStatsSection renders table-level statistics as a bullet list suitable for
// UpsertSection(body, "Stats", ...). Renders "" when no stats are present so no
// empty section is emitted.
func RenderStatsSection(s TableStats) string {
	var b strings.Builder
	if s.HasRowCount {
		fmt.Fprintf(&b, "- **Row Count**: %d\n", s.RowCount)
	}
	if s.FreshnessColumn != "" && (s.Earliest != "" || s.Latest != "") {
		fmt.Fprintf(&b, "- **Freshness** (`%s`): %s … %s\n",
			SanitizeCell(s.FreshnessColumn), SanitizeCell(s.Earliest), SanitizeCell(s.Latest))
	}
	return b.String()
}

// RenderViewDefinition renders a view's defining SQL as a fenced code block,
// suitable for UpsertSection(body, "View Definition", ...). Empty SQL renders "".
func RenderViewDefinition(viewSQL string) string {
	viewSQL = strings.TrimSpace(viewSQL)
	if viewSQL == "" {
		return ""
	}
	return "```sql\n" + viewSQL + "\n```\n"
}

// RenderProfileSection renders column profiles as a markdown table suitable for
// embedding via UpsertSection(body, "Data Profile", RenderProfileSection(...)).
func RenderProfileSection(profiles []ColumnProfile) string {
	var b strings.Builder
	b.WriteString("| Column | Non-Null | Null | Distinct | Min | Max |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, p := range profiles {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %s | %s |\n",
			SanitizeCell(p.Column), p.NonNull, p.Null, p.Distinct,
			SanitizeCell(p.Min), SanitizeCell(p.Max))
	}
	return b.String()
}

// RenderSampleSection renders sample rows as a markdown table suitable for
// embedding via UpsertSection(body, "Sample", RenderSampleSection(...)).
func RenderSampleSection(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	var b strings.Builder
	sani := make([]string, len(headers))
	seps := make([]string, len(headers))
	for i, h := range headers {
		sani[i] = SanitizeCell(h)
		seps[i] = "---"
	}
	b.WriteString("| " + strings.Join(sani, " | ") + " |\n")
	b.WriteString("| " + strings.Join(seps, " | ") + " |\n")
	for _, r := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			if i < len(r) {
				cells[i] = SanitizeCell(r[i])
			}
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return b.String()
}

// SanitizeCell makes a value safe for a single markdown table cell by escaping
// pipes and flattening newlines.
func SanitizeCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}
