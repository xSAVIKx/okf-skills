package okf

import (
	"fmt"
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
