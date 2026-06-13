# Open Knowledge Format (OKF) Skills Registry

This repository acts as a central registry of agent skills for producing and ingesting Open Knowledge Format (OKF) bundles. OKF is a simple, human- and agent-friendly specification for documenting data assets (schemas, comments, constraints, and metrics) as a directory of Markdown files with YAML frontmatter.

Each skill in this repository serves as a connector to a specific data source.

## Registry Structure

```
okf-skills-registry/
├── AGENTS.md                      # Developer agent guide
├── README.md                      # This documentation
├── go.work                        # Go workspace mapping all sub-modules
├── okf-go/                        # Shared Go library (OKF spec, YAML/MD serialization)
│   ├── okf.go                     # Core types & helpers (Frontmatter, ConceptDoc)
│   ├── okf_test.go                # Unit tests
│   └── okf-SPEC.md               # OKF specification document
├── skills/
│   ├── okf-sqlite/                # SQLite connector (CGO-free, modernc.org/sqlite)
│   ├── okf-mysql/                 # MySQL schema & comment connector
│   ├── okf-postgresql/            # PostgreSQL schema & comment connector
│   ├── okf-bigquery/              # Google Cloud BigQuery metadata connector
│   ├── okf-fs/                    # Local filesystem connector
│   ├── okf-git/                   # Git repository connector
│   └── okf-reader/                # Ingestion guidance skill (Instructions-only)
├── agent/                         # OKF Agent — LLM-powered orchestrator
│   ├── agents-cli-manifest.yaml   # Google Agents CLI project descriptor
│   ├── Makefile                   # Agent build, run, and clean scripts
│   └── app/                       # Go ADK agent source code
│       ├── main.go                # Entrypoint, CLI chat loop, session management
│       ├── agent.go               # LLM agent config, prompts, tool registration
│       └── tools.go               # Subprocess runner wrappers for each skill
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

### Available Connectors

| Skill | Data Source | Key Feature |
|---|---|---|
| `okf-sqlite` | SQLite databases | CGO-free via `modernc.org/sqlite` |
| `okf-mysql` | MySQL databases | DDL-based comment sync |
| `okf-postgresql` | PostgreSQL databases | `COMMENT ON` based sync |
| `okf-bigquery` | Google Cloud BigQuery | GCP credentials / API key |
| `okf-fs` | Local filesystem | `.okfignore` & `.okf-metadata.yaml` support |
| `okf-git` | Git repositories | Commit history & file-level metadata |

### How to Build a Skill
Navigate to the skill directory and run:
```bash
go build -o okf-<connector> main.go
```

### Commands
All connectors support two subcommands:
- **`produce`**: Extracts metadata from the data source and generates an OKF bundle.
- **`ingest`**: Reads an OKF bundle and compares/synchronizes descriptions back to the source.

---

## 2. Ingestion Guidance Skill (`okf-reader`)

Located in `skills/okf-reader/`, this is an instructions-only skill (`SKILL.md`). It teaches AI agents how to read and navigate OKF bundles efficiently, minimizing context token overhead and preventing slow, recursive directory reads.

---

## 3. Shared Library (`okf-go/`)

The `okf-go` module provides shared Go types and helpers used by all skills:
- `Frontmatter` / `ConceptDoc` structs for OKF YAML+Markdown serialization
- `ReadConceptDoc` / `WriteConceptDoc` for parsing and writing OKF concept files
- `IgnoreMatcher` for `.okfignore` wildcard support
- `ReadFolderMetadata` / `WriteFolderMetadata` for `.okf-metadata.yaml`

All skills import this module via a local `replace` directive in their `go.mod`.

---

## 4. OKF Agent (`agent/`)

The OKF Agent under `agent/` is built using the **Google Agent Development Kit (ADK) for Go** (`google.golang.org/adk`) and is fully compatible with the **Google Agents CLI** (`agents-cli`).

It registers all connector skills as function tools (`sqlite_connector`, `mysql_connector`, `postgresql_connector`, `bigquery_connector`, `fs_connector`, `git_connector`) and accepts natural language instructions to automatically perform OKF extraction and synchronization.

### How to Run
Ensure you have set your `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) and run:
```bash
cd agent
make run
```
Or build the binary and run directly:
```bash
cd agent
make build
./okf-agent.exe
```
Or start the Agents CLI playground:
```bash
agents-cli playground
```

---

## 5. Local Testing Environment

Integration tests live in `tests/` and cover all connectors. A `docker-compose.yml` is provided to spin up MySQL and PostgreSQL instances with pre-loaded mock databases:

```bash
# Start test databases
cd tests
docker-compose up -d

# Build all skill binaries (required by integration tests)
cd ../agent
make skills-build

# Run the full integration test suite
cd ../tests
go test -v .
```

SQLite, filesystem, and git tests run without Docker. Only MySQL and PostgreSQL tests require the containers.
