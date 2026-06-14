package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

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
}

// buildDocs renders every concept's body to HTML keyed by concept ID.
func buildDocs(m *Model) map[string]Doc {
	docs := map[string]Doc{}
	for id, doc := range m.concepts {
		docs[id] = Doc{
			Title:       firstNonEmpty(doc.Frontmatter.Title, id),
			Type:        doc.Frontmatter.Type,
			Description: doc.Frontmatter.Description,
			Resource:    doc.Frontmatter.Resource,
			Tags:        doc.Frontmatter.Tags,
			Timestamp:   doc.Frontmatter.Timestamp,
			BodyHTML:    renderMarkdown(doc.Body),
		}
	}
	return docs
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

// EmitOptions controls page generation.
type EmitOptions struct {
	Title   string
	Theme   string // light|dark|system
	Offline bool
	Lang    string
}

type pageData struct {
	Title, CSS, AppJS, LibTag, DataJSON, InitTheme, Lang string
}

// Emit renders the full self-contained HTML for a model.
func Emit(m *Model, opt EmitOptions) (string, error) {
	css, _ := assetFS.ReadFile("assets/app.css")
	appjs, _ := assetFS.ReadFile("assets/app.js")
	tmplBytes, _ := assetFS.ReadFile("assets/page.tmpl.html")

	payload := map[string]any{"nodes": m.Nodes, "edges": m.Edges, "docs": buildDocs(m)}
	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
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
		return "", err
	}
	tmpl, err := template.New("page").Parse(string(tmplBytes))
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	err = tmpl.Execute(&buf, pageData{
		Title: opt.Title, CSS: string(css), AppJS: string(appjs),
		LibTag: libTag, DataJSON: string(dataJSON), InitTheme: theme, Lang: lang,
	})
	return buf.String(), err
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
