---
name: okf-openapi
description: OpenAPI/Swagger connector that produces and ingests Open Knowledge Format (OKF) bundles from an API contract. Parses an OpenAPI 3.x or Swagger 2.0 spec (JSON or YAML) into one concept per operation (API Endpoint) and per component schema (Schema), links endpoints to the schemas they reference as typed cross-links, and syncs descriptions back to a .okf-metadata.yaml sidecar. Use when documenting or cataloging an HTTP API from its spec, with no live server or credentials. Pure Go.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary. No CGO needed.
metadata:
  version: "0.2.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, openapi, swagger, api, schema"
---

# OpenAPI OKF Connector

This skill provides a Go-based CLI tool to document an OpenAPI/Swagger specification as
an Open Knowledge Format (OKF) bundle: one `API Endpoint` concept per operation and one
`Schema` concept per component schema, with endpoints linked to the schemas they
reference. It needs only the spec file — no live server, no credentials.

## When to Use

Use this skill when you need to:
1. Catalog an HTTP API from its OpenAPI 3.x or Swagger 2.0 contract (JSON or YAML).
2. Turn endpoints and schemas into a connected, browsable graph (the endpoint→schema
   `references` edges render as typed edges in `okf-viz`).
3. Round-trip enriched descriptions back to a `.okf-metadata.yaml` sidecar.

## Setup

```bash
# Install the published binary (Go 1.24+):
go install github.com/xSAVIKx/okf-skills/skills/okf-openapi@latest

# …or build from a clone:
cd skills/okf-openapi && go build -o okf-openapi .
```

## How to Use

### 1. Produce an OKF bundle

```bash
./okf-openapi produce --spec <api.yaml|api.json> --out <bundle-dir>
```

- `--spec` (required): path to the OpenAPI 3.x or Swagger 2.0 spec (JSON or YAML;
  2.0 is converted to 3.x automatically).
- `--out` (required): output OKF bundle directory.

Each operation becomes `endpoints/<operationId>.md` (with parameter and response
tables); each component schema becomes `schemas/<Name>.md` (with a `# Columns`
property table: `Name | Type | Required`). Endpoints and schemas link to the schemas
they reference under `# Relationships`. Re-running preserves enriched descriptions.

### 2. Ingest / sync descriptions

```bash
./okf-openapi ingest --spec <api.yaml> --bundle <bundle-dir> [--sync]
```

- Verifies each concept still exists in the spec (reports drift); the spec file is
  never modified.
- `--sync`: writes the bundle's descriptions to `.okf-metadata.yaml` next to the spec.

### 3. Inspect the schema (self-description)

```bash
./okf-openapi schema
```

Prints the machine-readable JSON description used by `okf-mcp` to expose the skill.
