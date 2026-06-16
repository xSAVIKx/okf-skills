package okf

import (
	"fmt"
	"sort"
	"strings"
)

// Relationship is one directed edge from the current concept to another concept
// in the bundle, expressed as a bundle-relative target plus a short label. It is
// the deterministic, connector-emitted form of a structural relationship (a
// foreign key, a co-change pair, …); okf-enrich later explains what the
// relationship *means* in prose. Connectors must not embed any LLM to build one.
type Relationship struct {
	Label  string // e.g. "FK on customer_id", "co-changed (42x)"
	Target string // bundle-relative link target, e.g. "/tables/customers.md"
	Text   string // link text shown to the reader, e.g. "customers"
}

// RenderRelationshipsSection renders relationships as a markdown bullet list of
// links, suitable for embedding beneath a "# Relationships"/"# Related Files"
// heading. Each line is "- <Label> [<Text>](<Target>)" — an ordinary markdown
// link that okf-viz's links.go resolves unchanged. Edges are sorted by
// (Target, Label, Text) so re-runs over the same input are byte-identical,
// matching the determinism the profile/sample renderers already guarantee.
func RenderRelationshipsSection(rels []Relationship) string {
	sorted := make([]Relationship, len(rels))
	copy(sorted, rels)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Target != sorted[j].Target {
			return sorted[i].Target < sorted[j].Target
		}
		if sorted[i].Label != sorted[j].Label {
			return sorted[i].Label < sorted[j].Label
		}
		return sorted[i].Text < sorted[j].Text
	})

	var b strings.Builder
	for _, r := range sorted {
		fmt.Fprintf(&b, "- %s [%s](%s)\n", r.Label, r.Text, r.Target)
	}
	return b.String()
}

// AppendRelationshipsSection appends a level-1 "# <heading>" section containing
// the rendered relationship links to body and returns the new body. The heading
// is level-1 — mirroring how connectors emit "# Columns" directly — so it reads
// as a primary section and so both okf.GetSectionAny and okf-viz's link parser
// pick it up. If rels is empty, body is returned unchanged: a source with no
// relationships emits no section rather than an empty one.
func AppendRelationshipsSection(body, heading string, rels []Relationship) string {
	if len(rels) == 0 {
		return body
	}
	trimmed := strings.TrimRight(body, "\n")
	var b strings.Builder
	if trimmed != "" {
		b.WriteString(trimmed)
		b.WriteString("\n\n")
	}
	b.WriteString("# " + heading + "\n\n")
	b.WriteString(RenderRelationshipsSection(rels))
	return b.String()
}
