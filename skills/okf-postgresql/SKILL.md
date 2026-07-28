---
name: okf-postgresql
description: PostgreSQL connector that produces and ingests Open Knowledge Format (OKF) bundles from database schemas and table/column descriptions. Use when documenting or cataloging a PostgreSQL database, extracting its schema and comments into OKF, or syncing descriptions back with COMMENT ON statements.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary, plus network access and credentials for the target PostgreSQL database.
metadata:
  version: "0.8.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, postgresql, database, schema, documentation"
---

# PostgreSQL OKF Connector

This skill provides a Go-based CLI tool to convert PostgreSQL database schemas and comments (table and column descriptions) into Open Knowledge Format (OKF) bundles, and synchronize comments back to a PostgreSQL database from an OKF bundle.

## When to Use

Use this skill when you need to:
1. Extract table structures, column definitions, keys, schemas, and native PostgreSQL comments (descriptions) from a schema into a portable OKF bundle.
2. Synchronize business descriptions or documentation changes written in an OKF bundle back into native PostgreSQL table and column comments using native SQL comment mechanisms.

## Setup

The connector requires Go 1.24+:

```bash
# Install the published binary (Go 1.24+) — no clone needed:
go install github.com/xSAVIKx/okf-skills/skills/okf-postgresql@v0.1.0

# …or build from a clone of the repository:
cd skills/okf-postgresql
go build -o okf-postgresql .
```

## How to Use

### 1. Produce an OKF Bundle
Extract database schema metadata and comments:

```bash
./okf-postgresql produce --host <host> --port <port> --user <user> --password <password> --db <database-name> --schema <schema-name> --out <output-bundle-dir> [--tables <comma-separated-table-names>] [--sample <N>] [--profile]
```

### 2. Ingest / Synchronize an OKF Bundle
Synchronize table/column descriptions from the OKF bundle back into database comments:

```bash
./okf-postgresql ingest --host <host> --port <port> --user <user> --password <password> --db <database-name> --schema <schema-name> --bundle <path-to-okf-bundle> [--sync]
```

**Parameters:**
- `--host` (default `localhost`): PostgreSQL host.
- `--port` (default `5432`): PostgreSQL port.
- `--user` (default `postgres`): PostgreSQL user.
- `--password` (required): PostgreSQL password. May also be supplied via the `PGPASSWORD` environment variable to keep secrets out of the command line.
- `--db` (required): Target database name.
- `--schema` (default `public`): Target schema name.
- `--bundle` (required for ingest): Path to the OKF bundle.
- `--out` (required for produce): Path to output OKF bundle.
- `--tables` (optional): Filter to extract only specific tables.
- `--sample <N>` (optional): Embed up to N sample rows per table as a `## Sample` section in each table doc (default 0 = none).
- `--profile` (optional): Compute per-column statistics (non-null, null, distinct, min, max) and embed a `## Data Profile` section.
- `--sync` (optional for ingest): If provided, runs `COMMENT ON TABLE ...` and `COMMENT ON COLUMN ...` statements to synchronize descriptions.

### 3. Inspect the Schema (self-description)
Print a machine-readable JSON description of this skill's commands and flags (used by `okf-mcp` to expose the skill as an MCP tool):

```bash
./okf-postgresql schema
```
