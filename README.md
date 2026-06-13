# Open Knowledge Format (OKF) Skills Registry

This repository is a central registry of standalone CLI skills for producing and ingesting Open Knowledge Format (OKF) bundles. OKF is a simple, human- and agent-friendly specification for documenting data assets (schemas, comments, constraints, and metrics) as a directory of Markdown files with YAML frontmatter.

Each skill is a self-contained Go module that compiles to a single portable binary.

## Registry Structure

```
okf-skills-registry/
├── AGENTS.md                      # Developer agent guide
├── README.md                      # This documentation
├── LICENSE                        # Apache License 2.0
├── go.work                        # Go workspace mapping all sub-modules
├── Makefile                       # Build, test, install shortcuts
├── skills.sh                      # Build and install all skills to a directory
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
│   └── okf-producer-generator/    # Producer-authoring guidance skill (Instructions-only)
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

### Commands

All connectors support three subcommands:
- **`produce`**: Extracts metadata from the data source and generates an OKF bundle. The four SQL connectors (`okf-sqlite`, `okf-mysql`, `okf-postgresql`, `okf-bigquery`) also support `--sample` and `--profile` flags on `produce`.
- **`ingest`**: Reads an OKF bundle and compares/synchronizes descriptions back to the source.
- **`schema`**: Emits a JSON description of the skill's commands, flags, and parameters.

### How to Build a Skill

Navigate to the skill directory and run:
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

All skills import this module via a local `replace` directive in their `go.mod`.

---

## 6. Installing Skills

Use `skills.sh` (or `make install`) to build all skills and install them into a directory:

```bash
# Install to $HOME/.local/bin (default)
./skills.sh

# Install to a custom directory
./skills.sh /usr/local/bin

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
| Database Connectors | `okf-sqlite`, `okf-mysql`, `okf-postgresql`, `okf-bigquery` |
| Filesystem & Git | `okf-fs`, `okf-git` |
| Agent Guidance | `okf-reader`, `okf-enrich`, `okf-producer-generator` |

`okf-mcp` is deliberately omitted from the registry manifest: it is the host server that exposes the skills over MCP (and lives outside `skills/`), not a discoverable skill itself.

---

## 9. Producer Generator Skill (`okf-producer-generator`)

Located in `skills/okf-producer-generator/`, this is an instructions-only skill (`SKILL.md`) — no binary. It is the "write a producer" on-ramp for the registry: it ships a copy of the OKF spec (`okf-SPEC.md`), an `okf-go` library API reference (`okf-go-api.md`), and a step-by-step guide for authoring a new connector that matches the existing six rather than reverse-engineering them.

It covers the architectural principles (deterministic extraction with **no embedded LLM**, `okf-go` as the single source of OKF types, `schema` as the MCP-discovery contract), the `produce`/`ingest`/`schema` command surface, the secret-handling and `--sync` conventions, and the full registration checklist (`go.work`, `Makefile`, `skills.sh`, `skills.sh.json`, docs, and tests).

Load it when extending the registry to a source it doesn't yet cover — e.g. MongoDB, Redis, Kafka, a CSV directory, or an HTTP API.

---

## License

Licensed under the **Apache License 2.0** — see [`LICENSE`](LICENSE) for the full text. Copyright 2026 Yurii Serhiichuk.
