# Open Knowledge Format (OKF) Skills Registry

This repository is a central registry of standalone CLI skills for producing and ingesting Open Knowledge Format (OKF) bundles. OKF is a simple, human- and agent-friendly specification for documenting data assets (schemas, comments, constraints, and metrics) as a directory of Markdown files with YAML frontmatter.

Each skill is a self-contained Go module that compiles to a single portable binary.

## Registry Structure

```
okf-skills-registry/
├── AGENTS.md                      # Developer agent guide
├── README.md                      # This documentation
├── go.work                        # Go workspace mapping all sub-modules
├── Makefile                       # Build, test, install shortcuts
├── skills.sh                      # Build and install all skills to a directory
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
│   ├── okf-enrich/                # LLM enrichment skill
│   └── okf-reader/                # Ingestion guidance skill (Instructions-only)
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

## 2. LLM Enrichment Skill (`okf-enrich`)

`okf-enrich` uses an LLM to add or improve descriptions in an OKF bundle. Subcommands:
- **`enrich`**: Reads an OKF bundle and fills in missing or incomplete descriptions using an LLM.
- **`schema`**: Emits the skill's JSON schema.

---

## 3. MCP Server (`okf-mcp`)

`okf-mcp` is a generic MCP (Model Context Protocol) server that discovers all installed `okf-*` skill binaries and exposes their commands as MCP tools to any MCP-capable harness (Claude Code, Gemini CLI, etc.).

No bespoke agent is required. Point any MCP-capable harness at `okf-mcp`:

```bash
okf-mcp                                    # discovers skills on PATH
okf-mcp --skills-dir /path/to/skills       # explicit skills directory
```

Once registered as an MCP server, every connector and enrichment command appears as a callable tool.

---

## 4. Ingestion Guidance Skill (`okf-reader`)

Located in `skills/okf-reader/`, this is an instructions-only skill (`SKILL.md`). It teaches AI agents how to read and navigate OKF bundles efficiently, minimizing context token overhead and preventing slow, recursive directory reads.

---

## 5. Shared Library (`okf-go/`)

The `okf-go` module provides shared Go types and helpers used by all skills:
- `Frontmatter` / `ConceptDoc` structs for OKF YAML+Markdown serialization
- `ReadConceptDoc` / `WriteConceptDoc` for parsing and writing OKF concept files
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

Integration tests live in `tests/` and cover all connectors. A `docker-compose.yml` is provided to spin up MySQL and PostgreSQL instances with pre-loaded mock databases:

```bash
# Start test databases
cd tests
docker-compose up -d

# Build all skill binaries (required by integration tests)
make build

# Run the full integration test suite
cd tests
go test -v .
```

SQLite, filesystem, and git tests run without Docker. Only MySQL and PostgreSQL tests require the containers.
