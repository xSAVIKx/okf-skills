# Open Knowledge Format (OKF) Skills — Developer Agent Guide

Welcome, Agent! This guide contains crucial context, structural guidelines, and best practices for working in the `okf-skills` repository. Follow these principles to maintain high-quality, spec-compliant, and portable implementations.

---

## 1. Repository Overview & Architecture

This repository is a central collection of skills for producing, consuming, and authoring Open Knowledge Format (OKF) bundles — standalone CLI connectors, instructions-only guidance skills (read, enrich, and author), and a generic MCP server. It is organized as a Go workspace containing multiple modules:

```
okf-skills/
├── AGENTS.md                      # This guide
├── README.md                      # General user-facing overview
├── LICENSE                        # Apache License 2.0
├── go.work                        # Go workspace defining monorepo modules
├── Makefile                       # Build, test, install shortcuts
├── install.sh                      # Build and install all skills to a directory
├── skills.sh.json                 # skills.sh registry manifest (groups skills for discovery)
├── okf-go/                        # Shared Go library (YAML/MD serialization, ignore/metadata helpers)
│   ├── okf.go                     # Core types: Frontmatter, ConceptDoc, helpers
│   ├── okf_test.go                # Unit tests
│   └── okf-SPEC.md               # Full OKF specification document
├── okf-mcp/                       # Generic MCP server — the host that exposes skills (not a skill)
├── skills/                        # Standalone Go-based CLI skills
│   ├── okf-sqlite/                # SQLite connector (CGO-free)
│   ├── okf-csv/                # CSV File connector
│   ├── okf-mysql/                 # MySQL connector
│   ├── okf-postgresql/            # PostgreSQL connector
│   ├── okf-bigquery/              # GCP BigQuery connector
│   ├── okf-fs/                    # Local filesystem connector
│   ├── okf-git/                   # Git repository connector
│   ├── okf-enrich/                # Enrichment guidance skill (Instructions-only)
│   ├── okf-reader/                # Ingestion guidance skill (Instructions-only)
│   ├── okf-producer-generator/    # Producer-authoring guidance skill (Instructions-only)
│   ├── okf-viz/                   # Bundle visualizer — renders OKF bundles to interactive HTML
│   └── okf-lint/                  # Bundle validator — spec conformance + coverage, CI gate
└── tests/                         # Central integration testing directory
    ├── docker-compose.yml         # MySQL & PostgreSQL containers
    ├── helpers_test.go            # Shared test utilities (getBinaryPath, isPortOpen, etc.)
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
- **Do Not Duplicate Structs**: The `Frontmatter` and `ConceptDoc` structs must not be defined in individual skills. Import `github.com/xSAVIKx/okf-skills/okf-go` instead.
- **Spec Compliance**: OKF concepts are Markdown files with YAML frontmatter.
  - Subdirectory `index.md` files must contain **no frontmatter**.
  - The bundle-root `index.md` is the only index permitted to contain frontmatter, and it should only declare `okf_version: "0.1"` (omit `type`, `title`, and `description` from the YAML block; place them directly inside the Markdown body).
- **Line Ending Compatibility**: `ReadConceptDoc` split operations must handle both Unix LF (`\n`) and Windows CRLF (`\r\n`) markers for frontmatter boundaries.
- **Ignore & Metadata Helpers**: Use the shared `IgnoreMatcher` helper to load `.okfignore` wildcard matchers, and `ReadFolderMetadata`/`WriteFolderMetadata` to serialize/deserialize path-to-description mapping inside `.okf-metadata.yaml`.
- **Unit Testing**: Maintain robust tests in `okf_test.go` and run `go test -v ./...` inside `okf-go/` after making changes.

---

## 3. Skills Development & Best Practices

The connector skills compile to standalone Go CLI binaries, each exposing three subcommands (the `okf-viz` visualizer is a consumer binary exposing `render`/`schema`; the guidance skills are instructions-only, no binary):
1. `produce`: Extract database schema comments, local filesystem folder structures, or git repository commit history into an OKF bundle. The four SQL connectors (`okf-sqlite`, `okf-mysql`, `okf-postgresql`, `okf-bigquery`) also support `--sample` and `--profile` flags.
2. `ingest`: Read an OKF bundle, validate assets, and optionally synchronize comments/descriptions back to the source database or `.okf-metadata.yaml` using the `-sync` flag.
3. `schema`: Emit a JSON description of the skill's commands, flags, and parameters (used by `okf-mcp` for tool discovery).

> **Authoring a new connector?** The `okf-producer-generator` skill (`skills/okf-producer-generator/`) codifies this entire section — the architectural principles, the `okf-go` contract, the `produce`/`ingest`/`schema` surface, secret handling, the three `--sync` patterns, and the registration checklist — into a step-by-step guide. Load it first.

### Best Practices for Skills:
- **Portability**: Write skills in pure Go with zero runtime dependencies. To guarantee CGO-free compilation for SQLite, use `modernc.org/sqlite` instead of `github.com/mattn/go-sqlite3`.
- **Shared-library imports**: Give each skill a full, publishable module path (`module github.com/xSAVIKx/okf-skills/skills/okf-<name>`) and require the shared library at its published version (`require github.com/xSAVIKx/okf-skills/okf-go v0.1.0`). Do **not** add a per-module `replace` directive — the root `go.work` already maps `okf-go` to the on-disk copy for local development, so edits are picked up without republishing, and the clean `go.mod` lets the skill be `go install`ed standalone.
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
- **Documentation**: Keep each skill's `SKILL.md` detailed and descriptive so that MCP consumers and coding agents know what options the CLI supports.
- **`SKILL.md` Frontmatter Spec**: Every `SKILL.md` must conform to the [Agent Skills specification](https://agentskills.io/specification). Only `name`, `description`, `license`, `compatibility`, `metadata`, and `allowed-tools` are permitted as top-level YAML keys, and `name` must equal the skill's directory name. Put project-specific fields (`version`, `author`, `tags`, …) under `metadata:` as string values — **never at the top level** (a top-level `version:`/`tags:` fails `skills-ref validate`). Write `description` as "what it does + when to use it" with searchable keywords so coding agents and the skills.sh registry surface it correctly. Set `license: Apache-2.0` on every skill to match the repository license (see `LICENSE`).
- **Registry Discovery**: Skills under `skills/` are grouped for the [skills.sh](https://www.skills.sh) registry via the root `skills.sh.json`. When adding or removing a skill, update its `groupings` array. `okf-mcp` is intentionally excluded — it is the host server (lives outside `skills/`), not a discoverable registry skill.

---

## 4. MCP Server (`okf-mcp`)

`okf-mcp` is a generic MCP (Model Context Protocol) server. It discovers all installed `okf-*` binaries, calls their `schema` subcommand, and registers each command as an MCP tool. Any MCP-capable harness (Claude Code, Gemini CLI, etc.) can then invoke skills without a bespoke agent.

### Running `okf-mcp`

First install all skill binaries:
```bash
# Install to $HOME/.local/bin (default)
./install.sh

# Or to a custom directory
./install.sh /path/to/dir
```

Then register `okf-mcp` as an MCP server in your harness configuration. For Claude Code (`~/.claude/settings.json`):
```json
{
  "mcpServers": {
    "okf": {
      "command": "okf-mcp",
      "args": []
    }
  }
}
```

Or pass an explicit skills directory:
```json
{
  "mcpServers": {
    "okf": {
      "command": "okf-mcp",
      "args": ["--skills-dir", "/path/to/skills"]
    }
  }
}
```

Once registered, every connector command (`produce`/`ingest`) appears as a callable MCP tool. The guidance skills (`okf-enrich`, `okf-reader`) are loaded as `SKILL.md` instructions, not exposed as tools.

### Adding a New Skill to `okf-mcp`

When you add a new skill under `skills/okf-<name>/`, the only requirement for it to appear as an MCP tool is that it:
1. Compiles to a binary named `okf-<name>`.
2. Implements the `schema` subcommand outputting a JSON descriptor.

`okf-mcp` discovers and registers it automatically — no changes to `okf-mcp` itself are needed.

---

## 5. Integration Testing

Integration tests are centralized under `tests/` and organized by connector type:

| File | Coverage |
|---|---|
| `helpers_test.go` | `getBinaryPath()`, `isPortOpen()` shared utilities |
| `db_integration_test.go` | SQLite (no Docker), MySQL (Docker), PostgreSQL (Docker) |
| `fs_integration_test.go` | Filesystem produce & ingest |
| `git_integration_test.go` | Git repository produce & ingest |
| `mcp_integration_test.go` | Connector `schema`-contract checks + `okf-mcp` discovery of a built skill |

### Running Tests
```bash
# 1. Build skill binaries IN PLACE (the tests invoke them as subprocesses and
#    locate them at skills/<name>/<name>). From the repo root, with GNU make:
make build
#    Without make: build each connector and okf-mcp with `go build -o <name> .`.

# 2. (Optional) start MySQL & PostgreSQL for the database tests:
cd tests && docker-compose up -d && cd ..

# 3. Run the suite:
cd tests && go test -v .

# 4. (Optional) stop the databases:
cd tests && docker-compose down && cd ..
```

SQLite, filesystem, git, the `schema`-contract checks, and `okf-mcp` discovery run without Docker; the MySQL/PostgreSQL cases are guarded by connection checks and skip when the containers are down.

---

## 6. Ongoing Development Workflow

When adding a new connector or modifying an existing one, follow these steps (the `okf-producer-generator` skill in `skills/okf-producer-generator/` walks an agent through this end to end — principles, code patterns, and the full registration checklist):
1. **Initialize Module**: Create `skills/okf-<name>/go.mod` and add it to `go.work` at the root.
2. **Update Workspace Dependencies**: Run `go mod tidy` in the new skill directory, ensuring it links to `okf-go` locally.
3. **Implement `schema` Subcommand**: Implement the `schema` subcommand so `okf-mcp` can discover and register the new skill as an MCP tool automatically.
4. **Local Testing**: Run unit tests in the skill directory. For database connectors, start Docker databases (`cd tests && docker-compose up -d`). Run all integration tests under `tests/` using `go test -v .` to verify correctness.
5. **Compile Binaries**: Run `make build` (or `go build -o <name> .` in each skill directory and `okf-mcp/`) and verify everything compiles without errors.
6. **Code Clean-up**: Shut down database containers via `cd tests && docker-compose down`.
7. **Commit Conventions**: Use conventional commit messages (`feat: ...`, `fix: ...`, `refactor: ...`, `docs: ...`) and commit modularly.
