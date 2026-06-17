---
name: okf-fs
description: Local filesystem connector that produces and ingests Open Knowledge Format (OKF) bundles documenting directory trees and file metadata, honoring .okfignore and .okf-metadata.yaml. Use when documenting or cataloging a local folder structure, or capturing filesystem metadata as an OKF bundle.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary.
metadata:
  version: "0.6.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, filesystem, documentation"
---

# Skill: okf-fs (Local Filesystem OKF Connector)

This skill produces and ingests OKF bundles documenting local directory tree structures and metadata, respecting `.okfignore` files.

## Setup

The connector is written in Go and requires Go 1.24+ to build from source:

```bash
# Install the published binary (Go 1.24+) — no clone needed:
go install github.com/xSAVIKx/okf-skills/skills/okf-fs@v0.1.0

# …or build from a clone of the repository:
cd skills/okf-fs
go build -o okf-fs .
```

## Commands

### 1. `produce`
Traverses the specified directory, ignores files matching `.okfignore` rules, reads description text from `.okf-metadata.yaml`, and outputs an OKF bundle.

```bash
./okf-fs produce --dir <directory-to-document> --out <bundle-output-dir>
```

Options:
- `--dir` (string): The path to the local directory to document (required).
- `--out` (string): The path where the generated OKF bundle will be created (required).

### 2. `ingest`
Compares an OKF bundle with the target directory, checks for file existence and mismatches, and optionally syncs file descriptions back into `.okf-metadata.yaml`.

```bash
./okf-fs ingest --dir <target-directory> --bundle <okf-bundle-dir> [--sync]
```

Options:
- `--dir` (string): The target directory to compare against (required).
- `--bundle` (string): The path to the OKF bundle to ingest (required).
- `--sync` (bool): If true, updates `.okf-metadata.yaml` at the directory root with the descriptions from the bundle.

### 3. Inspect the Schema (self-description)
Print a machine-readable JSON description of this skill's commands and flags (used by `okf-mcp` to expose the skill as an MCP tool):

```bash
./okf-fs schema
```
