---
name: okf-enrich
description: Guidance for an AI agent to enrich an Open Knowledge Format (OKF) bundle with high-quality concept descriptions using its own LLM — grounded in the bundle's schema, data profile, and samples — then optionally sync them back to the source. Use when an OKF bundle has missing, weak, or low-quality descriptions that should be improved before publishing or ingestion. Instructions-only; no binary required.
license: Apache-2.0
metadata:
  version: "0.1.0"
  author: Yurii Serhiichuk
  tags: "okf, enrichment, documentation, agent-guidance, prompt-engineering"
---

# OKF Bundle Enrichment Guidance Skill

This skill teaches an AI agent (Claude Code, Cursor, Gemini CLI, Copilot, …) how to **enrich** an Open Knowledge Format (OKF) bundle — adding or improving the human-readable `description` of each concept (table, dataset, file, directory) — using the agent's **own** LLM.

There is deliberately **no binary and no embedded model** here. Generating a good description is a judgment task, and the harness driving the project already has a capable LLM in the loop. Embedding a second one would mean a model calling a tool that calls another model: redundant cost, an extra API key to manage, and usually a worse result than the model already doing the work. So enrichment is delivered as guidance — the procedure and the quality bar — for whatever LLM is present, exactly as `okf-reader` is guidance for *reading* a bundle.

## When to Use

Load this skill when asked to enrich, document, describe, annotate, or "improve the descriptions in" an OKF bundle — typically after a connector has produced the bundle and before syncing descriptions back to the source.

Pairs with:
- **`okf-reader`** — follow its rules to read and navigate the bundle efficiently (index-first, frontmatter-only when possible, grep for targeted lookups).
- **the connectors** (`okf-sqlite`, `okf-mysql`, `okf-postgresql`, `okf-bigquery`, `okf-fs`, `okf-git`) — the producers and the sync target. Enrichment is far better when the bundle was produced with `--profile` and `--sample` (the four SQL connectors), and the descriptions you write can be pushed back to the origin with the connector's `ingest --sync`.

## The OKF concept document

Each concept is a markdown file with YAML frontmatter:

```markdown
---
type: SQLite Table
title: orders
description:                       # <- the field you write
resource: sqlite:///.../orders
tags: [sqlite, table]
timestamp: 2026-06-13T12:00:00Z
---
# Columns

| Name | Type | Primary Key | Nullable | Default |
| ---  | ---  | ---         | ---      | ---     |

## Data Profile                    # present only when produced with --profile

| Column | Non-Null | Null | Distinct | Min | Max |
| ---    | ---      | ---  | ---      | --- | --- |

## Sample                          # present only when produced with --sample

| id | customer_id | total | status |
| ...
```

Your enrichment target is the frontmatter **`description`** field. For sources that carry per-column comments (MySQL, PostgreSQL, BigQuery — their `# Columns` table includes a `Comment`/`Description` column), you may also fill the empty cells in that column.

## Procedure

### 1. Discover concepts (index-first)
Follow the `okf-reader` rules: read `index.md` first and use it to locate concept files; route directly to the files you need. Do **not** recursively read the whole bundle.

### 2. Decide what to enrich
Enrich a concept when its `description` is empty or a generic placeholder the connector inserted (e.g. `"SQLite table orders"`, `"File config.yaml"`, `"No description available"`, `"Git file main.go"`). Do **not** overwrite a substantive, human- or source-authored description unless the user explicitly asks you to regenerate. This keeps the operation idempotent and safe to re-run.

### 3. Gather grounding (never guess)
Base every claim on evidence in the document. Read only what you need:
- **Schema** (`# Columns`): names, types, keys, nullability → the shape of the concept.
- **`## Data Profile`** (if present): per-column non-null / null / distinct / min / max. Signals: a 2–3-distinct column is likely a flag, enum, or status; min/max timestamps reveal the time span the data covers; a high null ratio flags optional fields.
- **`## Sample`** (if present): real example rows — the strongest signal for what the data actually *means*.
- **Relationships**: links in the body to other concept files → how this concept connects to others.

If the profile/sample sections are absent, enrich from the schema alone — but prefer to (re)produce the bundle with `--profile --sample` first when you can; it yields markedly better descriptions.

### 4. Write the description
- **Grain first**: state what one row / record / file represents, then its purpose — e.g. *"One row per customer order, capturing line-item totals, payment status, and the placing customer."*
- **Length**: one sentence for a table / dataset / file; a short noun phrase for a column.
- **Ground every claim** in the schema/profile/sample. Do not invent business meaning the evidence doesn't support. If the purpose is genuinely ambiguous, describe the structure and note what's uncertain rather than fabricating.
- **Add meaning, don't restate**: don't just list the columns the reader can already see — convey what the schema alone doesn't tell them.

### 5. Write back surgically
- Set **only** the frontmatter `description` field. Preserve `type`, `title`, `resource`, `tags`, `timestamp`, and the **entire** markdown body (including the Columns, Data Profile, and Sample sections) unchanged.
- Where the source carries per-column comments (see the **Source variations** table below), fill only the **empty** cells in that column; leave populated cells and every other cell untouched.
- Never modify `index.md` or `log.md`.

### 6. Close the loop (optional)
To persist enriched descriptions back to the origin system, run the matching connector's `ingest --sync`. See the **Source variations** table below for exactly what each connector writes back — and note that SQLite has no comment mechanism, so SQLite enrichment stays in the bundle (the descriptions still serve the catalog and any agent reading it).

The full flow:

```
<connector> produce --profile --sample   →   enrich (this skill)   →   <connector> ingest --sync
```

## Source variations

Enrichment is the same procedure for every source — only three things differ per connector: where a description can live, what `ingest --sync` persists it to, and what to lean on when writing it. This table is the single place that per-source knowledge lives; the connectors themselves stay deterministic extract/sync tools.

| Connector | Concept `type` | Description target(s) | `ingest --sync` writes to | Grounding signal |
|---|---|---|---|---|
| `okf-sqlite` | `SQLite Table` | frontmatter `description` only | schema only — **no** description sync (SQLite has no comments); enrichment stays in the bundle | `# Columns` + `## Data Profile` + `## Sample` |
| `okf-mysql` | `MySQL Table` | frontmatter `description` + `Comment` column | table & column **comments** (`ALTER TABLE … COMMENT`) | `# Columns` + profile + sample |
| `okf-postgresql` | `PostgreSQL Table` | frontmatter `description` + `Comment` column | table & column comments (`COMMENT ON …`) | `# Columns` + profile + sample |
| `okf-bigquery` | `BigQuery Table` | frontmatter `description` + `Description` column | table & field **descriptions** (BigQuery API) | `# Columns` + profile + sample |
| `okf-fs` | `File` / `Directory` | frontmatter `description` only | `.okf-metadata.yaml` | path, extension, size — infer role from name/type (no data content) |
| `okf-git` | `Git File` / `Git Directory` | frontmatter `description` only | `.okf-metadata.yaml` | path + last commit author/date/**message** in the body |

## Quality rules (summary)

1. **Ground, don't guess** — evidence in the document backs every word you write.
2. **One field, surgical edits** — touch `description` (and empty comment cells); preserve everything else byte-for-byte.
3. **Concise and purposeful** — grain plus purpose, never a restated schema.
4. **Idempotent** — don't clobber real descriptions; the procedure is safe to re-run.
