---
name: okf-viz
description: Visualizer that renders an Open Knowledge Format (OKF) bundle into a single self-contained interactive index.html — a three-pane explorer (tree/filters, a Cytoscape graph with switchable layouts, and a rendered reader) written next to index.md. Use when you want a human-browsable, graph view of an OKF bundle, or to generate a shareable HTML catalog from any connector's bundle.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the binary. The default output references the Cytoscape library from a CDN; use --offline for a fully isolated file.
metadata:
  version: "0.1.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, visualization, graph, cytoscape, html, consumer"
---

# okf-viz — OKF Bundle Visualizer

Renders any OKF bundle into one self-contained interactive `index.html` beside
`index.md`: a three-pane explorer with a Cytoscape graph (switchable layouts),
a navigator (tree + type/tag filters + search), and a rendered concept reader.

## Setup

```bash
# Install the published binary (Go 1.24+) — no clone needed:
go install github.com/xSAVIKx/okf-skills/skills/okf-viz@v0.1.0

# …or build from a clone of the repository:
cd skills/okf-viz && go build -o okf-viz .
```

## Commands

### render

```bash
./okf-viz render --bundle <bundle-dir> [--out <file>] [--offline] [--theme system|light|dark] [--lang en] [--title <text>]
```

- `--bundle` (required): the OKF bundle to visualize.
- `--out`: output path; default `<bundle>/index.html`.
- `--offline`: inline the graph library for a fully isolated file (no CDN).
- `--theme`: initial theme (default `system`, follows the OS); switchable in-page.
- `--lang`: UI-chrome language (default `en`).
- `--title`: page title (default derived from the bundle).
- `--inline-all`: always inline every concept body into the single file, regardless of size (the archival/shareable form).
- `--threshold`: concept count above which bodies are written lazily as sibling `_okf/<id>.html` fragments instead of inlined (default 150). Small bundles stay a single self-contained file; large bundles load a concept's body on selection. Re-running produces byte-identical `index.html` and fragments. In-page, **ER** toggles the entity-relationship view and **Cluster** collapses directories into tappable super-nodes for large graphs.

### schema

```bash
./okf-viz schema
```

Prints the machine-readable JSON used by `okf-mcp` to expose `okf-viz__render`.
