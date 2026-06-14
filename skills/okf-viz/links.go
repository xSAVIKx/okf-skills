package main

import (
	"path"
	"regexp"
	"strings"
)

// markdownLink matches [text](target); we only need the target.
var markdownLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// addCrossLinks parses each concept body for links to other concepts, adds a
// solid "crosslink" edge when the target resolves to an existing node, and
// computes Degree for every node. Broken links (no matching node) are skipped.
func addCrossLinks(m *Model) {
	exists := map[string]bool{}
	for _, n := range m.Nodes {
		exists[n.ID] = true
	}
	for srcID, doc := range m.concepts {
		for _, match := range markdownLink.FindAllStringSubmatch(doc.Body, -1) {
			target := resolveLink(srcID, match[1])
			if target == "" || target == srcID || !exists[target] {
				continue // external, self, or broken -> no edge
			}
			m.Edges = append(m.Edges, Edge{Source: srcID, Target: target, Kind: "crosslink"})
		}
	}
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
