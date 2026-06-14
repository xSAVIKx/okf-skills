package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	okf "github.com/xSAVIKx/okf-skills/okf-go"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed assets/page.tmpl.html assets/app.css assets/app.js
var assetFS embed.FS

//go:embed all:assets/vendor
var vendorFS embed.FS

var md = goldmark.New(goldmark.WithExtensions(extension.GFM))

// renderMarkdown converts a Markdown body to HTML (GFM: tables, autolinks).
func renderMarkdown(body string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		return "<pre>" + htmlEscape(body) + "</pre>"
	}
	return buf.String()
}

// Doc is the per-concept payload embedded for the reader pane.
type Doc struct {
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Resource    string   `json:"resource,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Timestamp   string   `json:"timestamp,omitempty"`
	BodyHTML    string   `json:"bodyHtml"`
	Columns     []Column `json:"columns,omitempty"`
}

// buildDocs renders every concept's body to HTML keyed by concept ID, and parses
// the "# Columns" table into structured columns for ER mode (marking FK columns
// from the references edges that originate at each concept).
func buildDocs(m *Model) map[string]Doc {
	// fkCols[srcID] = set of column names that are FK sources (from edges' Label).
	fkCols := map[string]map[string]bool{}
	for _, e := range m.Edges {
		if e.Relation == "references" && e.Label != "" {
			if fkCols[e.Source] == nil {
				fkCols[e.Source] = map[string]bool{}
			}
			fkCols[e.Source][e.Label] = true
		}
	}

	docs := map[string]Doc{}
	for id, doc := range m.concepts {
		cols := parseColumns(doc.Body)
		for i := range cols {
			if fkCols[id][cols[i].Name] {
				cols[i].FK = true
			}
		}
		docs[id] = Doc{
			Title:       firstNonEmpty(doc.Frontmatter.Title, id),
			Type:        doc.Frontmatter.Type,
			Description: doc.Frontmatter.Description,
			Resource:    doc.Frontmatter.Resource,
			Tags:        doc.Frontmatter.Tags,
			Timestamp:   doc.Frontmatter.Timestamp,
			BodyHTML:    renderMarkdown(doc.Body),
			Columns:     cols,
		}
	}
	return docs
}

// parseColumns reads the "# Columns" GFM table (header: Name | Type | Primary Key
// | Nullable | Default) into structured columns, preserving source order. Returns
// nil when no recognizable Columns table is present (e.g. an okf-fs bundle), so ER
// mode is a no-op for non-tabular concepts.
func parseColumns(body string) []Column {
	section, ok := okf.GetSectionAny(body, "Columns")
	if !ok {
		return nil
	}
	var cols []Column
	var idx map[string]int
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		if strings.Contains(line, "---") {
			continue
		}
		cells := splitTableRow(line)
		if idx == nil {
			// Header row: map column positions case-insensitively.
			idx = map[string]int{}
			for i, h := range cells {
				idx[strings.ToLower(h)] = i
			}
			// A table without a Name column is not a recognizable schema table.
			if _, has := idx["name"]; !has {
				return nil
			}
			continue
		}
		get := func(key string) string {
			if p, ok := idx[key]; ok && p < len(cells) {
				return cells[p]
			}
			return ""
		}
		name := get("name")
		if name == "" {
			continue
		}
		cols = append(cols, Column{
			Name:     name,
			Type:     get("type"),
			PK:       strings.EqualFold(get("primary key"), "yes"),
			Nullable: strings.EqualFold(get("nullable"), "yes"),
		})
	}
	return cols
}

// splitTableRow splits a "| a | b |" row into trimmed cells.
func splitTableRow(row string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(row), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func htmlEscape(s string) string {
	r := bytes.NewBuffer(nil)
	for _, c := range s {
		switch c {
		case '<':
			r.WriteString("&lt;")
		case '>':
			r.WriteString("&gt;")
		case '&':
			r.WriteString("&amp;")
		default:
			r.WriteRune(c)
		}
	}
	return r.String()
}

// DefaultLazyThreshold is the concept count above which Emit defaults to lazy
// payloads (graph + frontmatter inlined, bodies written as sibling fragments)
// instead of inlining every rendered body. Below it, the single self-contained
// file is the default.
const DefaultLazyThreshold = 150

// EmitOptions controls page generation.
type EmitOptions struct {
	Title     string
	Theme     string // light|dark|system
	Offline   bool
	Lang      string
	InlineAll bool // force full single-file output regardless of size
	Threshold int  // concept-count lazy threshold (0 = DefaultLazyThreshold)
}

type pageData struct {
	Title, CSS, AppJS, LibTag, DataJSON, InitTheme, Lang string
}

// Emit renders the self-contained HTML for a model and, in lazy mode, the set of
// per-concept body fragments to write next to it (relative path -> HTML content).
// In inline mode (small bundles or --inline-all) fragments is nil and the output
// is byte-identical to the historical single-file behavior.
func Emit(m *Model, opt EmitOptions) (string, map[string]string, error) {
	css, _ := assetFS.ReadFile("assets/app.css")
	appjs, _ := assetFS.ReadFile("assets/app.js")
	tmplBytes, _ := assetFS.ReadFile("assets/page.tmpl.html")

	threshold := opt.Threshold
	if threshold <= 0 {
		threshold = DefaultLazyThreshold
	}
	lazy := !opt.InlineAll && len(m.concepts) > threshold

	var payload map[string]any
	var fragments map[string]string
	docs := buildDocs(m)
	if lazy {
		// Inline graph + lightweight docs (frontmatter + columns, no body); write
		// bodies as deterministic sibling fragments referenced by a manifest.
		manifest := map[string]string{}
		fragments = map[string]string{}
		light := map[string]Doc{}
		for id, d := range docs {
			frag := "_okf/" + id + ".html"
			fragments[frag] = d.BodyHTML
			manifest[id] = frag
			d.BodyHTML = "" // dropped from the inline payload
			light[id] = d
		}
		payload = map[string]any{"nodes": m.Nodes, "edges": m.Edges, "docs": light, "manifest": manifest, "lazy": true}
	} else {
		payload = map[string]any{"nodes": m.Nodes, "edges": m.Edges, "docs": docs}
	}
	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	theme := opt.Theme
	if theme != "light" && theme != "dark" {
		theme = "system"
	}
	lang := opt.Lang
	if lang == "" {
		lang = "en"
	}
	libTag, err := libraryTag(opt.Offline)
	if err != nil {
		return "", nil, err
	}
	tmpl, err := template.New("page").Parse(string(tmplBytes))
	if err != nil {
		return "", nil, err
	}
	var buf strings.Builder
	err = tmpl.Execute(&buf, pageData{
		Title: opt.Title, CSS: string(css), AppJS: string(appjs),
		LibTag: libTag, DataJSON: string(dataJSON), InitTheme: theme, Lang: lang,
	})
	return buf.String(), fragments, err
}

// vendorOrder lists the pinned vendor JS files in dependency-first load order:
// cytoscape core → layout-base → cose-base → fcose → webcola → cytoscape-cola
// → dagre → cytoscape-dagre.
var vendorOrder = []string{
	"cytoscape.min.js",
	"layout-base.min.js",
	"cose-base.min.js",
	"cytoscape-fcose.min.js",
	"cola.min.js",
	"cytoscape-cola.min.js",
	"dagre.min.js",
	"cytoscape-dagre.min.js",
}

// libraryTag returns CDN <script> tags (default) or inlined library <script>
// blocks (offline), emitting vendored files in dependency-first order.
func libraryTag(offline bool) (string, error) {
	if !offline {
		return cdnTags(), nil
	}
	var b strings.Builder
	b.WriteString("<!-- OKF_INLINE_LIB -->\n")
	for _, name := range vendorOrder {
		js, err := vendorFS.ReadFile("assets/vendor/" + name)
		if err != nil {
			return "", fmt.Errorf("vendored library %s missing; run the offline vendoring step", name)
		}
		b.WriteString("<script>")
		b.Write(js)
		b.WriteString("</script>\n")
	}
	return b.String(), nil
}
