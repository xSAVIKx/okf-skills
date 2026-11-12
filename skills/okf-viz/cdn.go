package main

import "strings"

// cdnTags returns the pinned CDN <script> tags with SRI integrity hashes
// computed from the vendored copies of each library.
func cdnTags() string {
	// Each URL names a file that the npm package publishes. Do not use a
	// .min.js URL for a package that does not publish one: jsDelivr then
	// minifies the file on request, and the output bytes can change. That
	// breaks the SRI hash. cytoscape-fcose and cytoscape-cola publish no
	// .min.js file. TestCDNTagsLive checks every hash against the CDN.
	tags := []struct{ url, integrity string }{
		{"https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js", "sha384-IWROdLKRsN1UuJywMlWl7/blXQ8GEooN2n7dzTxfEPd7ybYIKCUJ2Ol/1Gpf3YV4"},
		{"https://cdn.jsdelivr.net/npm/layout-base@2.0.1/layout-base.min.js", "sha384-wORSveLcAX75yM0BmukpnoPBNNhzBkTW19ggbZt2Adj/OGO871ZAiQAuHUDO9OV7"},
		{"https://cdn.jsdelivr.net/npm/cose-base@2.2.0/cose-base.min.js", "sha384-UrN5MK6+mjwxHnGlBPp2bUV0WkAYIGjmrx8C35EV5z7mAyfeIPMsJg4AnmTOjL3T"},
		{"https://cdn.jsdelivr.net/npm/cytoscape-fcose@2.2.0/cytoscape-fcose.js", "sha384-uk5Wbjq1+KqUdHO30w7N7GrEGdzBhaJeW9o/ANF6v9+yx3M/cBmoX51C000JNCUf"},
		{"https://cdn.jsdelivr.net/npm/webcola@3.4.0/WebCola/cola.min.js", "sha384-o4yPeUKY7q5q4fuMcFuJWSBJPJgSHtssnfVZvjNRGOEuBwT8zxXnzyGJcy5Ojpeo"},
		{"https://cdn.jsdelivr.net/npm/cytoscape-cola@2.5.1/cytoscape-cola.js", "sha384-hS0F3AnUYl+EBC3wCT4+d6MKdLR27UlifwZZYJi5oRCNhKA1ptaEWExH6BqyfMeW"},
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
