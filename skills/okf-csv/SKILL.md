---
name: okf-csv
description: CSV connector that produces and ingests Open Knowledge Format (OKF) bundles from a directory of CSV files. Infers each file's column schema (integer/number/boolean/date/string) by sampling rows, optionally embeds a per-column data profile and sample rows, and syncs descriptions back to a .okf-metadata.yaml sidecar. Use when documenting or cataloging a folder of CSV/flat files, or capturing their inferred schema and stats as an OKF bundle. CGO-free, pure Go.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary. No CGO needed.
metadata:
  version: "0.1.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, csv, flat-file, schema, profile"
---

# CSV OKF Connector

This skill provides a Go-based CLI tool to document a directory of CSV files as an
Open Knowledge Format (OKF) bundle: one concept per CSV, with an inferred column
schema and an optional data profile, and to sync enriched descriptions back to a
`.okf-metadata.yaml` sidecar.

## When to Use

Use this skill when you need to:
1. Catalog a folder of CSV files — each becomes a `CSV File` concept under `tables/`,
   with a `# Columns` table whose types are inferred by sampling values.
2. Capture per-column statistics (`--profile`) and sample rows (`--sample`) as
   grounding for enrichment.
3. Round-trip enriched descriptions back to the source via `.okf-metadata.yaml`.

CSV has no declared types or comment store, so column types are inferred and
descriptions live in the sidecar (the same pattern `okf-fs` uses).

## Setup

```bash
# Install the published binary (Go 1.24+):
go install github.com/xSAVIKx/okf-skills/skills/okf-csv@latest

# …or build from a clone:
cd skills/okf-csv && go build -o okf-csv .
```

## How to Use

### 1. Produce an OKF bundle

```bash
./okf-csv produce --dir <csv-directory> --out <bundle-dir> [--sample <N>] [--profile]
```

- `--dir` (required): directory of CSV files (traversed recursively, honoring `.okfignore`).
- `--out` (required): output OKF bundle directory.
- `--sample <N>`: embed up to N sample rows per file as a `## Sample` section.
- `--profile`: compute per-column statistics (non-null, null, distinct, min, max,
  detected semantic type, low-cardinality value sets) into a `## Data Profile` section.

Each `data/orders.csv` becomes `tables/orders.md`. Re-running preserves unchanged
concepts byte-for-byte (incremental produce), keeping any enriched descriptions.

### 2. Ingest / sync descriptions

```bash
./okf-csv ingest --dir <csv-directory> --bundle <bundle-dir> [--sync]
```

- Verifies each concept's columns still match its CSV header (reports drift).
- `--sync`: writes the bundle's descriptions back to `.okf-metadata.yaml` in `--dir`.

### 3. Inspect the schema (self-description)

```bash
./okf-csv schema
```

Prints the machine-readable JSON description used by `okf-mcp` to expose the skill.
