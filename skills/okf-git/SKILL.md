---
name: okf-git
description: Git repository connector that produces and ingests Open Knowledge Format (OKF) bundles documenting repository file trees plus commit-history metadata (times, authors, messages), honoring .gitignore and .okfignore. Use when documenting or cataloging a Git repository as an OKF bundle.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the connector binary, plus a local Git repository to read history from.
metadata:
  version: "0.4.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, git, documentation"
---

# Skill: okf-git (Git Repository OKF Connector)

This skill produces and ingests OKF bundles documenting Git repository file trees, including revision history metadata (commit times, authors, messages), respecting `.gitignore` and `.okfignore` files.

## Setup

The connector is written in Go and requires Go 1.24+ to build from source:

```bash
# Install the published binary (Go 1.24+) — no clone needed:
go install github.com/xSAVIKx/okf-skills/skills/okf-git@v0.1.0

# …or build from a clone of the repository:
cd skills/okf-git
go build -o okf-git .
```

## Commands

### 1. `produce`
Traverses the Git repository files, skips ignored files, queries commit logs for history metadata, reads description comments from `.okf-metadata.yaml`, and outputs an OKF bundle.

```bash
./okf-git produce --repo <repository-path> --out <bundle-output-dir>
```

Options:
- `--repo` (string): The path to the local Git repository to document (required).
- `--out` (string): The path where the generated OKF bundle will be created (required).

### 2. `ingest`
Validates an OKF bundle against tracked Git repository files and optionally syncs descriptions back into `.okf-metadata.yaml` at the repository root.

```bash
./okf-git ingest --repo <repository-path> --bundle <okf-bundle-dir> [--sync]
```

Options:
- `--repo` (string): The path to the local Git repository to update (required).
- `--bundle` (string): The path to the OKF bundle to ingest (required).
- `--sync` (bool): If true, updates `.okf-metadata.yaml` at the repository root with the descriptions from the bundle.

### 3. Inspect the Schema (self-description)
Print a machine-readable JSON description of this skill's commands and flags (used by `okf-mcp` to expose the skill as an MCP tool):

```bash
./okf-git schema
```
