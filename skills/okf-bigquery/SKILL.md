---
name: okf-bigquery
description: Google Cloud BigQuery connector that produces and ingests Open Knowledge Format (OKF) bundles from dataset schemas, table/field descriptions, and metadata. Use when documenting or cataloging BigQuery datasets, extracting schema metadata into OKF, or syncing descriptions back to BigQuery tables and fields.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary, plus Google Cloud credentials or an API key to reach BigQuery.
metadata:
  version: "0.7.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, bigquery, google-cloud, database, schema, documentation"
---

# BigQuery OKF Connector

This skill provides a Go-based CLI tool to convert Google Cloud BigQuery dataset schemas and table/field descriptions into Open Knowledge Format (OKF) bundles, and synchronize comments/descriptions back to BigQuery tables and fields.

## When to Use

Use this skill when you need to:
1. Extract table structures, schemas, field descriptions, and dataset descriptions from a Google Cloud BigQuery dataset into a portable OKF bundle.
2. Synchronize business descriptions or documentation changes written in an OKF bundle back into native BigQuery table and schema field descriptions.

## Setup

The connector requires Go 1.24+ and utilizes the official Google Cloud BigQuery Client:

```bash
# Install the published binary (Go 1.24+) — no clone needed:
go install github.com/xSAVIKx/okf-skills/skills/okf-bigquery@v0.1.0

# …or build from a clone of the repository:
cd skills/okf-bigquery
go build -o okf-bigquery .
```

## How to Use

Ensure you have set your GCP credentials. Usually, this is done by setting the `GOOGLE_APPLICATION_CREDENTIALS` environment variable:
```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/credentials.json"
```

### 1. Produce an OKF Bundle
Extract dataset schema metadata and descriptions:

```bash
./okf-bigquery produce --project <gcp-project-id> --dataset <dataset-id> --out <output-bundle-dir> [--tables <comma-separated-table-names>] [--sample <N>] [--profile] [--relationships] [--stats]
```

### 2. Ingest / Synchronize an OKF Bundle
Synchronize table and field descriptions from the OKF bundle back to BigQuery:

```bash
./okf-bigquery ingest --project <gcp-project-id> --dataset <dataset-id> --bundle <path-to-okf-bundle> [--sync]
```

**Parameters:**
- `--project` (required): Google Cloud Project ID.
- `--dataset` (required): Target BigQuery dataset ID.
- `--bundle` (required for ingest): Path to the OKF bundle.
- `--out` (required for produce): Path to output OKF bundle.
- `--tables` (optional): Filter to extract only specific tables.
- `--sample <N>` (optional): Embed up to N sample rows per table as a `## Sample` section in each table doc (default 0 = none).
- `--profile` (optional): Compute per-column statistics (non-null, null, distinct, min, max) and embed a `## Data Profile` section.
- `--relationships` (optional): Extract informational foreign-key constraints into a `## Relationships` section.
- `--stats` (optional): Add a `## Stats` section with the table row count (from table metadata) and, when a timestamp-like column is detected, a freshness window (min/max). Constraints (`## Constraints`) and view definitions (`## View Definition`, views only) are emitted by default. BigQuery has no secondary indexes, so no Indexes section is produced.
- `--sync` (optional for ingest): If provided, calls the BigQuery API to update table and schema descriptions.

### 3. Inspect the Schema (self-description)
Print a machine-readable JSON description of this skill's commands and flags (used by `okf-mcp` to expose the skill as an MCP tool):

```bash
./okf-bigquery schema
```
