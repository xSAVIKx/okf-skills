package main

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

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
