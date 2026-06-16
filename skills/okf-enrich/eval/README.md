# Enrichment evaluation harness

A lightweight, **optional** LLM-as-judge workflow for scoring the *quality* of OKF
descriptions — not just whether they exist (that is `okf-viz coverage`). There is
**no binary and no embedded model**: the agent already in the loop runs the judge
with its own model, exactly as `okf-enrich` itself is instructions-only.

> **Coverage vs eval.** `okf-viz coverage` deterministically counts *how much* is
> enriched. This harness judges *how well*. They compose: coverage tells you which
> concepts have descriptions; the eval tells you which of those descriptions are
> weak. Run coverage first, then eval the concepts that have descriptions.

## When to run it

- **Regression mode** — after changing `okf-enrich/SKILL.md` guidance (e.g. the
  grounding, relationship-prose, or tag rules), re-score the fixtures in
  [`fixtures/`](fixtures/) and compare aggregate per-dimension scores to the
  recorded baseline. A drop flags a regression in the prompt guidance.
- **Bundle-review mode** — score a (sample of a) real bundle's descriptions; the
  lowest-scoring concepts are the **low-confidence** ones to flag for human review.
  Pair with `okf-viz coverage` so review effort targets concepts that *have*
  descriptions but score poorly.

This is **advisory**, not a hard CI gate: an LLM judge is non-deterministic, so use
score **bands** and aggregate trends, not exact pass/fail. (A team may still choose
to gate on aggregate scores; the harness does not mandate it.)

## How the agent runs the judge

For each concept being evaluated:

1. **Gather the same grounding the writer had** — per `okf-reader`, read the
   concept's `# Columns`, `## Data Profile` (incl. `Semantic`/`Values`), `## Sample`,
   and `# Relationships`. The judge must score against *evidence*, not vibes.
2. **Read the candidate description** (the frontmatter `description`, and any
   relationship prose / tags if those are in scope).
3. **Score each dimension** in [`rubric.md`](rubric.md) on its 1–5 scale, using the
   written anchors. Record a one-line rationale per concept.
4. **Emit a small table**: `concept | grounding | specificity | conciseness | (idempotency) | rationale`.

### Regression mode procedure

1. Run the judge over every case in [`fixtures/cases.yaml`](fixtures/cases.yaml)
   using the *current* `SKILL.md` guidance.
2. Confirm each candidate lands in (or adjacent to) its `expected_band`. Gross
   misordering — a hallucinated description out-scoring a grounded one — is a rubric
   or guidance bug.
3. Record the aggregate per-dimension mean as the baseline.
4. After a guidance change, re-run and compare. Investigate any dimension that drops
   by more than the documented variance (±0.5 on the 1–5 scale).

### Bundle-review mode procedure

1. Run `okf-viz coverage --bundle <dir>` to find which concepts *have*
   (non-placeholder) descriptions.
2. Score those concepts (or a representative sample) with the rubric.
3. Sort ascending by total score; the bottom of the list is your human-review queue.

## Files

- [`rubric.md`](rubric.md) — the scoring dimensions, scale, and anchors (derived
  from the `okf-enrich` quality rules).
- [`fixtures/`](fixtures/) — concept fixtures spanning connector shapes, each paired
  in [`fixtures/cases.yaml`](fixtures/cases.yaml) with several candidate
  descriptions (strong / restated-schema / hallucinated / over-long) and an expected
  score band. The fixtures double as worked examples and the regression baseline.
