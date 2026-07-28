# OKF Skills — Future Development

**Status:** Proposal / roadmap
**Date:** 2026-06-14
**Scope:** Three deliberate directions — (1) more connectors, (2) optimizations for
smart enrichments, (3) visualizer improvements — plus the cross-cutting
foundations they share.

This document proposes where the project goes next. It is written to respect the
five principles that already define the project (see
`skills/okf-producer-generator/SKILL.md`): deterministic extraction with **no
embedded LLM**, `okf-go` as the single source of OKF types, `schema` as the
MCP-discovery contract, the fixed `produce`/`ingest`/`schema` command surface, and
pure-Go portability. Nothing below asks us to break those; several items make them
pay off harder.

---

## 0. Two foundations that unlock all three directions

Before the per-direction roadmaps, two gaps in the current code touch every
direction at once. Doing these first multiplies the value of everything after.

### 0.1 Relationship extraction (typed cross-links)

**Today:** Every SQL connector emits only `# Columns` with `Name | Type | Primary
Key | Nullable | Default` (see `okf-producer-generator/SKILL.md` §3). A
`grep -ri foreign` across `skills/` and `okf-go/` finds foreign keys **only in the
spec's hand-written examples** — no connector actually extracts them. That means
the graph in `okf-viz` shows containment edges plus whatever cross-links an
enrichment agent *happened* to type into prose (`links.go` parses `[text](../x.md)`
from the body). The richest structural signal a database has — its foreign keys —
is thrown away.

**Proposal:** Add deterministic relationship extraction to every connector that has
relationships to give:

- **SQL connectors** (`okf-sqlite`, `okf-mysql`, `okf-postgresql`, `okf-bigquery`):
  read FK constraints (`PRAGMA foreign_key_list` for SQLite,
  `information_schema.KEY_COLUMN_USAGE` / `REFERENTIAL_CONSTRAINTS` for MySQL &
  Postgres, `INFORMATION_SCHEMA` views for BigQuery) and emit a `# Relationships`
  section with bundle-relative links (`FK to [customers](/tables/customers.md) on
  customer_id`). This makes the column already shown in the spec example (§4.3) a
  *generated* fact rather than a manual one.
- **`okf-git`** already has commit data — emit co-change edges (files frequently
  committed together) as a `# Related Files` section.
- Keep it deterministic: the connector emits the *edge*; `okf-enrich` later
  explains *what the relationship means* in prose.

**Where:** new `relationships.go` per connector; a shared
`okf.RenderRelationshipsSection(...)` helper in `okf-go` mirroring
`RenderProfileSection`. Because edges are encoded as ordinary markdown links,
`okf-viz` picks them up **with zero changes** — but see §3.1 for making them
*typed*.

**Why it matters across directions:** richer connectors (D1), far better
enrichment grounding (D2 — the LLM can describe a join instead of guessing), and a
genuinely connected graph instead of a containment tree (D3).

### 0.2 Incremental produce (content hashing / change detection)

**Today:** A `grep` for `incremental|--since|mtime|checksum|hash` across the
connectors finds nothing (only `okf-viz/cdn.go`'s SRI hashes). Every `produce` is a
full re-extraction that rewrites every concept file, which (a) churns git diffs,
(b) destroys the idempotency `okf-enrich` relies on, and (c) makes large sources
slow.

**Proposal:** Add a `--since` / change-detection path:

- Store a per-concept `content_hash` of the *structural* portion (schema +
  profile, excluding the enriched `description`) in frontmatter.
- On re-produce, if the structural hash is unchanged, **preserve the existing
  file byte-for-byte** — crucially preserving any human/agent-written
  `description`. Only rewrite concepts whose structure actually changed.
- Append a `log.md` entry per change (the spec already reserves `log.md` for
  exactly this, OKF-SPEC §7) — turning the bundle into a real change history.

**Where:** `okf-go` gains `ConceptStructuralHash(doc)` and a merge helper; each
connector calls it in its produce loop. This is the single highest-leverage
correctness fix in the repo: it makes the documented
`produce → enrich → ingest` loop safe to run repeatedly without clobbering
enrichment.

---

## 1. Direction 1 — More Connectors

The project has six connectors across three shapes (tabular DB, filesystem, VCS).
The `okf-producer-generator` skill makes adding more cheap and consistent. The goal
is breadth of *source shapes*, not just count — each new shape teaches the
`--sync` patterns something new.

### 1.1 Prioritized connector roadmap

Ordered by value ÷ effort. Effort assumes copying the nearest existing reference
connector per the generator skill.

| Priority | Connector | Source shape | Nearest reference | `--sync` target | Notes |
|---|---|---|---|---|---|
| **P0** | `okf-csv` | Flat files / dir of CSVs | `okf-fs` + SQL profiler | `.okf-metadata.yaml` | Pure Go (`encoding/csv`); reuses the profile/sample machinery directly. Huge install base, zero credentials, great demo. |
| **P0** | `okf-mongodb` | Document store | `okf-sqlite` (structure) | validate-only | Driver `go.mongodb.org/mongo-driver` is CGO-free (already called out in the generator skill). Infer schema by sampling docs → `Name | Type | Presence` columns. |
| **P1** | `okf-openapi` | HTTP API spec | `okf-fs` | validate-only | Parse an OpenAPI/Swagger doc into `API Endpoint` / `Schema` concepts. The spec explicitly lists `API Endpoint` as a `type`. No live server needed. |
| **P1** | `okf-bigquery` enhancements | (existing) | — | (existing) | Add **views, routines, and partitioning/clustering metadata** — currently table-centric. |
| **P1** | `okf-snowflake` | Cloud DW | `okf-postgresql` | `COMMENT ON` | Snowflake supports comments; closest to an existing pattern. Gated behind a build tag if the driver is heavy. |
| **P2** | `okf-dbt` | Transformation layer | `okf-fs` | `.yml` schema files | dbt's `manifest.json` + `schema.yml` are a goldmine: lineage, tests, existing descriptions. Maps cleanly to relationship edges (§0.1). |
| **P2** | `okf-kafka` | Event streams / schema registry | `okf-sqlite` | validate-only | Topics + Avro/Protobuf schemas from a schema registry → `Topic` / `Message Schema` concepts. |
| **P2** | `okf-parquet` | Columnar files | `okf-csv` | `.okf-metadata.yaml` | Embedded schema + rich stats (min/max/null counts) map *directly* onto `ColumnProfile` with no scan needed. |
| **P3** | `okf-redis` | KV / cache | `okf-fs` | validate-only | Keyspace patterns, types, TTLs as concepts. Demonstrates a non-tabular live source. |
| **P3** | `okf-graphql` | API schema | `okf-openapi` | validate-only | Introspection query → types/queries/mutations as concepts with native relationship edges. |
| **P3** | `okf-airtable` / `okf-notion` | SaaS knowledge | `okf-bigquery` (API+token) | API write-back | SaaS sources with native description fields — good `--sync` round-trip story. |

**Recommended first wave:** `okf-csv`, `okf-mongodb`, `okf-openapi`. They cover the
three missing shapes (flat-file, schemaless document, API contract), need no exotic
drivers, and `okf-csv`/`okf-openapi` need no credentials — ideal for the codelab
and the case study in `docs/`.

### 1.2 Connector *capability* upgrades (not new sources)

New connectors are only half the story; the existing six can do more:

1. **Relationship extraction** — §0.1. The biggest single upgrade.
2. **Incremental produce** — §0.2.
3. **Constraint & index metadata** — emit unique constraints, check constraints,
   and indexes (strong enrichment + viz signals; e.g. a unique index says "this
   column identifies a row").
4. **View/materialized-view awareness** — for SQL connectors, distinguish `Table`
   vs `View` in `type`, and capture the view's defining SQL as a grounding signal.
5. **Row-count & freshness stats** — cheap `COUNT(*)` / max(timestamp) per table
   feeds both enrichment ("covers 2019–2026") and a viz "size" encoding.

### 1.3 Make authoring even cheaper

- **`okf-scaffold` command or script** — codify the generator skill's "copy
  `okf-sqlite`, rename module, wire `go.work`/`Makefile`/`install.sh`/
  `skills.sh.json`" checklist into a one-shot generator so a new connector skeleton
  builds green before any source logic is written.
- **Connector conformance test kit** — a shared, table-driven test in `tests/`
  that any connector can opt into: round-trip `produce → ingest`, root-`index.md`
  carries only `okf_version`, every concept has a non-empty `type`, `schema`
  declares all three commands. Today each connector hand-rolls these (see
  `tests/mcp_integration_test.go`'s `connectors` slice).

---

## 2. Direction 2 — Optimizations for Smart Enrichments

`okf-enrich` is correctly an **instructions-only** skill: enrichment is a judgment
task handled by the agent's own LLM, and the project deliberately deleted an
embedded second model. The optimizations below keep that principle intact. The
theme is: **do more deterministic work up front so the LLM guesses less, costs
less, and is more consistent** — and add the deterministic scaffolding that makes
enrichment measurable and cheap to re-run.

### 2.1 Richer deterministic grounding (better signal, same model)

Enrichment quality is bounded by the evidence in the concept doc. Today that's
`# Columns`, `## Data Profile`, `## Sample` (see `okf-enrich/SKILL.md` §3). Add
deterministic, connector-side signals that make the model's job nearly mechanical:

- **Relationships (§0.1)** — the single biggest grounding win. "FK to `customers`"
  lets the model describe a fact instead of inferring one.
- **Semantic-type / pattern hints** — extend `okf.ColumnProfile` (currently
  `NonNull/Null/Distinct/Min/Max` in `okf-go/profile.go`) with a deterministic
  detected pattern: looks-like-email, UUID, ISO-timestamp, monetary, enum
  (low-distinct), boolean, foreign-key-ish. Pure regex/heuristics, no LLM. A
  `Distinct=2` column flagged `enum` plus its two sample values is almost
  self-describing.
- **Value distribution for low-cardinality columns** — for `Distinct ≤ N`, emit
  the actual distinct values (`status ∈ {pending, shipped, cancelled}`). This is
  the strongest possible signal for status/enum columns and is cheap to compute.

**Where:** extend `ColumnProfile` and `RenderProfileSection` in `okf-go/profile.go`
(shared by all SQL connectors automatically), plus a new
`okf.DetectSemanticType(samples []string) string`.

### 2.2 Cost & consistency optimizations for the enrichment pass

The agent's LLM is the cost center. Make each token count:

- **Prioritization / triage** — guidance + a tiny helper to rank concepts by
  enrichment value: high graph **degree** (already computed in `okf-viz/links.go`),
  high row count, missing description, many downstream FK references. Enrich the
  hubs first; the long tail can wait. Add this as a documented strategy in
  `okf-enrich/SKILL.md` plus an optional `okf-enrich` *coverage report* (see 2.4).
- **Description propagation / dedup** — a column named `customer_id` recurs across
  dozens of tables. Today each is enriched independently (redundant tokens, drifting
  wording). Propose a deterministic **glossary** pass: a `glossary.md` concept (or
  `.okf-glossary.yaml`) of canonical term → definition; the enrichment guidance
  says "reuse the glossary definition for a known term; only write a fresh
  description for novel meaning." Consistency *and* token savings.
- **Batching guidance** — explicit instruction to enrich a whole directory in one
  grounded pass (one read of index + frontmatter, per `okf-reader`) rather than
  file-by-file round-trips. Reduces context re-loading.
- **Idempotency markers** — building on §0.2's structural hash, record which
  description was generated against which structural hash. Re-running enrichment
  then *skips* concepts whose structure is unchanged and whose description is
  current — turning re-enrichment from O(bundle) into O(changes).

### 2.3 Relationship & body enrichment, not just `description`

`okf-enrich` today targets the frontmatter `description` (and empty comment cells).
Two natural extensions, still LLM-as-judge, still surgical:

- **Explain relationships** — given the deterministic FK edges from §0.1, have the
  agent write a one-line semantics for each (`# Relationships` prose: "one customer
  has many orders"). The connector supplies the edge; the model supplies meaning.
- **Tag suggestion** — propose `tags` from schema/sample evidence (e.g. detecting
  PII-shaped columns → `pii` tag), with the same idempotent, don't-clobber rules.
  Tags drive `okf-viz` filtering, so this directly improves D3.

### 2.4 Make enrichment *measurable* (the missing feedback loop)

Right now there is no way to see how well-enriched a bundle is. Add a deterministic
(no-LLM) **enrichment coverage report**:

- A small tool/command (could live in `okf-viz` or a new `okf-lint`) that scans a
  bundle and reports: % concepts with non-placeholder descriptions, % columns
  commented, broken cross-link count, concepts missing `type`, orphan nodes
  (degree 0), placeholder-still-present count.
- Detect placeholders deterministically — the connectors emit known patterns
  (`fmt.Sprintf("SQLite table %s", name)`, `"File %s"`, etc., per the generator
  skill), so a regex catalog can flag "not yet enriched" precisely.
- Surface it in `okf-viz` as a **coverage badge / heatmap** (ties to D3) and as CI
  output ("bundle is 62% enriched") so teams can gate publishing.

This closes the loop: produce → **measure** → enrich the gaps → re-measure.

### 2.5 An enrichment evaluation harness (quality, not just coverage)

Coverage counts descriptions; it doesn't judge them. Propose a lightweight,
**optional** LLM-as-judge eval (run by the agent, consistent with the
no-embedded-model rule): given schema+profile+sample and a candidate description,
score grounding/specificity/conciseness against the `okf-enrich` quality rules. Use
it to (a) regression-test prompt guidance changes and (b) flag low-confidence
descriptions for human review. Keep it a `docs/`-described workflow + fixtures, not
a binary.

---

## 3. Direction 3 — Visualizer Improvements

`okf-viz` is already strong: a single self-contained `index.html`, three panes
(navigator tree + type/tag filters + search, a Cytoscape graph with seven layouts
and an edge-kind legend, and a goldmark-rendered reader), offline vendoring, and
deterministic output (`render.go`, `model.go`, `links.go`). Improvements below are
ordered roughly by value.

### 3.1 Typed, semantic edges (pairs with §0.1)

**Today:** `links.go` collapses everything to two kinds — `containment` (dashed)
and `crosslink` (solid). Once connectors emit `# Relationships` (§0.1), the graph
should distinguish a *foreign key* from a *see-also* from a *co-change* edge.

**Proposal:** carry an edge `label`/`relation` through `Edge` (model.go) — derived
either from the section the link came from (`# Relationships` → `references`,
`# Joins` → `joins-with`) or from a `[fk](...)`-style annotation. Render distinct
colors/styles and add them to the existing collapsible legend. This is the change
that turns the graph from "a tree with some extra lines" into an **ER/lineage
diagram**.

### 3.2 ER-diagram / schema mode for tabular bundles

For SQL-shaped bundles, add a layout/mode that renders each table node with its
column list and draws FK edges between specific columns (crow's-foot style).
Cytoscape can do compound/HTML-label nodes; the data is already in each doc's
`# Columns` section. This is the single most-requested view for database catalogs
and directly showcases §0.1.

### 3.3 Scale: handle large bundles

**Today:** the entire model (`nodes`, `edges`, every concept's rendered `bodyHtml`)
is JSON-inlined into one HTML file (`render.go` `Emit`). For a 5,000-table
warehouse this is a multi-MB file and a sluggish DOM-rendered graph.

**Proposals:**
- **Lazy reader payloads** — don't inline every `bodyHtml`; inline frontmatter +
  graph, fetch/expand body on selection (or gate full-inline behind a `--inline-all`
  flag and default to lazy for big bundles). Keeps the single-file default for small
  bundles.
- **Canvas/WebGL rendering** for large graphs, or automatic neighborhood-only
  rendering (show the selected node + N hops) with a "load more" control.
- **Clustering by directory/type** at scale — collapse directories into
  super-nodes that expand on click (the containment data in `model.go` already
  supports this hierarchy).

### 3.4 Navigation & focus

- **Neighborhood focus / "spotlight"** — click a node to dim everything beyond its
  N-hop neighborhood; essential once graphs are dense.
- **Permalinks / shareable state** — encode selected node + active filters + layout
  in the URL hash so a view can be shared or bookmarked (self-contained file stays
  self-contained).
- **Path / lineage tracing** — "show the path from A to B" and "show all upstream of
  X" using the FK/lineage edges. The killer feature for data-lineage use cases.
- **Fuzzy search + jump-to** — current search is full-text; add ranked fuzzy
  matching over titles/types/tags with keyboard nav.

### 3.5 Data-aware reader pane

- **Render `## Data Profile` as charts** — null-ratio bars, distinct-count, min/max
  ranges as inline sparklines instead of a raw markdown table. The data is already
  in the doc; this is presentation only.
- **Resolve cross-links in the reader** — make `[text](/tables/x.md)` links in the
  rendered body click through to that concept's reader+graph selection instead of a
  dead relative link.
- **Enrichment coverage overlay** — color nodes by enrichment state (placeholder vs
  enriched vs human-authored), surfacing §2.4's report visually. A "what still needs
  documenting" heatmap.

### 3.6 Output & integration

- **Static export** — export the current graph as PNG/SVG and export a selected
  subgraph as a new OKF bundle or JSON (for embedding in docs/wikis).
- **Diff mode** — given two bundle versions (or a git ref), highlight
  added/removed/changed concepts and edges. Pairs naturally with §0.2's change
  history and `log.md`.
- **Multi-bundle / federation view** — render several bundles together with
  cross-bundle links, for organizations with many catalogs.
- **Accessibility & i18n** — `--lang` already exists for chrome; complete keyboard
  navigation of the graph, ARIA roles on the panes, and contrast-safe theming for
  the dark/light/system themes already supported.

---

## 4. Cross-cutting foundations

These support all three directions and are worth funding explicitly:

- **`okf-go` as the leverage point** — relationship rendering, semantic-type
  detection, structural hashing, and the coverage scanner all belong in `okf-go` so
  every connector and consumer gets them for free. Keep resisting per-skill
  duplication (AGENTS.md §2).
- **OKF spec evolution (v0.2)** — several proposals imply small, backward-compatible
  spec additions: a conventional `# Relationships` heading, an optional typed-link
  annotation, and a `# Data Profile`/semantic-type convention. These are exactly the
  "new optional fields / new conventional section headings" a minor bump allows
  (OKF-SPEC §11). Draft them as v0.2 and keep consumers permissive.
- **`okf-lint` / conformance tooling** — a deterministic validator (spec
  conformance + enrichment coverage + broken-link report) usable in CI and surfaced
  by `okf-viz`. Complements the existing `skills-ref validate` (which only checks
  `SKILL.md` frontmatter).
- **Performance & determinism guarantees** — keep the byte-stable output property
  (`links.go` already sorts edges for determinism); extend it to new sections so
  diffs stay clean, which §0.2 and §3.6's diff mode both rely on.

---

## 5. Suggested sequencing

A dependency-aware order. Each phase is independently shippable.

**Phase 1 — Foundations (highest leverage):**
1. Relationship extraction in `okf-go` + SQL connectors (§0.1).
2. Incremental produce / structural hashing (§0.2).
3. Semantic-type + value-distribution profiling (§2.1).

→ Immediately improves enrichment quality, connector richness, and the graph at
once.

**Phase 2 — Breadth & measurement:**
4. First connector wave: `okf-csv`, `okf-mongodb`, `okf-openapi` (§1.1).
5. Enrichment coverage report + placeholder detection (§2.4).
6. `okf-scaffold` + shared conformance test kit (§1.3).

**Phase 3 — Visualizer payoff:**
7. Typed edges + ER/schema mode (§3.1–3.2).
8. Coverage heatmap + data-profile charts + cross-link click-through (§3.5).
9. Scale work: lazy payloads + neighborhood focus (§3.3–3.4).

**Phase 4 — Maturity:**
10. dbt / Snowflake / Kafka connectors (§1.1).
11. Diff mode + permalinks + lineage tracing (§3.4, §3.6).
12. OKF v0.2 spec draft + `okf-lint` (§4); enrichment eval harness (§2.5).

---

## 6. Guardrails (don't regress these)

Every proposal above is constrained by the principles that make the project good:

- **No embedded LLM in producers.** Enrichment intelligence stays in the agent's
  own model; connectors stay deterministic (generator skill, principle 1).
- **`okf-go` is the single source of OKF types** (AGENTS.md §2).
- **`schema` stays the only MCP contract** — new commands self-describe and
  `okf-mcp` needs no changes (generator skill, principle 3).
- **Pure Go, zero CGO, one binary per skill** — vet every new driver
  (generator skill, principle 5).
- **Permissive consumption** — `okf-viz` and any new consumer must tolerate
  unknown types, missing fields, and broken links (OKF-SPEC §9).
- **Byte-stable, diffable output** — preserve determinism so version control and
  diff mode stay meaningful.
