---
name: okf-sqlite
description: SQLite connector that produces and ingests Open Knowledge Format (OKF) bundles from database schemas and comments, CGO-free via modernc.org/sqlite. Use when documenting or cataloging a SQLite database, extracting its schema into OKF, or comparing/validating a database against an OKF bundle.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary. No CGO or system SQLite library needed.
metadata:
  version: "0.1.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, sqlite, database, schema"
---

# SQLite OKF Connector

This skill provides a Go-based CLI tool to convert SQLite database schemas and comments into Open Knowledge Format (OKF) bundles, and validate existing databases against OKF bundles.

## When to Use

Use this skill when you need to:
1. Extract metadata, table structures, column definitions, and primary keys from a local SQLite database into a portable OKF bundle.
2. Verify that an existing SQLite database conforms to an OKF bundle definition.

## Setup

The connector is written in Go and requires Go 1.24+ to build from source:

```bash
cd skills/okf-sqlite
go build -o okf-sqlite .
```

## How to Use

### 1. Produce an OKF Bundle
Extract metadata from a SQLite database file and save it as an OKF bundle directory:

```bash
./okf-sqlite produce --db <path-to-sqlite-db-file> --out <output-bundle-dir> [--tables <comma-separated-table-names>] [--sample <N>] [--profile]
```

**Parameters:**
- `--db` (required): Path to the SQLite `.db` or `.sqlite` file.
- `--out` (required): Path to the directory where the OKF bundle will be generated.
- `--tables` (optional): Filter to extract only specific tables.
- `--sample <N>` (optional): Embed up to N sample rows per table as a `## Sample` section in each table doc (default 0 = none).
- `--profile` (optional): Compute per-column statistics (non-null, null, distinct, min, max) and embed a `## Data Profile` section.

### 2. Ingest / Verify an OKF Bundle
Verify an existing SQLite database's schema against an OKF bundle, or bootstrap missing tables:

```bash
./okf-sqlite ingest --db <path-to-sqlite-db-file> --bundle <path-to-okf-bundle> [--sync]
```

**Parameters:**
- `--db` (required): Path to the SQLite database.
- `--bundle` (required): Path to the OKF bundle.
- `--sync` (optional): If provided, the tool will attempt to create any missing tables or columns defined in the OKF bundle.

### 3. Inspect the Schema (self-description)
Print a machine-readable JSON description of this skill's commands and flags (used by `okf-mcp` to expose the skill as an MCP tool):

```bash
./okf-sqlite schema
```
