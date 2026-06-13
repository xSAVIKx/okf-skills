---
name: okf-postgresql
description: PostgreSQL connector for producing and ingesting Open Knowledge Format (OKF) bundles.
version: 0.1.0
author: Yurii Serhiichuk
tags:
  - okf
  - knowledge-catalog
  - postgresql
  - database
  - schema
  - documentation
---

# PostgreSQL OKF Connector

This skill provides a Go-based CLI tool to convert PostgreSQL database schemas and comments (table and column descriptions) into Open Knowledge Format (OKF) bundles, and synchronize comments back to a PostgreSQL database from an OKF bundle.

## When to Use

Use this skill when you need to:
1. Extract table structures, column definitions, keys, schemas, and native PostgreSQL comments (descriptions) from a schema into a portable OKF bundle.
2. Synchronize business descriptions or documentation changes written in an OKF bundle back into native PostgreSQL table and column comments using native SQL comment mechanisms.

## Setup

The connector requires Go 1.18+:

```bash
cd skills/okf-postgresql
go build -o okf-postgresql main.go
```

## How to Use

### 1. Produce an OKF Bundle
Extract database schema metadata and comments:

```bash
./okf-postgresql produce --host <host> --port <port> --user <user> --password <password> --db <database-name> --schema <schema-name> --out <output-bundle-dir> [--tables <comma-separated-table-names>]
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
- `--password` (required): PostgreSQL password.
- `--db` (required): Target database name.
- `--schema` (default `public`): Target schema name.
- `--bundle` (required for ingest): Path to the OKF bundle.
- `--out` (required for produce): Path to output OKF bundle.
- `--sync` (optional for ingest): If provided, runs `COMMENT ON TABLE ...` and `COMMENT ON COLUMN ...` statements to synchronize descriptions.
