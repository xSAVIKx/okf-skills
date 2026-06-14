package main

import "strings"

// cdnTags returns the pinned CDN <script> tags with SRI integrity. The integrity
// values are filled in Task 8 (computed from the pinned files); until then they
// are empty and the offline path is used for real rendering.
func cdnTags() string {
	tags := []struct{ url, integrity string }{
		{"https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js", ""},
		{"https://cdn.jsdelivr.net/npm/layout-base@2.0.1/layout-base.min.js", ""},
		{"https://cdn.jsdelivr.net/npm/cose-base@2.2.0/cose-base.min.js", ""},
		{"https://cdn.jsdelivr.net/npm/cytoscape-fcose@2.2.0/cytoscape-fcose.min.js", ""},
		{"https://cdn.jsdelivr.net/npm/webcola@3.4.0/WebCola/cola.min.js", ""},
		{"https://cdn.jsdelivr.net/npm/cytoscape-cola@2.5.1/cytoscape-cola.min.js", ""},
		{"https://cdn.jsdelivr.net/npm/dagre@0.8.5/dist/dagre.min.js", ""},
		{"https://cdn.jsdelivr.net/npm/cytoscape-dagre@2.5.0/cytoscape-dagre.min.js", ""},
	}
	var b strings.Builder
	for _, t := range tags {
		b.WriteString(`<script src="` + t.url + `"`)
		if t.integrity != "" {
			b.WriteString(` integrity="` + t.integrity + `" crossorigin="anonymous"`)
		}
		b.WriteString("></script>\n")
	}
	return b.String()
}
