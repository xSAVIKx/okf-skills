---
name: okf-mongodb
description: MongoDB connector that produces and ingests Open Knowledge Format (OKF) bundles by sampling documents to infer each collection's schema. Emits one concept per collection with a Name | Type | Presence table (top-level fields, with mixed types shown as a union and presence as the percentage of sampled documents). The connection URI is bound via the MONGODB_URI environment variable and stripped of credentials before being recorded. ingest verifies the bundle against the database and, with --sync, creates missing collections; descriptions stay in the bundle. Use when documenting or cataloging a MongoDB database. CGO-free pure Go.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary. No CGO needed.
metadata:
  version: "0.2.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, mongodb, document-store, schema"
---

# MongoDB OKF Connector

This skill provides a Go-based CLI tool to document a MongoDB database as an Open
Knowledge Format (OKF) bundle. MongoDB is schemaless, so the connector **samples
documents** from each collection and infers a top-level field schema: field name,
type (a sorted union when documents disagree), and presence (the percentage of
sampled documents that contain the field).

## When to Use

Use this skill when you need to:
1. Catalog a MongoDB database — one `MongoDB Collection` concept per collection.
2. Capture an inferred field schema (`Name | Type | Presence`) as grounding for
   enrichment.
3. Verify a bundle against a live database in CI, optionally creating missing
   collections.

MongoDB has no per-field comment store, so enriched descriptions remain in the OKF
bundle (there is no description write-back).

## Setup

```bash
go install github.com/xSAVIKx/okf-skills/skills/okf-mongodb@latest
# …or: cd skills/okf-mongodb && go build -o okf-mongodb .
```

## How to Use

### 1. Produce an OKF bundle

```bash
export MONGODB_URI="mongodb://user:pass@host:27017"
./okf-mongodb produce --db <database> --out <bundle-dir> [--collections a,b] [--sample 100]
```

- `--uri` (required): connection URI, or set `MONGODB_URI` (preferred — keeps the
  secret out of argv). Credentials are stripped before being written to `Resource`.
- `--db` (required): database name.
- `--out` (required): output OKF bundle directory.
- `--collections`: comma-separated allowlist (default: all).
- `--sample`: documents to sample per collection for inference (default 100).

Re-running preserves enriched descriptions (incremental produce).

### 2. Ingest / verify

```bash
./okf-mongodb ingest --db <database> --bundle <bundle-dir> [--sync]
```

- Reports collections that are in the bundle but missing from the database (drift).
- `--sync`: creates those missing collections (structure only; no documents, no
  description write-back).

### 3. Inspect the schema (self-description)

```bash
./okf-mongodb schema
```

Prints the machine-readable JSON description used by `okf-mcp` to expose the skill.
