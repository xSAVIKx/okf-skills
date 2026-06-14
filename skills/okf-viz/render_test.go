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
