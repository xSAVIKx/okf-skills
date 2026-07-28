package main

import (
	"path/filepath"
	"sort"
	"strings"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
)

// ComputeDiff annotates the new model (newer) against an older one: each concept
// node gets Diff "added" (only in newer), "changed" (structural content_hash
// differs, or — when a hash is missing — the body differs), or "" (unchanged);
// concepts only in older are injected as "removed" ghost nodes. Edges are diffed
// by their identity tuple. The line is exactly as stable as incremental produce
// makes it: content_hash drives "changed", with a body-comparison fallback.
func ComputeDiff(newer, older *Model) {
	oldDocs := older.concepts
	newIDs := map[string]bool{}
	for i := range newer.Nodes {
		n := &newer.Nodes[i]
		if n.Kind != "concept" {
			continue
		}
		newIDs[n.ID] = true
		od, ok := oldDocs[n.ID]
		if !ok {
			n.Diff = "added"
			continue
		}
		if conceptChanged(newer.concepts[n.ID], od) {
			n.Diff = "changed"
		}
	}

	// Removed concepts: present in older, absent from newer → ghost nodes.
	var removedIDs []string
	for id := range oldDocs {
		if !newIDs[id] {
			removedIDs = append(removedIDs, id)
		}
	}
	sort.Strings(removedIDs)
	for _, id := range removedIDs {
		od := oldDocs[id]
		title := od.Frontmatter.Title
		if title == "" {
			title = filepath.Base(id)
		}
		newer.Nodes = append(newer.Nodes, Node{
			ID: id, Kind: "concept", Type: od.Frontmatter.Type, Title: title,
			Description: od.Frontmatter.Description, Dir: pathDir(id), Diff: "removed",
		})
	}

	// Edge diff by identity tuple (Kind, Relation, Source, Target).
	key := func(e Edge) string { return e.Kind + "\x00" + e.Relation + "\x00" + e.Source + "\x00" + e.Target }
	oldEdges := map[string]bool{}
	for _, e := range older.Edges {
		oldEdges[key(e)] = true
	}
	newEdges := map[string]bool{}
	for i := range newer.Edges {
		newEdges[key(newer.Edges[i])] = true
		if !oldEdges[key(newer.Edges[i])] {
			newer.Edges[i].Diff = "added"
		}
	}
	for _, e := range older.Edges {
		if !newEdges[key(e)] {
			e.Diff = "removed"
			newer.Edges = append(newer.Edges, e)
		}
	}
	sortEdges(newer.Edges)
}

// conceptChanged reports whether a concept's structure changed between two docs,
// preferring the content_hash and falling back to a body comparison when a hash is
// absent (a bundle produced before incremental produce).
func conceptChanged(newer, older *okf.ConceptDoc) bool {
	nh, oh := newer.Frontmatter.ContentHash, older.Frontmatter.ContentHash
	if nh != "" && oh != "" {
		return nh != oh
	}
	return strings.TrimSpace(newer.Body) != strings.TrimSpace(older.Body)
}

// sortEdges re-applies the canonical (Kind, Relation, Source, Target, Diff)
// ordering so diff/federation output stays byte-stable.
func sortEdges(edges []Edge) {
	sort.Slice(edges, func(i, j int) bool {
		ei, ej := edges[i], edges[j]
		if ei.Kind != ej.Kind {
			return ei.Kind < ej.Kind
		}
		if ei.Relation != ej.Relation {
			return ei.Relation < ej.Relation
		}
		if ei.Source != ej.Source {
			return ei.Source < ej.Source
		}
		if ei.Target != ej.Target {
			return ei.Target < ej.Target
		}
		return ei.Diff < ej.Diff
	})
}

// Federate merges additional bundles into the primary model, namespacing every
// node ID by a per-bundle key so colliding IDs across bundles stay distinct, and
// stamping each node's owning Bundle. The single-bundle default is untouched
// (Federate runs only when extra bundles are supplied).
func Federate(primaryKey string, primary *Model, others map[string]*Model) {
	namespace(primaryKey, primary)
	keys := make([]string, 0, len(others))
	for k := range others {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		om := others[k]
		namespace(k, om)
		primary.Nodes = append(primary.Nodes, om.Nodes...)
		primary.Edges = append(primary.Edges, om.Edges...)
		for id, d := range om.concepts {
			primary.concepts[id] = d
		}
	}
	sort.Slice(primary.Nodes, func(i, j int) bool { return primary.Nodes[i].ID < primary.Nodes[j].ID })
	sortEdges(primary.Edges)
}

// namespace prefixes a model's node/edge IDs with "<key>:" and stamps Bundle=key
// on its nodes, so federated bundles never collide.
func namespace(key string, m *Model) {
	pfx := key + ":"
	ns := func(id string) string {
		if id == "" {
			return id
		}
		return pfx + id
	}
	for i := range m.Nodes {
		m.Nodes[i].ID = ns(m.Nodes[i].ID)
		if m.Nodes[i].Dir != "" {
			m.Nodes[i].Dir = ns(m.Nodes[i].Dir)
		}
		m.Nodes[i].Bundle = key
	}
	for i := range m.Edges {
		m.Edges[i].Source = ns(m.Edges[i].Source)
		m.Edges[i].Target = ns(m.Edges[i].Target)
	}
	relabeled := map[string]*okf.ConceptDoc{}
	for id, d := range m.concepts {
		relabeled[ns(id)] = d
	}
	m.concepts = relabeled
}
