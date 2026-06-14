package main

import (
	"strings"
	"testing"
)

func TestRenderDoc_TableToHTML(t *testing.T) {
	body := "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INTEGER |\n"
	html := renderMarkdown(body)
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "<td>id</td>") {
		t.Errorf("expected a rendered HTML table, got:\n%s", html)
	}
}

func TestBuildDocs(t *testing.T) {
	m, err := BuildModel("testdata/linked")
	if err != nil {
		t.Fatalf("BuildModel: %v", err)
	}
	docs := buildDocs(m)
	d, ok := docs["tables/orders"]
	if !ok {
		t.Fatal("missing doc for tables/orders")
	}
	if d.Title != "orders" || !strings.Contains(d.BodyHTML, "Columns") {
		t.Errorf("unexpected doc: %+v", d)
	}
}

func TestEmit_DefaultUsesCDNWithIntegrity(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, err := Emit(m, EmitOptions{Title: "Shop", Theme: "system", Offline: false})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="okf-data"`, `id="panes"`, "cytoscape", "cdn.jsdelivr.net/npm/cytoscape", "integrity="} {
		if !strings.Contains(html, want) {
			t.Errorf("default output missing %q", want)
		}
	}
	if strings.Contains(html, "OKF_INLINE_LIB") {
		t.Error("default output must not inline the library")
	}
}

func TestEmit_OfflineInlinesLib(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, err := Emit(m, EmitOptions{Title: "Shop", Theme: "system", Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "https://cdn") || strings.Contains(html, "integrity=") {
		t.Error("offline output must have no CDN/integrity references")
	}
}

func TestRenderOfflineHasNoNetwork(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, err := Emit(m, EmitOptions{Title: "Shop", Theme: "dark", Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	// No fetched network resources: no CDN <script src> tags and no integrity= attributes.
	// (Vendored JS files may contain http:// in comments/string literals, which is harmless.)
	if strings.Contains(html, `src="https://`) || strings.Contains(html, "https://cdn") {
		t.Error("offline output must not reference any fetched CDN resource")
	}
	if strings.Contains(html, "integrity=") {
		t.Error("offline output must not contain integrity= (CDN SRI attr)")
	}
	if !strings.Contains(html, "cytoscape") {
		t.Error("offline output must inline cytoscape")
	}
}
