# Open Knowledge Format (OKF) Skills

**Turn the structure your data already has — database schemas, column comments, foreign keys, file trees, commit history — into a browsable, agent-readable knowledge catalog.**

OKF Skills are small, deterministic connectors that **produce** an [Open Knowledge Format](https://github.com/GoogleCloudPlatform/knowledge-catalog/tree/main/okf) bundle (a directory of Markdown + YAML) from any source, let an LLM **enrich** it with grounded descriptions, **visualize** it as an interactive graph, and **sync** descriptions back to the source. Extraction is pure and reproducible — no embedded model — so the only LLM in the loop is *your* agent's, guided by instructions. Every connector is a single portable binary that self-describes over MCP, so it drops into any agent harness.

## 🚀 Get started

### 1. Install

The quickest way — add the whole skill set to your agent through the [skills.sh](https://www.skills.sh) registry:

```bash
npx skills add xSAVIKx/okf-skills
```

…or build + install the binaries (all connectors + the `okf-mcp` server) from a clone:

```bash
git clone https://github.com/xSAVIKx/okf-skills && cd okf-skills
./install.sh                       # builds + installs to ~/.local/bin
# ./install.sh /usr/local/bin      # …or a directory of your choice
```

…or grab just what you need, no clone (Go 1.24+):

```bash
go install github.com/xSAVIKx/okf-skills/okf-mcp@latest            # the MCP server
go install github.com/xSAVIKx/okf-skills/skills/okf-sqlite@latest  # one connector
```

Ensure the install directory is on your `PATH` — `okf-sqlite --version` should print a version.

### 2. Connect it to your agent

`okf-mcp` discovers the installed `okf-*` skills and exposes each command as an MCP tool named `<skill>__<command>` (e.g. `okf-sqlite__produce`, `okf-viz__render`). Wire it into your harness once:

**Claude Code**
```bash
claude mcp add okf-skills okf-mcp
```

**Gemini CLI**
```bash
gemini mcp add okf-skills okf-mcp
```

**Cursor / Codex / Windsurf / any MCP client** — add an MCP server entry:
```json
{
  "mcpServers": {
    "okf-skills": { "command": "okf-mcp" }
  }
}
```
`okf-mcp` scans your `PATH` for skills by default; pin a directory with `"args": ["--skills-dir", "/path/to/bin"]`. (Codex uses the TOML equivalent: `[mcp_servers.okf-skills]` with `command = "okf-mcp"`.)

### 3. Use it

Ask your agent in plain language — it drives the tools for you:

> *"Catalog my SQLite database at ./app.db, enrich the table descriptions, and render a visual graph."*

It calls `okf-sqlite__produce` (→ an OKF bundle), follows the `okf-enrich` guidance to write grounded descriptions, then `okf-viz__render`s a self-contained `index.html`. Prefer the CLI? The same flow by hand:

```bash
okf-sqlite produce --db ./app.db --out ./catalog --profile   # extract → OKF bundle
# enrich descriptions (by hand, or with an agent following the okf-enrich guidance)
okf-sqlite ingest  --db ./app.db --bundle ./catalog --sync    # push descriptions back
okf-viz    render  --bundle ./catalog                         # → ./catalog/index.html
```

## Why OKF?

Your data sources are full of structure that never reaches the people — or agents — who need it: schemas, `COMMENT ON` text, foreign keys, indexes, a repo's file tree and commit history. OKF captures it as plain Markdown + YAML — **diffable, greppable, reviewable in a PR**, and equally readable by a human, an LLM, or `grep`. Connectors extract it **deterministically** (re-running is a no-op when nothing changed), your agent adds meaning on top, and the catalog round-trips back to the source. No proprietary store, no lock-in — just files.

The rest of this document is reference: repository layout, each skill in detail, testing, and releases. Most skills are self-contained Go binaries; the agent-guidance skills (`okf-enrich`, `okf-reader`, `okf-producer-generator`) are instructions-only (`SKILL.md`, no binary).

---

## Repository Structure

```
okf-skills/
├── AGENTS.md                      # Developer agent guide
├── README.md                      # This documentation
├── LICENSE                        # Apache License 2.0
├── go.work                        # Go workspace mapping all sub-modules
├── Makefile                       # Build, test, install shortcuts
├── install.sh                     # Build and install all skills to a directory
├── skills.sh.json                 # skills.sh registry manifest (groups skills for discovery)
├── okf-go/                        # Shared Go library (OKF spec, YAML/MD serialization)
│   ├── okf.go                     # Core types & helpers (Frontmatter, ConceptDoc)
│   ├── okf_test.go                # Unit tests
│   └── okf-SPEC.md               # OKF specification document
├── okf-mcp/                      # Generic MCP server — the host that exposes skills (not a skill itself)
├── skills/
│   ├── okf-sqlite/                # SQLite connector (CGO-free, modernc.org/sqlite)
│   ├── okf-mysql/                 # MySQL schema & comment connector
│   ├── okf-postgresql/            # PostgreSQL schema & comment connector
│   ├── okf-bigquery/              # Google Cloud BigQuery metadata connector
│   ├── okf-fs/                    # Local filesystem connector
│   ├── okf-git/                   # Git repository connector
│   ├── okf-enrich/                # Enrichment guidance skill (Instructions-only)
│   ├── okf-reader/                # Ingestion guidance skill (Instructions-only)
│   ├── okf-producer-generator/    # Producer-authoring guidance skill (Instructions-only)
│   └── okf-viz/                   # Bundle visualizer — renders OKF bundles to interactive HTML
└── tests/                         # Integration test suite
    ├── docker-compose.yml         # MySQL & PostgreSQL containers
    ├── helpers_test.go            # Shared test utilities
    ├── db_integration_test.go     # SQLite, MySQL, PostgreSQL integration tests
    ├── fs_integration_test.go     # Filesystem integration tests
    ├── git_integration_test.go    # Git integration tests
    ├── mysql/
    │   └── init_mysql.sql         # Sample MySQL schema with comments
    ├── postgres/
    │   └── init_postgres.sql      # Sample PostgreSQL schema with comments
    └── testdata/                   # Test fixtures & sample data
```

---

## 1. Standalone Skills (`skills/`)

Each folder under `skills/` is a self-contained Go module containing:
- `SKILL.md`: Instructs any coding agent (like Claude Code, Cursor, Copilot) how to execute the connector.
- Go source code (`main.go` and `go.mod`): Compiles into a single portable binary.

Every skill is self-describing via a `schema` subcommand that emits JSON describing its commands, flags, and parameters.

### Available Connectors

| Skill | Data Source | Key Feature |
|---|---|---|
| `okf-sqlite` | SQLite databases | CGO-free via `modernc.org/sqlite` |
| `okf-mysql` | MySQL databases | DDL-based comment sync |
| `okf-postgresql` | PostgreSQL databases | `COMMENT ON` based sync |
| `okf-bigquery` | Google Cloud BigQuery | GCP credentials / API key |
| `okf-fs` | Local filesystem | `.okfignore` & `.okf-metadata.yaml` support |
| `okf-git` | Git repositories | Commit history & file-level metadata |
| `okf-csv` | CSV files | Inferred column types + data profile, `.okf-metadata.yaml` sync |
| `okf-openapi` | OpenAPI / Swagger specs | Endpoint + Schema concepts with typed cross-links |

### Commands

All connectors support three subcommands:
- **`produce`**: Extracts metadata from the data source and generates an OKF bundle. The four SQL connectors (`okf-sqlite`, `okf-mysql`, `okf-postgresql`, `okf-bigquery`) also support `--sample` and `--profile` flags on `produce`.
- **`ingest`**: Reads an OKF bundle and compares/synchronizes descriptions back to the source.
- **`schema`**: Emits a JSON description of the skill's commands, flags, and parameters.

### How to Install or Build a Skill

Each skill is a published Go module, so the simplest install needs no clone (Go 1.24+):
```bash
go install github.com/xSAVIKx/okf-skills/skills/okf-sqlite@latest   # → $(go env GOPATH)/bin/okf-sqlite
```
Or build from a clone — navigate to the skill directory and run:
```bash
go build ./...
```

---

## 2. Enrichment Guidance Skill (`okf-enrich`)

Located in `skills/okf-enrich/`, this is an instructions-only skill (`SKILL.md`) — no binary, no embedded model. It teaches the agent's *own* LLM how to enrich an OKF bundle: find concepts with weak or missing descriptions, ground new descriptions in the schema plus the `Data Profile`/`Sample` sections, write them back into the concept frontmatter, and (optionally) push them to the source with the matching connector's `ingest --sync`.

Enrichment is a judgment task, so it lives as guidance for whatever LLM is already in the loop rather than as a tool that embeds a second one.

---

## 3. MCP Server (`okf-mcp`)

`okf-mcp` is a generic MCP (Model Context Protocol) server that discovers all installed `okf-*` skill binaries and exposes their commands as MCP tools to any MCP-capable harness (Claude Code, Gemini CLI, etc.).

No bespoke agent is required. Point any MCP-capable harness at `okf-mcp`:

```bash
okf-mcp                                    # discovers skills on PATH
okf-mcp --skills-dir /path/to/skills       # explicit skills directory
```

Once registered as an MCP server, every connector command (`produce`/`ingest`) appears as a callable tool. The guidance skills (`okf-enrich`, `okf-reader`) are not tools — they are `SKILL.md` instructions an agent loads directly.

---

## 4. Ingestion Guidance Skill (`okf-reader`)

Located in `skills/okf-reader/`, this is an instructions-only skill (`SKILL.md`). It teaches AI agents how to read and navigate OKF bundles efficiently, minimizing context token overhead and preventing slow, recursive directory reads.

---

## 5. Shared Library (`okf-go/`)

The `okf-go` module provides shared Go types and helpers used by all skills:
- `Frontmatter` / `ConceptDoc` structs for OKF YAML+Markdown serialization
- `ReadConceptDoc` / `WriteConceptDoc` for parsing and writing OKF concept files
- `UpsertSection` / `GetSection` for adding or replacing markdown body sections (e.g. `Data Profile`, `Sample`) without clobbering surrounding content
- `ColumnProfile` with `RenderProfileSection` / `RenderSampleSection` for the `--profile` / `--sample` output
- `SkillSchema` / `PrintSchema` for the self-describing `schema` subcommand every skill emits
- `IgnoreMatcher` for `.okfignore` wildcard support
- `ReadFolderMetadata` / `WriteFolderMetadata` for `.okf-metadata.yaml`

All skills import this module at its published version (`github.com/xSAVIKx/okf-skills/okf-go v0.1.0`). Local development resolves it from disk via the `go.work` workspace, so edits to `okf-go` are picked up without republishing — no per-module `replace` directive is needed.

---

## 6. Installing Skills

Install any single skill (or the `okf-mcp` server) straight from the published module — no clone required (Go 1.24+):

```bash
go install github.com/xSAVIKx/okf-skills/skills/okf-sqlite@latest
go install github.com/xSAVIKx/okf-skills/okf-mcp@latest
```

The binary lands in `$(go env GOPATH)/bin` (ensure it is on your `PATH`).

To build and install **all** skills at once from a clone, use `install.sh` (or `make install`):

```bash
# Install to $HOME/.local/bin (default)
./install.sh

# Install to a custom directory
./install.sh /usr/local/bin

# Or via make
make install
```

After installation, ensure the directory is on your `PATH`. Then either invoke skills directly (`okf-sqlite produce ...`) or run `okf-mcp` to expose all skills as MCP tools to your coding agent.

---

## 7. Local Testing Environment

Integration tests live in `tests/` and cover all connectors, the connectors' `schema` self-description contract, and `okf-mcp`'s discovery of a built skill. A `docker-compose.yml` is provided to spin up MySQL and PostgreSQL instances with pre-loaded mock databases:

```bash
# 1. Build the skill binaries IN PLACE (required by the integration tests, which
#    locate them at skills/<name>/<name>). With GNU make (Linux/macOS/CI):
make build
#    Without make (e.g. Git Bash on Windows), build each connector and okf-mcp:
#    for each: cd skills/okf-sqlite && go build -o okf-sqlite .   (and cd okf-mcp && go build -o okf-mcp .)

# 2. (Optional) start MySQL & PostgreSQL for the database integration tests:
cd tests && docker-compose up -d && cd ..

# 3. Run the integration suite:
cd tests && go test -v .
```

SQLite, filesystem, git, the `schema`-contract checks, and `okf-mcp` discovery run without Docker; only the MySQL and PostgreSQL cases require the containers (they skip otherwise).

---

## 8. Spec Compliance & Registry Discovery

Every `SKILL.md` follows the [Agent Skills specification](https://agentskills.io/specification): the YAML frontmatter exposes only the spec's top-level keys (`name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools`), with project-specific fields (`version`, `author`, `tags`) nested under `metadata`. Each `name` matches its skill directory, and every skill declares `license: Apache-2.0` (the repository is Apache-2.0 licensed). Validate with the reference tool: `skills-ref validate skills/okf-sqlite`.

The root `skills.sh.json` manifest groups the skills for the [skills.sh](https://www.skills.sh) registry so they are organized when the repository is indexed:

| Group | Skills |
|---|---|
| Database Connectors | `okf-sqlite`, `okf-mysql`, `okf-postgresql`, `okf-bigquery`, `okf-csv` |
| Filesystem & Git | `okf-fs`, `okf-git` |
| API Connectors | `okf-openapi` |
| Agent Guidance | `okf-reader`, `okf-enrich`, `okf-producer-generator` |
| Visualization | `okf-viz` |
| Validation | `okf-lint` |

`okf-mcp` is deliberately omitted from the registry manifest: it is the host server that exposes the skills over MCP (and lives outside `skills/`), not a discoverable skill itself.

---

## 9. Producer Generator Skill (`okf-producer-generator`)

Located in `skills/okf-producer-generator/`, this is an instructions-only skill (`SKILL.md`) — no binary. It is the "write a producer" on-ramp for the project: it ships a snapshot of the [official OKF spec](https://github.com/GoogleCloudPlatform/knowledge-catalog/tree/main/okf) (`okf-SPEC.md`), an `okf-go` library API reference (`okf-go-api.md`), and a step-by-step guide for authoring a new connector that matches the existing six rather than reverse-engineering them.

It covers the architectural principles (deterministic extraction with **no embedded LLM**, `okf-go` as the single source of OKF types, `schema` as the MCP-discovery contract), the `produce`/`ingest`/`schema` command surface, the secret-handling and `--sync` conventions, and the full registration checklist (`go.work`, `Makefile`, `install.sh`, `skills.sh.json`, docs, and tests).

Load it when extending the project to a source it doesn't yet cover — e.g. MongoDB, Redis, Kafka, a CSV directory, or an HTTP API.

---

## 10. Visualization Skill (`okf-viz`)

Located in `skills/okf-viz/`, this is a Go CLI consumer skill that renders any OKF bundle into a single self-contained interactive `index.html` written next to `index.md`. It produces a three-pane explorer: a navigator (tree + type/tag filters + full-text search), a Cytoscape graph with seven switchable layouts and a collapsible edge-kind legend, and a rendered concept reader.

```bash
# Build
cd skills/okf-viz && go build -o okf-viz .

# Render a bundle (CDN mode — requires internet for the graph library)
./okf-viz render --bundle path/to/bundle

# Render fully offline — all graph JS inlined, no network needed
./okf-viz render --bundle path/to/bundle --offline

# Additional options
./okf-viz render --bundle path/to/bundle --theme dark --title "My Catalog"
```

The `render` command writes `<bundle>/index.html` by default (override with `--out`). The output is a single portable HTML file with no server required — open it in any browser. The graph shows containment edges (dashed, grey) for the directory hierarchy and solid cross-link edges (colored) for explicit `[text](../concept.md)` references between concepts.

---

## Continuous Integration & Releases

CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) runs on every PR:
gofmt, `go vet`, build of every module, unit tests, and the Docker-backed
integration suite (SQLite/FS/Git always; MySQL/PostgreSQL via service containers).

Releases are automated from [Conventional Commits](https://www.conventionalcommits.org/)
with [release-please](https://github.com/googleapis/release-please): merging work
to `master` maintains a release PR, and merging that PR tags each changed module
in Go's `<path>/vX.Y.Z` form, cuts a GitHub Release, and warms the module proxy.
See **[RELEASING.md](RELEASING.md)** for the full flow and one-time repo settings.

## License

Licensed under the **Apache License 2.0** — see [`LICENSE`](LICENSE) for the full text. Copyright 2026 Yurii Serhiichuk.
