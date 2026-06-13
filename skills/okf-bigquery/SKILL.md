---
name: okf-bigquery
description: Google Cloud BigQuery connector for producing and ingesting Open Knowledge Format (OKF) bundles.
version: 0.1.0
author: Yurii Serhiichuk
tags:
  - okf
  - knowledge-catalog
  - bigquery
  - google-cloud
  - database
  - schema
  - documentation
---

# BigQuery OKF Connector

This skill provides a Go-based CLI tool to convert Google Cloud BigQuery dataset schemas and table/field descriptions into Open Knowledge Format (OKF) bundles, and synchronize comments/descriptions back to BigQuery tables and fields.

## When to Use

Use this skill when you need to:
1. Extract table structures, schemas, field descriptions, and dataset descriptions from a Google Cloud BigQuery dataset into a portable OKF bundle.
2. Synchronize business descriptions or documentation changes written in an OKF bundle back into native BigQuery table and schema field descriptions.

## Setup

The connector requires Go 1.18+ and utilizes the official Google Cloud BigQuery Client:

```bash
cd skills/okf-bigquery
go build -o okf-bigquery main.go
```

## How to Use

Ensure you have set your GCP credentials. Usually, this is done by setting the `GOOGLE_APPLICATION_CREDENTIALS` environment variable:
```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/credentials.json"
```

### 1. Produce an OKF Bundle
Extract dataset schema metadata and descriptions:

```bash
./okf-bigquery produce --project <gcp-project-id> --dataset <dataset-id> --out <output-bundle-dir> [--tables <comma-separated-table-names>]
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
- `--sync` (optional for ingest): If provided, calls the BigQuery API to update table and schema descriptions.
