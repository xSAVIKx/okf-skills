package okf

import (
	"strings"
)

// isHeading reports whether a trimmed line begins a level-1 or level-2 ATX heading.
// These delimit the boundaries of a level-2 section managed by UpsertSection.
func isHeading(trimmed string) bool {
	return strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ")
}

// UpsertSection inserts or replaces a level-2 ("## heading") section in a markdown
// body. If the section already exists, its content — from the heading line up to
// the next level-1/level-2 heading or end of body — is replaced. Otherwise the
// section is appended to the end of the body. Surrounding content is preserved.
func UpsertSection(body, heading, content string) string {
	marker := "## " + heading
	section := marker + "\n\n" + strings.TrimRight(content, "\n") + "\n"

	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == marker {
			start = i
			break
		}
	}

	if start == -1 {
		trimmed := strings.TrimRight(body, "\n")
		if trimmed == "" {
			return section
		}
		return trimmed + "\n\n" + section
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isHeading(strings.TrimSpace(lines[i])) {
			end = i
			break
		}
	}

	before := strings.TrimRight(strings.Join(lines[:start], "\n"), "\n")
	after := strings.TrimLeft(strings.Join(lines[end:], "\n"), "\n")

	var b strings.Builder
	if before != "" {
		b.WriteString(before)
		b.WriteString("\n\n")
	}
	b.WriteString(section)
	if after != "" {
		b.WriteString("\n")
		b.WriteString(after)
		if !strings.HasSuffix(after, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// GetSection returns the content of the named level-2 section (excluding the
// heading line, trimmed) and whether it was found.
func GetSection(body, heading string) (string, bool) {
	marker := "## " + heading
	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == marker {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isHeading(strings.TrimSpace(lines[i])) {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start+1:end], "\n")), true
}

// headingTextEquals reports whether a trimmed line is an ATX heading
// ("# ", "## ", "### ", …) whose text equals want. For example, both
// "# Columns" and "## Columns" match want "Columns".
func headingTextEquals(trimmed, want string) bool {
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	rest := strings.TrimLeft(trimmed, "#")
	if !strings.HasPrefix(rest, " ") {
		return false // e.g. "#Columns" (no space) is not an ATX heading
	}
	return strings.TrimSpace(rest) == want
}

// GetSectionAny returns the content beneath the first heading whose text equals
// heading, matched at ANY ATX level (e.g. "# Columns" or "## Columns"), up to
// the next level-1/level-2 heading or end of body, plus whether it was found.
//
// GetSection matches only a level-2 "## heading"; GetSectionAny additionally
// matches the level-1 "# Columns" heading that the connectors emit for their
// schema table. Isolating that section lets a column parser read the schema
// table without mistaking appended "## Data Profile" / "## Sample" tables for
// schema rows.
func GetSectionAny(body, heading string) (string, bool) {
	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if headingTextEquals(strings.TrimSpace(ln), heading) {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isHeading(strings.TrimSpace(lines[i])) {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start+1:end], "\n")), true
}
