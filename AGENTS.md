# Open Knowledge Format (OKF) Skills Registry — Developer Agent Guide

Welcome, Agent! This guide contains crucial context, structural guidelines, and best practices for working in the `okf-skills-registry` repository. Follow these principles to maintain high-quality, spec-compliant, and portable implementations.

---

## 1. Repository Overview & Architecture

This repository acts as a central registry of skills for producing and ingesting Open Knowledge Format (OKF) bundles. It is organized as a Go workspace containing multiple modules:

```
okf-skills-registry/
├── AGENTS.md                      # This guide
├── README.md                      # General user-facing overview
├── go.work                        # Go workspace defining monorepo modules
├── okf-go/                        # Shared Go library (YAML/MD serialization, ignore/metadata helpers)
│   ├── okf.go                     # Core types: Frontmatter, ConceptDoc, helpers
│   ├── okf_test.go                # Unit tests
│   └── okf-SPEC.md               # Full OKF specification document
├── skills/                        # Standalone Go-based CLI skills
│   ├── okf-sqlite/                # SQLite connector (CGO-free)
│   ├── okf-mysql/                 # MySQL connector
│   ├── okf-postgresql/            # PostgreSQL connector
│   ├── okf-bigquery/              # GCP BigQuery connector
│   ├── okf-fs/                    # Local filesystem connector
│   ├── okf-git/                   # Git repository connector
│   └── okf-reader/                # Ingestion guidance skill (Instructions-only)
├── agent/                         # OKF Agent wrapping skills as tools
│   ├── agents-cli-manifest.yaml   # Google Agents CLI manifest
│   ├── Makefile                   # Agent build, run, clean, skills-build
│   └── app/                       # Go ADK agent source code
│       ├── main.go                # Entrypoint, CLI chat loop, session management
│       ├── agent.go               # LLM agent config, prompts, tool registration
│       └── tools.go               # Subprocess runner wrappers for each skill
└── tests/                         # Central integration testing directory
    ├── docker-compose.yml         # MySQL & PostgreSQL containers
    ├── helpers_test.go            # Shared test utilities (buildSkill, runSkill, etc.)
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

## 2. Shared Library (`okf-go`) Guidelines

All core OKF schemas and parsing helper functions live under `okf-go/`.
- **Do Not Duplicate Structs**: The `Frontmatter` and `ConceptDoc` structs must not be defined in individual skills or the agent. Import `github.com/savikne/okf-skills-registry/okf-go` instead.
- **Spec Compliance**: OKF concepts are Markdown files with YAML frontmatter.
  - Subdirectory `index.md` files must contain **no frontmatter**.
  - The bundle-root `index.md` is the only index permitted to contain frontmatter, and it should only declare `okf_version: "0.1"` (omit `type`, `title`, and `description` from the YAML block; place them directly inside the Markdown body).
- **Line Ending Compatibility**: `ReadConceptDoc` split operations must handle both Unix LF (`\n`) and Windows CRLF (`\r\n`) markers for frontmatter boundaries.
- **Ignore & Metadata Helpers**: Use the shared `IgnoreMatcher` helper to load `.okfignore` wildcard matchers, and `ReadFolderMetadata`/`WriteFolderMetadata` to serialize/deserialize path-to-description mapping inside `.okf-metadata.yaml`.
- **Unit Testing**: Maintain robust tests in `okf_test.go` and run `go test -v ./...` inside `okf-go/` after making changes.

---

## 3. Skills Development & Best Practices

Skills compile to standalone Go CLI binaries. They must support two main subcommands:
1. `produce`: Extract database schema comments, local filesystem folder structures, or git repository commit history into an OKF bundle.
2. `ingest`: Read an OKF bundle, validate assets, and optionally synchronize comments/descriptions back to the source database or `.okf-metadata.yaml` using the `-sync` flag.

### Best Practices for Skills:
- **Portability**: Write skills in pure Go with zero runtime dependencies. To guarantee CGO-free compilation for SQLite, use `modernc.org/sqlite` instead of `github.com/mattn/go-sqlite3`.
- **Local Module Imports**: When referencing `okf-go` in a skill's `go.mod`, map it locally via a relative replacement path:
  ```go
  replace github.com/savikne/okf-skills-registry/okf-go => ../../okf-go
  ```
  This ensures compatibility when the repository is cloned for execution in a sandbox environment.
- **Subcommand Flag Parsing**: Always register flags on subcommand FlagSets (e.g. `fs := flag.NewFlagSet("ingest", ...)`) rather than using global flags (`flag.Bool(...)`).
- **MySQL DDL Escaping**: MySQL does not support query placeholders (`?`) in DDL statements (like `ALTER TABLE ... COMMENT`). Escaping single quotes (`'`) and backslashes (`\`) manually using `strings.ReplaceAll` is required before formatting comments directly into DDL strings:
  ```go
  func escapeString(val string) string {
      val = strings.ReplaceAll(val, "\\", "\\\\")
      val = strings.ReplaceAll(val, "'", "''")
      return val
  }
  ```
- **Git Metadata Extraction**: For VCS tracking, query commit logs using `go-git`'s `LogOptions.FileName` targeting relative paths to pull commit message summaries, committer names, and commit dates.
- **Documentation**: Keep each skill's `SKILL.md` detailed and descriptive so that subsequent consumer agents know what options the CLI supports.

---

## 4. OKF Agent Development

The OKF Agent under `agent/` exposes all database, filesystem, and Git connector skills as function tools to LLMs.
- **Go ADK Framework**: Built using Google's Agent Development Kit (`google.golang.org/adk`).
- **Function Tools**: Exposes `sqlite_connector`, `mysql_connector`, `postgresql_connector`, `bigquery_connector`, `fs_connector`, and `git_connector`.
- **Subprocess Execution**: The agent wraps the compiled binaries under `skills/` using `exec.Command`, passing structured JSON parameters received from Gemini tool calls to the underlying executables.
- **Manifest Integration**: The agent is designed to work with `agents-cli`. Ensure any new commands or tool parameters align with [agents-cli-manifest.yaml](file:///d:/AntigravityProjects/okf-skills-registry/agent/agents-cli-manifest.yaml).

### Code Modularization

As the agent grows, keep `agent/app/` structured and modular. **Do not put everything in `main.go`**. Follow this layout:

| File | Responsibility |
|---|---|
| `main.go` | Application entrypoint, CLI scanner/chat loop, session initialization, runtime config |
| `agent.go` | LLM agent build definitions, core prompts, model specifications, tool list registration |
| `tools.go` | Argument/result structs and subprocess runner execution wrappers for each skill |

When adding new capabilities (e.g., web research, enrichment):
- Create a **new file** for each distinct feature area (e.g., `enrichment.go`, `web_tools.go`).
- Keep tool argument/result structs co-located with their runner functions.
- Register new tools in `agent.go`'s `BuildAgent()` function alongside the existing connectors.

---

## 5. Integration Testing

Integration tests are centralized under `tests/` and organized by connector type:

| File | Coverage |
|---|---|
| `helpers_test.go` | `buildSkill()`, `runSkill()`, `readConceptDoc()` shared utilities |
| `db_integration_test.go` | SQLite (no Docker), MySQL (Docker), PostgreSQL (Docker) |
| `fs_integration_test.go` | Filesystem produce & ingest |
| `git_integration_test.go` | Git repository produce & ingest |

### Running Tests
```bash
# Start Docker databases (only needed for MySQL/PostgreSQL tests)
cd tests && docker-compose up -d

# Build all skill binaries (tests invoke them as subprocesses)
cd ../agent && make skills-build

# Run the full suite
cd ../tests && go test -v .

# Stop Docker databases
docker-compose down
```

SQLite, filesystem, and git tests run without Docker. Tests that require Docker containers are guarded with connection checks.

---

## 6. Ongoing Development Workflow

When adding a new connector or modifying an existing one, follow these steps:
1. **Initialize Module**: Create `skills/okf-<name>/go.mod` and add it to `go.work` at the root.
2. **Update Workspace Dependencies**: Run `go mod tidy` in the new skill directory, ensuring it links to `okf-go` locally.
3. **Local Testing**: Run unit tests in the skill directory. For database connectors, start docker databases (`cd tests && docker-compose up -d`). Run all integration test checks under `tests/` using `go test -v .` to verify correctness.
4. **Compile Binaries**: Recompile all binaries. Verify they compile without errors.
5. **Agent Integration**: Expose the new skill as a tool in the OKF Agent — add the runner to `tools.go` and register it in `agent.go`'s `BuildAgent()`.
6. **Code Clean-up**: Shut down database containers via `cd tests && docker-compose down`.
7. **Commit Conventions**: Use conventional commit messages (`feat: ...`, `fix: ...`, `refactor: ...`, `docs: ...`) and commit modularly.
