package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/savikne/okf-skills/okf-go"
)

// Node is a graph node: a concept, a directory, or the bundle root.
type Node struct {
	ID          string   `json:"id"`          // concept ID (path without .md), dir path, or "" for root
	Kind        string   `json:"kind"`        // "concept" | "directory" | "root"
	Type        string   `json:"type"`        // frontmatter type (concepts only)
	Title       string   `json:"title"`       // display label
	Description string   `json:"description"` // one-line summary (concepts only)
	Resource    string   `json:"resource,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Dir         string   `json:"dir"`    // parent directory id ("" = root)
	Degree      int      `json:"degree"` // set in links.go
}

// Edge connects two nodes. Kind is "containment" (dashed) or "crosslink" (solid).
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// Model is the full graph plus the bundle root label.
type Model struct {
	RootID    string // node id of the root node
	RootTitle string // page title source
	Nodes     []Node
	Edges     []Edge
	// concepts maps concept ID -> parsed doc, used by render.go.
	concepts map[string]*okf.ConceptDoc
}

const rootNodeID = "__root__"

// BuildModel walks an OKF bundle and builds the graph model (nodes + containment edges).
// Cross-link edges and degree are added by addCrossLinks (links.go).
func BuildModel(bundleDir string) (*Model, error) {
	m := &Model{RootID: rootNodeID, concepts: map[string]*okf.ConceptDoc{}}
	dirsWithConcepts := map[string]bool{}
	rootTitle := filepath.Base(bundleDir)

	err := filepath.WalkDir(bundleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(bundleDir, path)
		rel = filepath.ToSlash(rel)
		base := filepath.Base(rel)
		// Reserved files are not concept nodes. The root index sets the title.
		if base == "index.md" || base == "log.md" {
			if filepath.Dir(rel) == "." && base == "index.md" {
				if doc, derr := okf.ReadConceptDoc(path); derr == nil {
					if t := firstHeading(doc.Body); t != "" {
						rootTitle = t
					}
				}
			}
			return nil
		}
		doc, derr := okf.ReadConceptDoc(path)
		if derr != nil {
			// Permissive: skip unparseable concept, warn on stderr.
			os.Stderr.WriteString("okf-viz: skipping " + rel + ": " + derr.Error() + "\n")
			return nil
		}
		id := strings.TrimSuffix(rel, ".md")
		dir := pathDir(id)
		title := doc.Frontmatter.Title
		if title == "" {
			title = filepath.Base(id)
		}
		m.Nodes = append(m.Nodes, Node{
			ID: id, Kind: "concept", Type: doc.Frontmatter.Type,
			Title: title, Description: doc.Frontmatter.Description,
			Resource: doc.Frontmatter.Resource, Tags: doc.Frontmatter.Tags, Dir: dir,
		})
		m.concepts[id] = doc
		markDirChain(dirsWithConcepts, dir)
		return nil
	})
	if err != nil {
		return nil, err
	}
	m.RootTitle = rootTitle

	// Directory nodes (sorted for determinism).
	var dirs []string
	for dir := range dirsWithConcepts {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		m.Nodes = append(m.Nodes, Node{
			ID: dir, Kind: "directory", Title: filepath.Base(dir), Dir: pathDir(dir),
		})
	}
	// Root node.
	m.Nodes = append(m.Nodes, Node{ID: rootNodeID, Kind: "root", Title: rootTitle, Dir: ""})

	// Containment edges: every concept/dir -> its parent (dir or root).
	for _, n := range m.Nodes {
		if n.Kind == "root" {
			continue
		}
		parent := n.Dir
		if parent == "" {
			parent = rootNodeID
		}
		m.Edges = append(m.Edges, Edge{Source: parent, Target: n.ID, Kind: "containment"})
	}
	return m, nil
}

// pathDir returns the slash-style parent directory of an id, or "" at top level.
func pathDir(id string) string {
	d := filepath.ToSlash(filepath.Dir(id))
	if d == "." || d == "/" {
		return ""
	}
	return d
}

// markDirChain records dir and all ancestor dirs as containing concepts.
func markDirChain(set map[string]bool, dir string) {
	for dir != "" {
		set[dir] = true
		dir = pathDir(dir)
	}
}

// firstHeading returns the text of the first ATX heading in a markdown body.
func firstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return ""
}
