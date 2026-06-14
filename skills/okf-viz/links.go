package main

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// markdownLink matches [text](target), capturing both the link text and target.
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// headingRelations maps a conventional section heading (case-folded) to the edge
// relation a link under it carries. Links outside any known section keep an empty
// relation and render as a generic crosslink — permissive by design.
var headingRelations = map[string]string{
	"relationships": "references",
	"related files": "co-changes",
	"joins":         "joins-with",
	"see also":      "see-also",
}

// annotationRelations maps an explicit link-text annotation (e.g. [fk](...)) to a
// relation, overriding the section-derived one so connectors can be explicit.
var annotationRelations = map[string]string{
	"fk":         "references",
	"references": "references",
	"joins-with": "joins-with",
	"see-also":   "see-also",
}

// addCrossLinks parses each concept body for links to other concepts, adds a
// solid "crosslink" edge when the target resolves to an existing node, types it
// with a Relation derived from the enclosing section (or a link-text annotation),
// and computes Degree for every node. Broken links are skipped.
func addCrossLinks(m *Model) {
	exists := map[string]bool{}
	for _, n := range m.Nodes {
		exists[n.ID] = true
	}
	seen := map[string]bool{}
	// Iterate concepts in sorted ID order so multi-relation dedup is deterministic.
	srcIDs := make([]string, 0, len(m.concepts))
	for id := range m.concepts {
		srcIDs = append(srcIDs, id)
	}
	sort.Strings(srcIDs)

	for _, srcID := range srcIDs {
		doc := m.concepts[srcID]
		relation := ""
		for _, line := range strings.Split(doc.Body, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				relation = headingRelations[strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))]
				continue
			}
			for _, match := range markdownLink.FindAllStringSubmatch(line, -1) {
				linkText, href := match[1], match[2]
				target := resolveLink(srcID, href)
				if target == "" || target == srcID || !exists[target] {
					continue // external, self, or broken -> no edge
				}
				rel := relation
				if a, ok := annotationRelations[strings.ToLower(strings.TrimSpace(linkText))]; ok {
					rel = a // explicit annotation overrides the section relation
				}
				key := srcID + "\x00" + target + "\x00" + rel
				if seen[key] {
					continue // dedup per (source, target, relation)
				}
				seen[key] = true
				m.Edges = append(m.Edges, Edge{Source: srcID, Target: target, Kind: "crosslink", Relation: rel})
			}
		}
	}
	// Sort edges deterministically by (Kind, Relation, Source, Target) so output
	// stays byte-identical across runs.
	sort.Slice(m.Edges, func(i, j int) bool {
		ei, ej := m.Edges[i], m.Edges[j]
		if ei.Kind != ej.Kind {
			return ei.Kind < ej.Kind
		}
		if ei.Relation != ej.Relation {
			return ei.Relation < ej.Relation
		}
		if ei.Source != ej.Source {
			return ei.Source < ej.Source
		}
		return ei.Target < ej.Target
	})

	// Degree = count of incident edges (both kinds).
	deg := map[string]int{}
	for _, e := range m.Edges {
		deg[e.Source]++
		deg[e.Target]++
	}
	for i := range m.Nodes {
		m.Nodes[i].Degree = deg[m.Nodes[i].ID]
	}
}

// resolveLink turns a markdown link target into a concept ID (path without .md),
// or "" if it's external or not a bundle link. Absolute targets begin with "/"
// (bundle root); relative targets resolve against the source's directory.
func resolveLink(srcID, target string) string {
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
		p = path.Join(pathDir(srcID), target)
	}
	p = path.Clean(p)
	return strings.TrimSuffix(p, ".md")
}
