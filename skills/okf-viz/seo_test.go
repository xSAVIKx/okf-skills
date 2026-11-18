package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var jsonLDRe = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)

// parseJSONLD returns the @graph nodes of the one JSON-LD block of a page.
func parseJSONLD(t *testing.T, html string) []map[string]any {
	t.Helper()
	blocks := jsonLDRe.FindAllStringSubmatch(html, -1)
	if len(blocks) != 1 {
		t.Fatalf("want 1 JSON-LD block, got %d", len(blocks))
	}
	var doc struct {
		Context string           `json:"@context"`
		Graph   []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(blocks[0][1]), &doc); err != nil {
		t.Fatalf("JSON-LD does not parse: %v\n%s", err, blocks[0][1])
	}
	if doc.Context != "https://schema.org" {
		t.Errorf("@context = %q", doc.Context)
	}
	return doc.Graph
}

func TestEmit_SocialImage(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{
		Title:        "Shop",
		CanonicalURL: "https://example.org/shop/",
		Image: PageImage{URL: "https://example.org/shop/og.png", Type: "image/png",
			Width: 1200, Height: 630, Alt: `Shop "graph" & more`},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<meta name="twitter:card" content="summary_large_image">`,
		`<meta property="og:image" content="https://example.org/shop/og.png">`,
		`<meta property="og:image:type" content="image/png">`,
		`<meta property="og:image:width" content="1200">`,
		`<meta property="og:image:height" content="630">`,
		`<meta property="og:image:alt" content="Shop &#34;graph&#34; &amp; more">`,
		`<meta name="twitter:image" content="https://example.org/shop/og.png">`,
		`<meta name="twitter:image:alt" content="Shop &#34;graph&#34; &amp; more">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("social image tags missing %q", want)
		}
	}
}

func TestEmit_SocialImageDefaults(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`og:image`, `twitter:image`, `summary_large_image`} {
		if strings.Contains(html, bad) {
			t.Errorf("unexpected %q without an image", bad)
		}
	}
	// The alternative text falls back to the page title.
	html, _, _ = Emit(m, EmitOptions{Title: "Shop", Image: PageImage{URL: "https://example.org/og.png"}})
	if !strings.Contains(html, `<meta property="og:image:alt" content="Shop">`) {
		t.Error("image alt does not default to the title")
	}
	if strings.Contains(html, `og:image:width`) || strings.Contains(html, `og:image:type`) {
		t.Error("size and type tags must be omitted when they are unknown")
	}
}

func TestEmit_JSONLD(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{
		Title:        "Shop </script><b>x</b>",
		Description:  "Orders & customers.",
		CanonicalURL: "https://example.org/shop/",
		Author:       "Ada",
		AuthorURL:    "https://ada.example/",
		Image:        PageImage{URL: "https://example.org/shop/og.png"},
		JSONLD: []map[string]any{
			{"@id": "https://example.org/shop/", "isPartOf": map[string]any{"@id": "https://example.org/#website"}},
			{"@type": "SoftwareSourceCode", "@id": "https://example.org/shop/#software",
				"codeRepository": "https://github.com/example/shop", "programmingLanguage": "Go"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The regexp stops at the first </script>, so an unescaped title would
	// cut the block short and make it fail to parse below.
	block := jsonLDRe.FindStringSubmatch(html)
	if block == nil || strings.Contains(block[1], "<") || !strings.Contains(block[1], `\u003c/script\u003e`) {
		t.Errorf("< and > in JSON-LD are not escaped as \\u003c and \\u003e: %v", block)
	}
	graph := parseJSONLD(t, html)
	if len(graph) != 2 {
		t.Fatalf("want 2 nodes (page merged with its patch, software), got %d: %v", len(graph), graph)
	}
	page := graph[0]
	for k, want := range map[string]string{
		"@type": "CollectionPage", "@id": "https://example.org/shop/", "url": "https://example.org/shop/",
		"name": "Shop </script><b>x</b>", "description": "Orders & customers.", "inLanguage": "en",
		"image": "https://example.org/shop/og.png",
	} {
		if page[k] != want {
			t.Errorf("page %s = %v, want %q", k, page[k], want)
		}
	}
	if part, _ := page["isPartOf"].(map[string]any); part["@id"] != "https://example.org/#website" {
		t.Errorf("extra node with the page @id is not merged into the page: %v", page)
	}
	if a, _ := page["author"].(map[string]any); a["@type"] != "Person" || a["name"] != "Ada" || a["url"] != "https://ada.example/" {
		t.Errorf("page author = %v", page["author"])
	}
	if graph[1]["@type"] != "SoftwareSourceCode" || graph[1]["programmingLanguage"] != "Go" {
		t.Errorf("extra node = %v", graph[1])
	}
}

func TestEmit_JSONLDDefaults(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	graph := parseJSONLD(t, html)
	if len(graph) != 1 || graph[0]["@type"] != "CollectionPage" {
		t.Fatalf("want one CollectionPage node, got %v", graph)
	}
	for _, k := range []string{"@id", "url", "image", "author"} {
		if _, ok := graph[0][k]; ok {
			t.Errorf("unexpected %q without its option", k)
		}
	}
}

func TestReadPageImage(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "og.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 120, 63))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	img, err := readPageImage(p)
	if err != nil {
		t.Fatal(err)
	}
	if img.Type != "image/png" || img.Width != 120 || img.Height != 63 {
		t.Errorf("readPageImage = %+v", img)
	}
	txt := filepath.Join(dir, "x.txt")
	_ = os.WriteFile(txt, []byte("not an image"), 0644)
	if _, err := readPageImage(txt); err == nil {
		t.Error("want an error for a file that is not an image")
	}
}

func TestReadJSONLDNodes(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.json")
	many := filepath.Join(dir, "many.json")
	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(one, []byte(`{"@type":"Person","name":"Ada"}`), 0644)
	_ = os.WriteFile(many, []byte(" [{\"@type\":\"Person\"},{\"@type\":\"WebSite\"}]"), 0644)
	_ = os.WriteFile(bad, []byte(`{"@type":`), 0644)
	if n, err := readJSONLDNodes(one); err != nil || len(n) != 1 || n[0]["name"] != "Ada" {
		t.Errorf("one node: %v, %v", n, err)
	}
	if n, err := readJSONLDNodes(many); err != nil || len(n) != 2 {
		t.Errorf("array: %v, %v", n, err)
	}
	if _, err := readJSONLDNodes(bad); err == nil {
		t.Error("want an error for bad JSON")
	}
}
