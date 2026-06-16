---
name: okf-reader
description: Guidance for AI agents on how to parse, traverse, and query Open Knowledge Format (OKF) bundles efficiently — minimizing token usage and avoiding slow recursive directory walks. Use when reading, navigating, analyzing, or answering questions about an existing OKF bundle. Instructions-only; no binary required.
license: Apache-2.0
metadata:
  version: "0.2.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, agent-guidance, documentation, prompt-engineering"
---

# OKF Bundle Reader Guidance Skill

Procedural rules for reading and traversing **any** OKF bundle efficiently, no matter how its producer organized it. Following them reduces token consumption, speeds execution, and avoids recursive directory walks.

OKF is deliberately flexible: the directory layout, the concept `type` values, and whether concepts cross-link are all **producer-defined**. So these rules key off the spec's universal structures — frontmatter `type`, `index.md`, markdown links, and `resource` — and never off any one source's conventions (there is no required `tables/` folder, no fixed type vocabulary, no guaranteed cross-links).

## When to Use

Load this skill whenever you are tasked with reading, parsing, querying, or analyzing an existing OKF bundle.

## Instructions for Agents

### 1. Index-first discovery (descend nested indexes)
- **Rule**: NEVER recursively read or load every markdown file at startup.
- **Protocol**:
  1. Read the bundle-root `index.md` first if present — it is the directory listing.
  2. Follow its links to map the available concepts. The index may be **flat** (every concept — including container/directory concepts — listed at the root as peers) or **hierarchical** (subdirectories carry their own `index.md` or sub-index concept). Descend any nested indexes for progressive disclosure instead of walking the tree.
  3. If there is no `index.md`, discover concepts cheaply — a shallow listing or a `*.md` glob — without reading bodies yet.
  4. Sanity-check it is an OKF bundle: concept files have YAML frontmatter with a `type`; the root `index.md` may declare `okf_version`.

### 2. Route directly to a concept (layout is producer-defined)
- **Rule**: Do **not** assume a fixed directory layout. There is no required `tables/` (or any other) folder — producers organize concepts however suits the domain (SQL connectors group under `tables/`; filesystem/git bundles mirror the source tree; others differ).
- **Protocol**:
  - Resolve a concept's location from the `index.md` links — use each link target **verbatim**; do not guess, strip, or rewrite paths (filenames and separators are producer-defined).
  - When the layout mirrors a source path, the concept file is usually that path; otherwise trust the index links or the directory tree.
  - Open only the one file you need — don't list or read its siblings.

### 3. Frontmatter-first parsing
- **Rule**: To identify, filter, or catalog concepts, read only the top frontmatter — not the bodies.
- **Protocol**:
  - Parse the YAML block between the first two `---`.
  - Use `type` (an arbitrary producer-defined kind — e.g. `Table`, `File`, `Metric`, `Playbook`), `title`, `description` (the one-line summary — the most useful field for cataloging), and `resource` (canonical URI of the underlying asset).
  - The bundle stores **knowledge about** an asset, not its contents — for filesystem/git bundles the body may be only metadata. To read the asset itself, dereference its `resource` URI: for `file://`, drop the scheme and URL-decode to a local path; other schemes (`bq://`, `postgres://`, `git://`, …) identify the asset but need the matching connector or credentials to fetch.

### 4. Search by name or keyword with grep
- **Rule**: To find a field, key, column, or any keyword across the bundle, don't open files one by one.
- **Protocol**: Run `grep`/`ripgrep` across the bundle directory and open only the files that match. For frontmatter-only filters, grep anchored lines (e.g. `^type:`, `^title:`).

### 5. Follow links for relationships — when they exist
- **Rule**: Relationships are expressed as standard markdown links between concepts; don't infer them. Some bundles are richly cross-linked; others (e.g. filesystem/git) encode structure only as index/tree containment and have **no** body links — tolerate their absence rather than forcing a graph.
- **Protocol**: Scan a concept body for links to other concept files and follow them to build a relationship view. Treat a broken link as not-yet-written knowledge, not an error.

### 6. Source-side catalog: `.okf-metadata.yaml`
- **Rule**: When pointed at a **source directory** (not a bundle) that has been ingested, a `.okf-metadata.yaml` at its root is a flat `path: description` catalog — read it directly for an instant index, no bundle required.
- **Protocol**: Inside a bundle you don't need it — the same descriptions already live in each concept's frontmatter `description` (Rule 3). Reach for `.okf-metadata.yaml` only when summarizing the source itself.
