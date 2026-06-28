package main

import (
	"strings"
	"testing"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

func TestRenderDoc_TableToHTML(t *testing.T) {
	body := "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INTEGER |\n"
	html := renderMarkdown(body)
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "<td>id</td>") {
		t.Errorf("expected a rendered HTML table, got:\n%s", html)
	}
}

func TestParseColumns(t *testing.T) {
	body := "# Columns\n\n| Name | Type | Primary Key | Nullable | Default |\n| --- | --- | --- | --- | --- |\n| id | INTEGER | Yes | No |  |\n| email | TEXT | No | Yes |  |\n\n## Data Profile\n\n| Column | x |\n| --- | --- |\n| id | 1 |\n"
	cols := parseColumns(body)
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns (Data Profile rows must not leak), got %d: %+v", len(cols), cols)
	}
	if cols[0].Name != "id" || cols[0].Type != "INTEGER" || !cols[0].PK || cols[0].Nullable {
		t.Errorf("unexpected first column: %+v", cols[0])
	}
	if cols[1].Name != "email" || cols[1].PK || !cols[1].Nullable {
		t.Errorf("unexpected second column: %+v", cols[1])
	}
}

func TestParseProfile(t *testing.T) {
	body := "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INT |\n\n## Data Profile\n\n| Column | Non-Null | Null | Distinct | Min | Max | Semantic |\n| --- | --- | --- | --- | --- | --- | --- |\n| id | 10 | 2 | 8 | 1 | 99 | fk-ish |\n"
	rows := parseProfile(body)
	if len(rows) != 1 {
		t.Fatalf("expected 1 profile row, got %d", len(rows))
	}
	r := rows[0]
	if r.Column != "id" || r.NonNull != 10 || r.Null != 2 || r.Distinct != 8 || r.Min != "1" || r.Max != "99" || r.Semantic != "fk-ish" {
		t.Fatalf("unexpected profile row: %+v", r)
	}
}

func TestParseProfile_Absent(t *testing.T) {
	if rows := parseProfile("# Columns\n\n| Name |\n| --- |\n| id |\n"); rows != nil {
		t.Fatalf("expected nil profile when section absent, got %+v", rows)
	}
}

func TestEmit_StampsCoverage(t *testing.T) {
	m := &Model{RootID: rootNodeID, concepts: map[string]*okf.ConceptDoc{}}
	m.Nodes = []Node{
		{ID: "a", Kind: "concept", Description: "One row per order."},     // enriched
		{ID: "b", Kind: "concept", Description: "SQLite table customers"}, // placeholder
		{ID: rootNodeID, Kind: "root"},
	}
	m.concepts["a"] = &okf.ConceptDoc{}
	m.concepts["b"] = &okf.ConceptDoc{}
	html, _, err := Emit(m, EmitOptions{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `"coverage":"enriched"`) || !strings.Contains(html, `"coverage":"placeholder"`) {
		t.Errorf("coverage state not stamped on nodes:\n%s", html[:min(len(html), 600)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestParseColumns_NonTabular(t *testing.T) {
	if cols := parseColumns("# File: x\n\n## Metadata\n\n- Size: 1\n"); cols != nil {
		t.Fatalf("non-tabular concept must yield nil columns, got %+v", cols)
	}
}

func TestBuildDocs_MarksFKColumns(t *testing.T) {
	m := &Model{concepts: map[string]*okf.ConceptDoc{
		"orders": {Body: "# Columns\n\n| Name | Type |\n| --- | --- |\n| id | INT |\n| customer_id | INT |\n"},
	}}
	m.Edges = []Edge{{Source: "orders", Target: "customers", Kind: "crosslink", Relation: "references", Label: "customer_id"}}
	docs := buildDocs(m)
	cols := docs["orders"].Columns
	var fk bool
	for _, c := range cols {
		if c.Name == "customer_id" {
			fk = c.FK
		}
	}
	if !fk {
		t.Fatalf("customer_id should be marked FK from the references edge: %+v", cols)
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

func TestEmit_LazyAboveThreshold(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, frags, err := Emit(m, EmitOptions{Title: "Shop", Threshold: 1}) // 2 concepts > 1
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) == 0 {
		t.Fatal("lazy mode must emit body fragments")
	}
	if !strings.Contains(html, `"lazy":true`) || !strings.Contains(html, `"manifest"`) {
		t.Errorf("lazy payload must carry lazy flag + manifest")
	}
	if strings.Contains(html, "FK to") {
		t.Errorf("lazy mode must not inline bodies into index.html")
	}
	var bodyInFragment bool
	for _, c := range frags {
		if strings.Contains(c, "FK to") {
			bodyInFragment = true
		}
	}
	if !bodyInFragment {
		t.Errorf("a fragment must contain the rendered body")
	}
}

func TestEmit_InlineAllForcesSingleFile(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, frags, err := Emit(m, EmitOptions{Title: "Shop", Threshold: 1, InlineAll: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 0 {
		t.Errorf("--inline-all must not produce fragments")
	}
	if !strings.Contains(html, "FK to") {
		t.Errorf("--inline-all must inline bodies")
	}
}

func TestEmit_DefaultUsesCDNWithIntegrity(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop", Theme: "system", Offline: false})
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
	html, _, err := Emit(m, EmitOptions{Title: "Shop", Theme: "system", Offline: true})
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
	html, _, err := Emit(m, EmitOptions{Title: "Shop", Theme: "dark", Offline: true})
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
