# Open Knowledge Format (OKF) Skills Registry

This repository acts as a central registry of agent skills for producing and ingesting Open Knowledge Format (OKF) bundles. OKF is a simple, human- and agent-friendly specification for documenting data assets (schemas, comments, constraints, and metrics) as a directory of Markdown files with YAML frontmatter.

Each skill in this repository serves as a connector to a specific data source. 

## Registry Structure

```
okf-skills-registry/
├── README.md                      # This documentation
├── go.work                        # Go workspace mapping all sub-modules
├── skills/
│   ├── okf-sqlite/                # SQLite connector (Go-native, zero setup)
│   ├── okf-mysql/                 # MySQL schema & comment connector (Go)
│   ├── okf-postgresql/            # PostgreSQL schema & comment connector (Go)
│   ├── okf-bigquery/              # Google Cloud BigQuery metadata connector (Go)
│   └── okf-reader/                # Ingestion guidance skill (Instructions-only)
├── agent/
│   ├── agents-cli-manifest.yaml   # Google Agents CLI project descriptor
│   ├── Makefile                   # Agent runner and builder commands
│   └── app/
│       └── main.go                # Reference agent code using Go ADK (google/adk-go)
└── tests/                         # Integration test configurations and fixtures
    ├── docker-compose.yml         # MySQL & PostgreSQL containers
    ├── mysql/
    │   └── init_mysql.sql         # Sample MySQL schema with comments
    └── postgres/
        └── init_postgres.sql      # Sample PostgreSQL schema with comments
```

---

## 1. Standalone Skills (`skills/`)


Each folder under `skills/` is a self-contained module containing:
- `SKILL.md`: Instructs any coding agent (like Claude Code, Cursor, Copilot) how to execute the connector.
- Go source code (`main.go` and `go.mod`): Compiles into a single portable binary.

### How to Build a Skill
Navigate to the skill directory and run:
```bash
go build -o okf-<connector> main.go
```

### Commands:
All database connectors support two subcommands:
- **`produce`**: Reads database schemas and table/column comments and generates an OKF bundle.
- **`ingest`**: Reads an OKF bundle and compares it with the database schema, verifying alignment or synchronizing comments back to database schema documentation.

---

## 2. Ingestion Guidance Skill (`okf-reader`)

Located in `skills/okf-reader/`, this is an instructions-only skill (`SKILL.md`). It teaches AI agents how to read and navigate OKF bundles efficiently, minimizing context token overhead and preventing slow, recursive directory reads.

---

## 3. Reference Agent (`agent/`)

The reference agent under `agent/` is built using the **Google Agent Development Kit (ADK) for Go** (`github.com/google/adk-go`) and is fully compatible with the **Google Agents CLI** (`agents-cli`).

It registers the standalone database skills as function tools and accepts natural language instructions to automatically perform OKF extraction and synchronization.

### How to Run:
Ensure you have set your `GEMINI_API_KEY` (or `GOOGLE_API_KEY`) and run:
```bash
cd agent
make run
```
Or start the Agents CLI playground:
```bash
agents-cli playground
```

---

## 4. Local Testing Environment

A `docker-compose.yml` is provided in the `tests/` directory to spin up MySQL and PostgreSQL instances with pre-loaded mock databases:
```bash
cd tests
docker-compose up -d
```
You can then compile and run `okf-mysql` or `okf-postgresql` against these containers to test extracting and updating database metadata.
