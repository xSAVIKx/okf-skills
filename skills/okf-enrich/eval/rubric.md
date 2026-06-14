# Enrichment scoring rubric

Each dimension is scored **1–5** against the written anchors below. The judge is
given the same grounding the writer had (`# Columns`, `## Data Profile` incl.
`Semantic`/`Values`, `## Sample`, `# Relationships`) and scores the candidate
description against that **evidence**, not against general plausibility.

The dimensions map directly to the four `okf-enrich` quality rules.

## 1. Grounding (rule 1 — "ground, don't guess")

Is every claim supported by the schema / profile / sample?

| Score | Anchor |
|---|---|
| 5 | Every claim is traceable to the evidence; no invented business meaning. |
| 4 | Almost entirely grounded; at most one mild, reasonable inference. |
| 3 | Mostly grounded but contains an unsupported assumption stated as fact. |
| 2 | Several claims the evidence does not support. |
| 1 | Hallucinated meaning; contradicts or ignores the schema/sample. |

## 2. Specificity (rules 1 & 3 — grain + purpose, not restatement)

Does it convey what the schema alone does not — the **grain** (what one row/record/
file represents) and the **purpose** — rather than restating column names?

| Score | Anchor |
|---|---|
| 5 | States the grain and the purpose; tells the reader something the columns don't. |
| 4 | States the grain clearly; purpose is implied. |
| 3 | Generic but correct ("a table of orders"); adds little beyond the title. |
| 2 | Mostly a restatement of column names. |
| 1 | Pure schema echo or empty/placeholder. |

## 3. Conciseness (rule 3 — concise and purposeful)

One sentence for a table/dataset/file; a short noun phrase for a column. No filler.

| Score | Anchor |
|---|---|
| 5 | Tight and complete — one well-formed sentence (or noun phrase for a column). |
| 4 | Slightly long but no filler. |
| 3 | Wordy; some redundancy. |
| 2 | Multi-sentence rambling or notable filler. |
| 1 | Bloated, repetitive, or padded with boilerplate. |

## 4. Surgical / idempotent respect (secondary; rule 2 & 4)

Did the candidate preserve what it should — touching only the `description` (and, in
scope, relationship prose / tags as union-only edits), never clobbering existing
substantive content or the body? Mostly a property of the pass; included for
completeness. Score `n/a` when evaluating a description in isolation.

| Score | Anchor |
|---|---|
| 5 | Only the intended field changed; existing content preserved; re-run stable. |
| 3 | Minor incidental change outside the intended field. |
| 1 | Clobbered a substantive description or rewrote the body. |

## Reading the scores

- **Per concept**: a description scoring ≤2 on Grounding or Specificity is a
  review/redo candidate regardless of the others.
- **Aggregate (regression mode)**: track the per-dimension mean across the fixtures.
  Treat a drop greater than the documented variance (±0.5) as a regression to
  investigate.
- Because the judge is non-deterministic, compare **bands and trends**, never exact
  equality.
