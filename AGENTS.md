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
├── okf-go/                        # Shared Go library (YAML/MD serialization, types)
├── skills/                        # Standalone Go-based CLI skills
│   ├── okf-sqlite/                # SQLite connector (CGO-free)
│   ├── okf-mysql/                 # MySQL connector
│   ├── okf-postgresql/            # PostgreSQL connector
│   ├── okf-bigquery/              # GCP BigQuery connector
│   └── okf-reader/                # Ingestion guidance skill (Instructions-only)
├── agent/                         # Reference agent wrapping skills as tools
│   ├── agents-cli-manifest.yaml   # Google Agents CLI manifest
│   ├── Makefile                   # Agent runner and compilation scripts
│   └── app/                       # Go ADK agent source code
└── tests/                         # Integration test configurations and fixtures
    ├── docker-compose.yml         # MySQL & PostgreSQL containers
    ├── mysql/
    │   └── init_mysql.sql         # Sample MySQL schema with comments
    └── postgres/
        └── init_postgres.sql      # Sample PostgreSQL schema with comments
```

---

## 2. Shared Library (`okf-go`) Guidelines

All core OKF schemas and parsing helper functions live under `okf-go/`.
- **Do Not Duplicate Structs**: The `Frontmatter` and `ConceptDoc` structs must not be defined in individual skills or the agent. Import `github.com/savikne/okf-skills-registry/okf-go` instead.
- **Spec Compliance**: OKF concepts are Markdown files with YAML frontmatter.
  - Subdirectory `index.md` files must contain **no frontmatter**.
  - The bundle-root `index.md` is the only index permitted to contain frontmatter, and it should only declare `okf_version: "0.1"` (omit `type`, `title`, and `description` from the YAML block; place them directly inside the Markdown body).
- **Line Ending Compatibility**: `ReadConceptDoc` split operations must handle both Unix LF (`\n`) and Windows CRLF (`\r\n`) markers for frontmatter boundaries.
- **Unit Testing**: Maintain robust tests in `okf_test.go` and run `go test -v ./...` inside `okf-go/` after making changes.

---

## 3. Skills Development & Best Practices

Skills compile to standalone Go CLI binaries. They must support two main subcommands:
1. `produce`: Extract database schema metadata and comments into an OKF bundle.
2. `ingest`: Read an OKF bundle, diff comments with the live database, and optionally synchronize them using the `-sync` flag.

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
- **Documentation**: Keep each skill's `SKILL.md` detailed and descriptive so that subsequent consumer agents know what options the CLI supports.

---

## 4. Reference Agent Development

The reference agent under `agent/` exposes the database skills as function tools to LLMs.
- **Go ADK Framework**: Built using Google's Agent Development Kit (`google.golang.org/adk`).
- **Subprocess Execution**: The agent wraps the compiled binaries under `skills/` using `exec.Command`, passing structured JSON parameters received from Gemini tool calls to the underlying executables.
- **Manifest Integration**: The agent is designed to work with `agents-cli`. Ensure any new commands or tool parameters align with [agents-cli-manifest.yaml](file:///d:/AntigravityProjects/okf-skills-registry/agent/agents-cli-manifest.yaml).

---

## 5. Ongoing Development Workflow

When adding a new database connector or modifying an existing one, follow these steps:
1. **Initialize Module**: Create `skills/okf-<name>/go.mod` and add it to `go.work` at the root.
2. **Update Workspace Dependencies**: Run `go mod tidy` in the new skill directory, ensuring it links to `okf-go` locally.
3. **Local Testing**: Run `cd tests && docker-compose up -d` to launch test databases. Test the extraction (`produce`) and ingestion/synchronization (`ingest -sync`) against the live container.
4. **Compile Binaries**: Recompile all binaries. Verify they compile without errors.
5. **Agent Integration**: Expose the new skill as a tool in the reference agent `app/main.go` and test it in the Agents CLI playground.
6. **Code Clean-up**: Shut down database containers via `cd tests && docker-compose down`.
7. **Commit Conventions**: Use conventional commit messages (`feat: ...`, `fix: ...`, `refactor: ...`, `docs: ...`) and commit modularly.
