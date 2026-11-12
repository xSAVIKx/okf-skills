package main

import (
	"strings"
	"testing"
)

// TestEmit_OfflineDefersOptionalLayouts checks the load-time split of the
// inlined libraries: the default layout stack runs at load, and the cola and
// dagre libraries are inert text/plain blocks that app.js runs on first use.
// The output must stay self-contained (every library is still inline).
func TestEmit_OfflineDefersOptionalLayouts(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop", Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cytoscape.min.js", "layout-base.min.js", "cose-base.min.js", "cytoscape-fcose.min.js"} {
		if !strings.Contains(html, `<script data-okf-lib="`+name+`">`) {
			t.Errorf("eager library %s must be an executable inline script", name)
		}
	}
	for _, name := range []string{"cola.min.js", "cytoscape-cola.min.js", "dagre.min.js", "cytoscape-dagre.min.js"} {
		if !strings.Contains(html, `<script type="text/plain" data-okf-lib="`+name+`">`) {
			t.Errorf("optional library %s must be an inert inline block", name)
		}
	}
	if strings.Contains(html, "data-src=") {
		t.Error("offline output must not reference a library URL")
	}
}

func TestEmit_CDNDefersOptionalLayouts(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `crossorigin="anonymous" data-okf-lib="cytoscape.min.js"></script>`) {
		t.Error("cytoscape must load eagerly from the CDN with SRI")
	}
	for _, name := range []string{"dagre.min.js", "cola.min.js"} {
		if !strings.Contains(html, `<script type="text/plain" data-okf-lib="`+name+`" data-src="https://cdn.jsdelivr.net/`) {
			t.Errorf("optional library %s must be an inert block with its CDN URL", name)
		}
	}
	if strings.Contains(html, `<script src="https://cdn.jsdelivr.net/npm/dagre@`) {
		t.Error("dagre must not load at page load")
	}
	if strings.Count(html, "data-integrity=") != 4 {
		t.Errorf("each deferred CDN library must keep its SRI hash, got %d", strings.Count(html, "data-integrity="))
	}
}

// TestEmit_DeferredGraphStart guards the hooks that keep the force layout off
// the critical path: a worker-based first layout, a main-thread fallback, and
// lazy loading of the optional layout libraries.
func TestEmit_DeferredGraphStart(t *testing.T) {
	m, _ := BuildModel("testdata/linked")
	addCrossLinks(m)
	html, _, err := Emit(m, EmitOptions{Title: "Shop", Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"function startGraph", "function layoutInWorker", "new Worker(", "function layoutWorkerMain", "function ensureLayout", `{ animate: false }`} {
		if !strings.Contains(html, want) {
			t.Errorf("app.js missing deferred-start hook %q", want)
		}
	}
}
