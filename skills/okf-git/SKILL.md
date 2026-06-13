# Skill: okf-git (Git Repository OKF Connector)

This skill produces and ingests OKF bundles documenting Git repository file trees, including revision history metadata (commit times, authors, messages), respecting `.gitignore` and `.okfignore` files.

## Commands

### 1. `produce`
Traverses the Git repository files, skips ignored files, queries commit logs for history metadata, reads description comments from `.okf-metadata.yaml`, and outputs an OKF bundle.

```bash
okf-git produce -repo <repository-path> -out <bundle-output-dir>
```

Options:
- `-repo` (string): The path to the local Git repository to document (required).
- `-out` (string): The path where the generated OKF bundle will be created (required).

### 2. `ingest`
Validates an OKF bundle against tracked Git repository files and optionally syncs descriptions back into `.okf-metadata.yaml` at the repository root.

```bash
okf-git ingest -repo <repository-path> -bundle <okf-bundle-dir> [-sync]
```

Options:
- `-repo` (string): The path to the local Git repository to update (required).
- `-bundle` (string): The path to the OKF bundle to ingest (required).
- `-sync` (bool): If true, updates `.okf-metadata.yaml` at the repository root with the descriptions from the bundle.
