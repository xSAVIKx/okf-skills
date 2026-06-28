---
name: okf-lint
description: Deterministic validator for Open Knowledge Format (OKF) bundles. Checks spec conformance (root index.md carries only okf_version, subdirectory index.md files carry no frontmatter, every concept has a non-empty type) plus enrichment coverage (placeholder descriptions, broken cross-links, orphans), and gates CI via its exit code. Use to validate an OKF bundle before publishing or ingestion, or to fail a build when a bundle regresses. Complements skills-ref validate, which only checks SKILL.md frontmatter.
license: Apache-2.0
compatibility: Requires the Go toolchain (1.24+) to build the consumer binary. No CGO needed.
metadata:
  version: "0.1.0"
  author: Yurii Serhiichuk
  tags: "okf, knowledge-catalog, lint, validation, conformance, ci"
---

# OKF Lint

`okf-lint` is a Go-based CLI consumer skill that validates an OKF bundle and gates CI
with its exit code. It is deterministic and read-only — no LLM, no mutation — and
shares its bundle scanner with `okf-viz coverage` (both call `okf-go`'s `ScanBundle`).

## When to Use

Use this skill to:
1. Validate that a bundle conforms to the OKF spec (frontmatter rules, concept `type`).
2. Gate a build/publish step on documentation quality (broken links, enrichment %).
3. Catch regressions in CI before a bundle is ingested or shared.

It complements `skills-ref validate` (which checks a skill's `SKILL.md` frontmatter):
`okf-lint` validates the *bundle's own* concept documents.

## How to Use

### Lint a bundle

```bash
okf-lint lint --bundle <path-to-okf-bundle> [flags]
```

**Flags:**
- `--bundle` (required): path to the OKF bundle directory.
- `--min <pct>`: fail if the enriched percentage is below this threshold (0 = no gate).
- `--max-broken-links <N>`: maximum tolerated broken cross-links before failing (default 0).
- `--require-types` (default true): fail if any concept is missing a non-empty `type`.
- `--strict` (default false): also fail when there are orphan (cross-link-less) concepts.
- `--json`: emit the full report as JSON instead of the text summary.

**Always-enforced spec conformance** (independent of flags): the root `index.md` may
carry only `okf_version`; subdirectory `index.md` files must carry no frontmatter;
every concept must have parseable frontmatter. These cause a non-zero exit.

**Exit code:** `0` when all gates pass, `1` when any gate is violated (with the
reasons printed to stderr) — suitable for a CI gate.

```bash
# Example CI gate: require 80% enriched, no broken links, no missing types.
okf-lint lint --bundle ./catalog --min 80
```

### Inspect the schema (self-description)

```bash
okf-lint schema
```

Prints the machine-readable JSON description used by `okf-mcp` to expose the skill.
