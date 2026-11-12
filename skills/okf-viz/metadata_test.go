package main

import (
	"strings"
	"testing"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func TestEmit_PageMetadata(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{
		Title:        `Shop "catalog"`,
		Description:  "Orders & customers.",
		CanonicalURL: "https://example.org/shop/",
		HubURL:       "https://example.org/",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<meta name="description" content="Orders &amp; customers.">`,
		`<link rel="icon" href="data:image/svg+xml,`,
		`<link rel="canonical" href="https://example.org/shop/">`,
		`<meta property="og:url" content="https://example.org/shop/">`,
		`<meta property="og:title" content="Shop &#34;catalog&#34;">`,
		`<meta property="og:description" content="Orders &amp; customers.">`,
		`<meta name="twitter:card" content="summary">`,
		`<span id="hub-link"><a href="https://example.org/">More projects</a></span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("page metadata missing %q", want)
		}
	}
}

func TestEmit_PageMetadataDefaults(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	// No base URL: no canonical link and no og:url, but the rest is present.
	for _, bad := range []string{`rel="canonical"`, `og:url`, `id="hub-link"`} {
		if strings.Contains(html, bad) {
			t.Errorf("unexpected %q without a base URL or hub URL", bad)
		}
	}
	for _, want := range []string{`<meta name="description" content="`, `rel="icon"`, `property="og:title"`} {
		if !strings.Contains(html, want) {
			t.Errorf("default page metadata missing %q", want)
		}
	}
}

func TestPageDescription_Fallbacks(t *testing.T) {
	m := &Model{RootDescription: "From the bundle.", concepts: map[string]*okf.ConceptDoc{"a": {}, "b": {}}}
	if got := pageDescription(m, EmitOptions{Description: "Explicit."}); got != "Explicit." {
		t.Errorf("explicit description must win, got %q", got)
	}
	if got := pageDescription(m, EmitOptions{}); got != "From the bundle." {
		t.Errorf("root description must be the first fallback, got %q", got)
	}
	m.RootDescription = ""
	if got := pageDescription(m, EmitOptions{Title: "Shop"}); !strings.Contains(got, "2 concepts") || !strings.Contains(got, "Shop") {
		t.Errorf("generic description must name the size and title, got %q", got)
	}
}

func TestFirstParagraph(t *testing.T) {
	cases := map[string]string{
		"# Title\n\nFirst line\ncontinues.\n\nSecond.\n":      "First line continues.",
		"# Title\n\n## Assets\n\n- [a](a.md)\n":               "",
		"# Title\n- [a](a.md)\n":                              "",
		"# Catalog\n\nThis bundle maps files.\n\n## Assets\n": "This bundle maps files.",
	}
	for body, want := range cases {
		if got := firstParagraph(body); got != want {
			t.Errorf("firstParagraph(%q) = %q, want %q", body, got, want)
		}
	}
}
