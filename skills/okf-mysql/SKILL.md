---
name: okf-mysql
description: MySQL connector that produces and ingests Open Knowledge Format (OKF) bundles from database schemas and table/column comments. Use when documenting or cataloging a MySQL database, extracting its schema and comments into OKF, or syncing descriptions back to MySQL via DDL.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary, plus network access and credentials for the target MySQL database.
metadata:
  version: "0.8.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, mysql, database, schema, documentation"
---

# MySQL OKF Connector

This skill provides a Go-based CLI tool to convert MySQL database schemas and comments (table and column comments) into Open Knowledge Format (OKF) bundles, and synchronize comments back to a MySQL database from an OKF bundle.

## When to Use

Use this skill when you need to:
1. Extract table structures, column definitions, keys, and native MySQL comments from a schema into a portable OKF bundle.
2. Synchronize business descriptions or documentation changes written in an OKF bundle back into native MySQL table and column comments.

## Setup

The connector requires Go 1.24+:

```bash
# Install the published binary (Go 1.24+) — no clone needed:
go install github.com/xSAVIKx/okf-skills/skills/okf-mysql@v0.1.0

# …or build from a clone of the repository:
cd skills/okf-mysql
go build -o okf-mysql .
```

## How to Use

### 1. Produce an OKF Bundle
Extract database schema metadata and comments:

```bash
./okf-mysql produce --host <host> --port <port> --user <user> --password <password> --db <database-name> --out <output-bundle-dir> [--tables <comma-separated-table-names>] [--sample <N>] [--profile]
```

### 2. Ingest / Synchronize an OKF Bundle
Synchronize table/column descriptions from the OKF bundle back into database comments:

```bash
./okf-mysql ingest --host <host> --port <port> --user <user> --password <password> --db <database-name> --bundle <path-to-okf-bundle> [--sync]
```

**Parameters:**
- `--host` (default `localhost`): MySQL host.
- `--port` (default `3306`): MySQL port.
- `--user` (required): MySQL user.
- `--password` (required): MySQL password. May also be supplied via the `MYSQL_PASSWORD` environment variable to keep secrets out of the command line.
- `--db` (required): Target database schema.
- `--bundle` (required for ingest): Path to the OKF bundle.
- `--out` (required for produce): Path to output OKF bundle.
- `--tables` (optional): Filter to extract only specific tables.
- `--sample <N>` (optional): Embed up to N sample rows per table as a `## Sample` section in each table doc (default 0 = none).
- `--profile` (optional): Compute per-column statistics (non-null, null, distinct, min, max) and embed a `## Data Profile` section.
- `--sync` (optional for ingest): If provided, runs `ALTER TABLE ... COMMENT = ...` and `MODIFY COLUMN ... COMMENT ...` statements to synchronize descriptions.

### 3. Inspect the Schema (self-description)
Print a machine-readable JSON description of this skill's commands and flags (used by `okf-mcp` to expose the skill as an MCP tool):

```bash
./okf-mysql schema
```
