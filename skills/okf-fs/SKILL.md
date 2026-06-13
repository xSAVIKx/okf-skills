# Skill: okf-fs (Local Filesystem OKF Connector)

This skill produces and ingests OKF bundles documenting local directory tree structures and metadata, respecting `.okfignore` files.

## Commands

### 1. `produce`
Traverses the specified directory, ignores files matching `.okfignore` rules, reads description text from `.okf-metadata.yaml`, and outputs an OKF bundle.

```bash
okf-fs produce -dir <directory-to-document> -out <bundle-output-dir>
```

Options:
- `-dir` (string): The path to the local directory to document (required).
- `-out` (string): The path where the generated OKF bundle will be created (required).

### 2. `ingest`
Compares an OKF bundle with the target directory, checks for file existence and mismatches, and optionally syncs file descriptions back into `.okf-metadata.yaml`.

```bash
okf-fs ingest -dir <target-directory> -bundle <okf-bundle-dir> [-sync]
```

Options:
- `-dir` (string): The target directory to compare against (required).
- `-bundle` (string): The path to the OKF bundle to ingest (required).
- `-sync` (bool): If true, updates `.okf-metadata.yaml` at the directory root with the descriptions from the bundle.
