---
name: okf-enrich
description: Generate LLM-powered concept descriptions and write them into an OKF bundle's frontmatter in place.
version: 0.1.0
author: Yurii Serhiichuk
tags:
  - okf
  - enrichment
  - llm
  - documentation
---

# OKF Enrich

`okf-enrich` walks an OKF bundle directory and uses an OpenAI-compatible LLM to generate a one-sentence `description` for every concept document that lacks one, writing the result back into the document's YAML frontmatter.

## How it works

1. **Walk** — scans the bundle directory recursively for `.md` files, skipping `index.md` and `log.md`.
2. **Decide** — by default only concepts with an empty `description` are processed; pass `--overwrite` to regenerate all.
3. **Prompt** — builds a prompt from the concept's title and body (including the `schema` and Data Profile / Sample sections when present).
4. **Write back** — stores the returned sentence in the `description` frontmatter field using `okf.WriteConceptDoc`.

The skill is source-agnostic: it reads whatever markdown the bundle contains, so it works with bundles produced by `okf-sqlite`, `okf-fs`, `okf-git`, or any other OKF skill.

> **NOTE:** `okf-enrich` currently enriches concept/table-level descriptions only. Column-level (field-level) description enrichment is a documented future enhancement.

## Commands

### `enrich`

Generate and write concept descriptions into an OKF bundle.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--bundle` | string | *(required)* | Path to the OKF bundle directory. |
| `--base-url` | string | `https://api.openai.com/v1` | OpenAI-compatible API base URL. |
| `--model` | string | `gpt-4o-mini` | Model name to use for generation. |
| `--api-key` | string | — | API key (see API key handling below). |
| `--overwrite` | bool | `false` | Regenerate descriptions even if already present. |

### `schema`

Print this skill's machine-readable JSON self-description (consumed by `okf-mcp` and other harnesses).

```bash
okf-enrich schema
```

## API key handling

The API key is resolved in order:

1. `--api-key` flag (explicit, takes precedence).
2. `OKF_LLM_API_KEY` environment variable.
3. `OPENAI_API_KEY` environment variable (common fallback).

If none is set, the skill exits with a clear error message. Using an environment variable keeps secrets out of process argument lists.

## Provider-agnostic usage

`okf-enrich` works with any OpenAI-compatible endpoint. Pass `--base-url` to target a different provider:

```bash
# OpenAI (default)
okf-enrich enrich --bundle ./my-bundle --model gpt-4o-mini

# Local Ollama instance
okf-enrich enrich --bundle ./my-bundle \
  --base-url http://localhost:11434/v1 \
  --model llama3 \
  --api-key ollama
```
