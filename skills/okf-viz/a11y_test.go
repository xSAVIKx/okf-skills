package main

import (
	"strings"
	"testing"
)

// TestEmit_AccessibleMarkup guards the ARIA fixes that Lighthouse checks: the
// navigator is a <nav> element (an <aside> may not take role="navigation"), and
// the concept listbox holds role="option" items, not bare list items.
func TestEmit_AccessibleMarkup(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `<nav id="nav" aria-label="Concept navigator">`) {
		t.Error("navigator must be a <nav> landmark")
	}
	if strings.Contains(html, `role="navigation"`) {
		t.Error("no element may carry an explicit role=navigation")
	}
	if !strings.Contains(html, `id="tree" role="listbox"`) {
		t.Error("concept tree must stay a listbox")
	}
	for _, want := range []string{`li.setAttribute("role", "option")`, `"aria-selected"`} {
		if !strings.Contains(html, want) {
			t.Errorf("listbox items must be options with aria-selected; missing %q", want)
		}
	}
}
