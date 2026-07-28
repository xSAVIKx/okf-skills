---
name: okf-graphql
description: GraphQL connector that produces and ingests Open Knowledge Format (OKF) bundles from a GraphQL SDL document. Parses the schema into one concept per user-defined type (object/input/interface/enum/union) and per root operation (query/mutation/subscription field), links fields and operations to the types they reference as native relationship edges, and syncs descriptions back to a .okf-metadata.yaml sidecar. Use when documenting or cataloging a GraphQL API from its schema, with no live server. Pure Go.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary. No CGO needed.
metadata:
  version: "0.2.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, graphql, api, schema"
---

# GraphQL OKF Connector

This skill provides a Go-based CLI tool to document a GraphQL schema (SDL) as an Open
Knowledge Format (OKF) bundle: one concept per user-defined type and per root
operation, with fields and operations linked to the types they reference. It needs
only the SDL file — no live server.

## When to Use

Use this skill when you need to:
1. Catalog a GraphQL API from its SDL — types under `types/`, queries under
   `queries/`, mutations under `mutations/` (subscriptions under `subscriptions/`).
2. Turn the type graph into a connected, browsable view — field→type and
   operation→type `references` edges render as typed edges in `okf-viz`.
3. Round-trip enriched descriptions back to a `.okf-metadata.yaml` sidecar.

## Setup

```bash
go install github.com/xSAVIKx/okf-skills/skills/okf-graphql@latest
# …or: cd skills/okf-graphql && go build -o okf-graphql .
```

## How to Use

### 1. Produce an OKF bundle

```bash
./okf-graphql produce --schema <schema.graphql> --out <bundle-dir>
```

- `--schema` (required): path to the GraphQL SDL document.
- `--out` (required): output OKF bundle directory.

Each object/input/interface type becomes `types/<Name>.md` with a `# Columns` field
table (`Name | Type | Required`); enums emit a `# Values` list; unions emit their
members. Each root field becomes an operation concept with its arguments and return
type. Fields/operations link to the types they reference under `# Relationships`.
Re-running preserves enriched descriptions.

### 2. Ingest / sync descriptions

```bash
./okf-graphql ingest --schema <schema.graphql> --bundle <bundle-dir> [--sync]
```

- Verifies each concept still exists in the schema (reports drift); the SDL is never
  modified.
- `--sync`: writes the bundle's descriptions to `.okf-metadata.yaml` next to the SDL.

### 3. Inspect the schema (self-description)

```bash
./okf-graphql schema
```

Prints the machine-readable JSON description used by `okf-mcp` to expose the skill.
