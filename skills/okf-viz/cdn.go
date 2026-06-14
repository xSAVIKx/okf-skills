package main

import "strings"

// cdnTags returns the pinned CDN <script> tags with SRI integrity. The integrity
// values are filled in Task 8 (computed from the pinned files); until then they
// are empty and the offline path is used for real rendering.
func cdnTags() string {
	tags := []struct{ url, integrity string }{
		{"https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js", "sha384-IWROdLKRsN1UuJywMlWl7/blXQ8GEooN2n7dzTxfEPd7ybYIKCUJ2Ol/1Gpf3YV4"},
		{"https://cdn.jsdelivr.net/npm/layout-base@2.0.1/layout-base.min.js", "sha384-wORSveLcAX75yM0BmukpnoPBNNhzBkTW19ggbZt2Adj/OGO871ZAiQAuHUDO9OV7"},
		{"https://cdn.jsdelivr.net/npm/cose-base@2.2.0/cose-base.min.js", "sha384-UrN5MK6+mjwxHnGlBPp2bUV0WkAYIGjmrx8C35EV5z7mAyfeIPMsJg4AnmTOjL3T"},
		{"https://cdn.jsdelivr.net/npm/cytoscape-fcose@2.2.0/cytoscape-fcose.min.js", "sha384-Z4ysnuh0vXITdK1HwTvkKEhx03x06ZvweXnxnPvV0xKagye5YfD6ad/MJWybSpm0"},
		{"https://cdn.jsdelivr.net/npm/webcola@3.4.0/WebCola/cola.min.js", "sha384-o4yPeUKY7q5q4fuMcFuJWSBJPJgSHtssnfVZvjNRGOEuBwT8zxXnzyGJcy5Ojpeo"},
		{"https://cdn.jsdelivr.net/npm/cytoscape-cola@2.5.1/cytoscape-cola.min.js", "sha384-WWuGu1EcZ0HZKT1myqP0xQf4g0nAYz9bjgbrDr/QUXWC0vD6RFcAwFopc55Gkub/"},
		{"https://cdn.jsdelivr.net/npm/dagre@0.8.5/dist/dagre.min.js", "sha384-2IH3T69EIKYC4c+RXZifZRvaH5SRUdacJW7j6HtE5rQbvLhKKdawxq6vpIzJ7j9M"},
		{"https://cdn.jsdelivr.net/npm/cytoscape-dagre@2.5.0/cytoscape-dagre.min.js", "sha384-EHCdyFVbhtbpgI+4x7ETlZUvJwOkxJublmhTpH114NSk3fqfiUgcLl6pQm8JQwg9"},
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
