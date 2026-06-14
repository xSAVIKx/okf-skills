---
name: okf-producer-generator
description: Guidance and the bundled OKF specification for building a new Open Knowledge Format (OKF) producer/connector skill in this project — the architectural principles, the okf-go library contract, the produce/ingest/schema command surface, the secret-handling and sync conventions, and the exact registration steps. Use when creating a new OKF connector or producer for a data source the project does not yet cover (e.g. MongoDB, Redis, Kafka, CSV, an HTTP API), scaffolding an okf-* skill, or extending the project to a new source. Instructions-only; no binary required.
license: Apache-2.0
metadata:
  version: "0.1.0"
  author: Yurii Serhiichuk
  tags: "okf, producer, connector, skill-authoring, scaffolding, schema, agent-guidance, documentation, code-generation"
---

# OKF Producer Skill Generator

This skill teaches a coding agent (Claude Code, Cursor, Gemini CLI, Copilot, …)
how to author a new **producer** for this project: a standalone CLI skill that
extracts metadata from a data source into an Open Knowledge Format (OKF) bundle,
and ingests/syncs descriptions back. It ships the OKF spec it must conform to,
the conventions every existing connector follows, and the exact wiring steps so
the new skill builds, tests, and is auto-discovered over MCP.

Google introduced OKF and invited the community to *"write a producer, write a
consumer."* This skill is how you write a producer **here** — one that matches
the six that already exist (`okf-sqlite`, `okf-mysql`, `okf-postgresql`,
`okf-bigquery`, `okf-fs`, `okf-git`) instead of reverse-engineering them.

## When to Use

Load this skill when asked to:
- **Add a new data source** — "make an OKF connector for MongoDB / Redis / Kafka / CSV / an API".
- **Scaffold an `okf-*` producer skill**, or extend the project to a source it doesn't cover.

Do **not** use it for: reading a bundle (use `okf-reader`), enriching descriptions
(use `okf-enrich`), or building a *consumer* that only reads OKF (the spec in
`okf-SPEC.md` is enough for that).

## Read the spec first

The format your producer must emit is defined in **[`okf-SPEC.md`](./okf-SPEC.md)**
(bundled here; canonical source is `okf-go/okf-SPEC.md`). The library API you build
against is in **[`okf-go-api.md`](./okf-go-api.md)**. Skim both before designing
concept types. The rules you will lean on most:

- A bundle is a directory tree of markdown files with YAML frontmatter.
- The **only required frontmatter field is `type`**; everything else is optional.
- `index.md` files carry **no frontmatter**, except the **bundle-root** `index.md`,
  which may declare **only** `okf_version: "0.1"`.
- Consumers are permissive: unknown `type` values, extra keys, and broken links
  are all tolerated. Conformance is just "every concept has parseable frontmatter
  with a non-empty `type`."

## The five principles that define a producer

These are not obvious from reading one connector. Internalize them before writing code.

1. **Deterministic extraction — no embedded LLM.** `produce` and `ingest` are
   mechanical: read the source, emit/compare markdown. **Never call an LLM inside
   `produce` to write descriptions.** Emit a deterministic placeholder (e.g.
   `fmt.Sprintf("MongoDB collection %s", name)`); the meaning is added later by
   `okf-enrich`, which uses the agent's *own* LLM. (This project deliberately
   deleted an embedded second model — a model calling a tool that calls another
   model is redundant cost and usually worse output.)

2. **`okf-go` is the single source of OKF types.** Import it; never redefine
   `Frontmatter` or `ConceptDoc`. Its `WriteConceptDoc`/`ReadConceptDoc` and section
   helpers are what keep your output spec-conformant and round-trippable.

3. **`schema` is the contract.** Implement the `schema` subcommand and `okf-mcp`
   auto-discovers your binary and exposes `produce`/`ingest` as MCP tools — **no
   `okf-mcp` changes ever needed.** Secret-bearing flags declare an `Env` binding so
   `okf-mcp` passes them via the environment, never argv.

4. **Always three subcommands: `produce`, `ingest`, `schema`.** Register flags on a
   per-subcommand `flag.NewFlagSet(...)` — never global `flag.Bool(...)`.

5. **Portable, pure Go.** Zero CGO — verify any source driver you add is pure Go
   (e.g. `modernc.org/sqlite` not `mattn/go-sqlite3`; MongoDB's
   `go.mongodb.org/mongo-driver` is cgo-free). One binary named `okf-<name>`. Map
   `okf-go` with a local `replace` directive so it builds in a fresh clone.

## Anatomy of a producer skill

```
skills/okf-<name>/
├── SKILL.md        # frontmatter (spec-compliant) + When to Use / Setup / produce / ingest / schema
├── go.mod          # module okf-<name>; go 1.24.0; replace okf-go => ../../okf-go
├── main.go         # main() router + runProduce + runIngest + source-specific helpers
├── schema.go       # buildSchema() okf.SkillSchema
└── *_test.go       # schema_test.go (asserts name + commands) + a pure-function test
```

The cleanest starting point is to **copy `skills/okf-sqlite/`** (the minimal,
dependency-light reference producer) and adapt it. For a live-server source that
needs credentials, also look at `skills/okf-mysql/` for the env-secret pattern;
for a non-tabular / file-like source, look at `skills/okf-fs/` and `skills/okf-git/`.

## Build sequence

### 1. Map the source to OKF concepts
Decide: the concept **`type`** string (e.g. `"MongoDB Collection"`); the bundle
**layout** (a subdirectory like `tables/` for SQL or `collections/` for Mongo);
and the **`Resource` URI** scheme that uniquely identifies each asset
(`sqlite:///…/orders`, `bigquery://proj/ds/tbl`). **Strip any credentials from the
URI** before putting it in `Resource` — it is written to disk.

### 2. Scaffold the module
Copy `okf-sqlite` to `skills/okf-<name>/`, rename the module in `go.mod`, and keep
the toolchain line and `replace` exactly as the source skill has them (copy
`go 1.24.0` verbatim — do not invent a version; the workspace `go.work` line is
separate and you don't edit it per-skill). Add any pure-Go driver you need.

### 3. Implement `produce`
For each asset, build one `ConceptDoc` and write it with `okf.WriteConceptDoc`:

```go
var body bytes.Buffer
body.WriteString("# Columns\n\n")
body.WriteString("| Name | Type | Primary Key | Nullable | Default |\n")
body.WriteString("| --- | --- | --- | --- | --- |\n")
for _, c := range cols {
    fmt.Fprintf(&body, "| %s | %s | %s | %s | %s |\n",
        okf.SanitizeCell(c.Name), okf.SanitizeCell(c.Type), pk(c), null(c), okf.SanitizeCell(c.Default))
}
bodyStr := body.String()
if *profile { bodyStr = okf.UpsertSection(bodyStr, "Data Profile", okf.RenderProfileSection(profiles)) }
if *sample > 0 { bodyStr = okf.UpsertSection(bodyStr, "Sample", okf.RenderSampleSection(headers, rows)) }

doc := okf.ConceptDoc{
    Frontmatter: okf.Frontmatter{
        Type:        "MongoDB Collection",                 // REQUIRED
        Title:       name,
        Description: fmt.Sprintf("MongoDB collection %s", name), // deterministic placeholder
        Resource:    resourceURI,                          // no credentials
        Tags:        []string{"mongodb", "collection"},
        Timestamp:   timestamp,                             // time.Now().Format(time.RFC3339), computed once
    },
    Body: bodyStr,
}
okf.WriteConceptDoc(filepath.Join(outDir, "collections", name+".md"), doc)
```

Then write the **bundle-root `index.md`** — the only index with frontmatter, and
**only** `okf_version`:

```go
okf.WriteConceptDoc(filepath.Join(outDir, "index.md"), okf.ConceptDoc{
    Frontmatter: okf.Frontmatter{OKFVersion: "0.1"}, // nothing else
    Body:        indexBody.String(),                 // "# …", then "- [name](collections/name.md) - …" lines
})
```

**Schema-table heading & columns.** Emit the schema under a level-1 **`# Columns`**
heading — use `# Columns`, **not** the spec's conventional `# Schema` (§4.2) —
because `ingest` isolates it with `okf.GetSectionAny(body, "Columns")`, and the
helper-written `## Data Profile` / `## Sample` are level-2 sections beneath it. The
columns are **source-defined**: emit whatever structural attributes are
authoritative for the source (SQL: `Name | Type | Primary Key | Nullable | Default`;
a document store: `Name | Type | Presence`). If the source has **native per-item
comments** (MySQL, PostgreSQL, BigQuery), add a `Comment`/`Description` column and
leave its cells empty for `okf-enrich` to fill; if it does **not** (SQLite, and most
schemaless sources), emit structure only.

### 4. Implement `ingest`
Read each concept with `okf.ReadConceptDoc`. **Isolate the schema table before
parsing rows** so profile/sample rows aren't misread as columns:

```go
section := doc.Body
if s, ok := okf.GetSectionAny(doc.Body, "Columns"); ok { section = s }
cols := parseColumns(section)
```

`ingest` verifies the bundle against the source; `--sync` then **persists the
enriched descriptions back to the source.** Where they can go depends on the source
— pick from the three patterns this project already uses, and **do not invent a new
mechanism:**

| Source has… | Where `--sync` persists descriptions | Examples |
|---|---|---|
| A native comment/description store | write comments via the source's API/DDL | `okf-mysql`/`okf-postgresql` (`COMMENT`), `okf-bigquery` (field descriptions) |
| No comment store, but is file-rooted | `.okf-metadata.yaml` sidecar via `okf.ReadFolderMetadata`/`WriteFolderMetadata` | `okf-fs`, `okf-git` |
| No comment store and isn't file-rooted | nowhere — descriptions stay in the bundle; `--sync` reconciles only structure (or is validate-only) | `okf-sqlite` (its `--sync` creates missing tables/columns but cannot store descriptions) |

So for a comment-less, non-file source (e.g. MongoDB), **descriptions stay in the
bundle** — make `--sync` validate-only or structure-only rather than inventing a
sidecar collection. (For MySQL specifically, DDL has no placeholders — escape `'`
and `\` before formatting comments into `ALTER TABLE … COMMENT`.)

### 5. Implement `schema`
Return an `okf.SkillSchema` from `buildSchema()` (see `okf-go-api.md`). Declare
`produce`, `ingest`, `schema`; mark required flags; and set **`Env`** on every
secret-bearing flag (password, token, connection URI). Resolve that same env var
as a fallback in your command code, and **never log secrets or place them in argv
or `Resource`.**

### 6. Tests
Mirror the existing skills: a `schema_test.go` asserting `buildSchema()` returns
the right `Name` and the three commands, plus at least one pure-function test
(e.g. your column-table parser).

## okf-go helpers at a glance

Full reference: **[`okf-go-api.md`](./okf-go-api.md)**. Most-used:
`WriteConceptDoc`, `ReadConceptDoc`, `GetSectionAny`, `UpsertSection`,
`RenderProfileSection`/`RenderSampleSection`, `SanitizeCell`,
`ReadFolderMetadata`/`WriteFolderMetadata`, `SkillSchema`+`PrintSchema`.

## Registration checklist (a producer is a binary skill)

Adding the source files is not enough. Wire it in:

- [ ] `skills/okf-<name>/` exists with `go.mod` (+ `replace`), `main.go`, `schema.go`, `SKILL.md`, tests.
- [ ] **`go.work`** — add `./skills/okf-<name>` to the `use (…)` block (keep it alphabetical).
- [ ] **`Makefile`** — add `okf-<name>` to `SKILLS :=`.
- [ ] **`skills.sh`** — add `okf-<name>` to `SKILLS=`.
- [ ] **`skills.sh.json`** — add `okf-<name>` to the appropriate `groupings` entry (every `skills/` skill must appear in one).
- [ ] **`README.md`** — add a row to the "Available Connectors" table (§1) and to the §8 group table.
- [ ] **`AGENTS.md`** — list the directory in the §1 tree.
- [ ] **`tests/`** *(recommended)* — add an integration test; if you want the shared schema-contract check to cover the binary, add `"okf-<name>"` to the `connectors` slice in `tests/mcp_integration_test.go`. DB sources: add a service to `docker-compose.yml` and guard the test with a port check so it skips when down.
- [ ] **`okf-mcp`** — **nothing.** Auto-discovered once it builds and answers `schema`.
- [ ] `go mod tidy` in the new module, then `make build` and `cd tests && go test -v .`.
- [ ] `skills-ref validate skills/okf-<name>` (frontmatter spec compliance).

**`SKILL.md` frontmatter rules:** only `name`, `description`, `license`,
`compatibility`, `metadata`, `allowed-tools` are permitted at the top level; put
`version`/`author`/`tags` **under `metadata:`** as strings (a top-level `version:`
or `tags:` fails `skills-ref validate`). `name` must equal the directory name. Set
`license: Apache-2.0`. `compatibility`, when present, is a **free-text string**
(e.g. `compatibility: Requires the Go toolchain (1.24+) to build the connector
binary.`) — not a structured object; copy the phrasing from an existing connector.
Write `description` as *what it does + when to use it* with searchable keywords.

## Common mistakes

| Mistake | Fix |
|---|---|
| Calling an LLM in `produce` to generate descriptions | `produce` is deterministic; write a placeholder, let `okf-enrich` add meaning |
| Inventing a new `--sync` store (e.g. a metadata collection) | Reuse one of the three patterns; default to validate-only |
| Credentials in `Resource` or on the command line | Strip creds from URIs; bind secrets via `FlagSchema.Env`; never log them |
| Redefining `Frontmatter`/`ConceptDoc` in the skill | Import `okf-go` |
| Frontmatter in a subdir `index.md`, or extra keys in the root `index.md` | Subdir indexes: none. Root index: `okf_version` only |
| Emitting the schema under `# Schema` (the spec's conventional heading) | Use `# Columns` — `ingest` parses it via `GetSectionAny("Columns")` |
| Parsing the whole body for columns (profile/sample rows leak in) | `okf.GetSectionAny(body, "Columns")` first |
| A structured `compatibility:` object in `SKILL.md` | It's a free-text string; nest only `version`/`author`/`tags` (under `metadata`) |
| Global `flag.Bool(...)` | `flag.NewFlagSet(<cmd>, …)` per subcommand |
| CGO SQLite driver, or any cgo dep | Pure-Go drivers (`modernc.org/sqlite`) |
| Top-level `version:`/`tags:` in `SKILL.md` | Nest under `metadata:` |
| Forgetting `go.work` / `Makefile` / `skills.sh` / `skills.sh.json` | Follow the registration checklist |

## Verify before you call it done

1. `make build` succeeds and produces `skills/okf-<name>/okf-<name>`.
2. `okf-<name> produce …` on a real source emits a bundle where every concept has a
   non-empty `type` and the **root `index.md` carries only `okf_version`**.
3. `okf-<name> ingest …` round-trips (verifies, and `--sync` writes to the chosen target).
4. `okf-<name> schema` prints valid JSON; `okf-mcp --skills-dir <dir>` lists
   `okf-<name>__produce` / `okf-<name>__ingest`.
5. `skills-ref validate skills/okf-<name>` passes.
