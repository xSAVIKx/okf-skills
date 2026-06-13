---
name: okf-mysql
description: MySQL connector for producing and ingesting Open Knowledge Format (OKF) bundles.
version: 0.1.0
author: Yurii Serhiichuk
tags:
  - okf
  - knowledge-catalog
  - mysql
  - database
  - schema
  - documentation
---

# MySQL OKF Connector

This skill provides a Go-based CLI tool to convert MySQL database schemas and comments (table and column comments) into Open Knowledge Format (OKF) bundles, and synchronize comments back to a MySQL database from an OKF bundle.

## When to Use

Use this skill when you need to:
1. Extract table structures, column definitions, keys, and native MySQL comments from a schema into a portable OKF bundle.
2. Synchronize business descriptions or documentation changes written in an OKF bundle back into native MySQL table and column comments.

## Setup

The connector requires Go 1.18+:

```bash
cd skills/okf-mysql
go build -o okf-mysql main.go
```

## How to Use

### 1. Produce an OKF Bundle
Extract database schema metadata and comments:

```bash
./okf-mysql produce --host <host> --port <port> --user <user> --password <password> --db <database-name> --out <output-bundle-dir> [--tables <comma-separated-table-names>]
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
- `--password` (required): MySQL password.
- `--db` (required): Target database schema.
- `--bundle` (required for ingest): Path to the OKF bundle.
- `--out` (required for produce): Path to output OKF bundle.
- `--sync` (optional for ingest): If provided, runs `ALTER TABLE ... COMMENT = ...` and `MODIFY COLUMN ... COMMENT ...` statements to synchronize descriptions.
