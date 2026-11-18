package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder for image.DecodeConfig
	_ "image/png"  // registers the PNG decoder for image.DecodeConfig
	"os"
	"path/filepath"
	"strings"
)

// PageImage is the social preview image of a page (og:image and
// twitter:image). An empty URL means that the page has no image.
type PageImage struct {
	URL           string // absolute URL of the image
	Type          string // media type, for example "image/png"
	Width, Height int    // pixel size; 0 omits the size tags
	Alt           string // alternative text (default: the page title)
}

// readPageImage reads the size and the media type of a local PNG or JPEG file.
// The caller sets the URL and the alternative text.
func readPageImage(path string) (PageImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return PageImage{}, err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return PageImage{}, fmt.Errorf("%s: %w (use a PNG or JPEG file)", path, err)
	}
	return PageImage{Type: "image/" + format, Width: cfg.Width, Height: cfg.Height}, nil
}

// copyFile copies src to dst. It does nothing when both paths are the same file.
func copyFile(src, dst string) error {
	if a, err := filepath.Abs(src); err == nil {
		if b, err := filepath.Abs(dst); err == nil && a == b {
			return nil
		}
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// readJSONLDNodes reads extra schema.org nodes from a JSON file. The file holds
// one node object or an array of node objects.
func readJSONLDNodes(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var nodes []map[string]any
	if strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		err = json.Unmarshal(data, &nodes)
	} else {
		var node map[string]any
		err = json.Unmarshal(data, &node)
		nodes = []map[string]any{node}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return nodes, nil
}

// pageJSONLD builds the JSON-LD block of the page: a CollectionPage node for the
// catalog, followed by the extra nodes from EmitOptions.JSONLD. An extra node
// whose "@id" is the canonical URL is merged into the page node, so a site can
// add properties such as isPartOf or about to the page. json.Marshal escapes
// the characters <, > and & as \u003c, \u003e and \u0026, so the text
// cannot close the <script> element.
func pageJSONLD(opt EmitOptions, description string) (string, error) {
	page := map[string]any{
		"@type":       "CollectionPage",
		"name":        opt.Title,
		"description": description,
		"inLanguage":  opt.Lang,
	}
	if opt.CanonicalURL != "" {
		page["@id"] = opt.CanonicalURL
		page["url"] = opt.CanonicalURL
	}
	if opt.Image.URL != "" {
		page["image"] = opt.Image.URL
	}
	if opt.Author != "" {
		author := map[string]any{"@type": "Person", "name": opt.Author}
		if opt.AuthorURL != "" {
			author["url"] = opt.AuthorURL
		}
		page["author"] = author
	}
	graph := []map[string]any{page}
	for _, n := range opt.JSONLD {
		if id, _ := n["@id"].(string); id != "" && id == opt.CanonicalURL {
			for k, v := range n {
				page[k] = v
			}
			continue
		}
		graph = append(graph, n)
	}
	out, err := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	return string(out), err
}
