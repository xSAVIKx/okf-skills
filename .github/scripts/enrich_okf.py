#!/usr/bin/env python3
"""Optional, best-effort enrichment of an OKF bundle's concept descriptions.

This is a *CI helper*, deliberately NOT an okf-* connector and NOT part of the
Go workspace: the connectors stay deterministic with no embedded LLM (see
skills/okf-producer-generator/SKILL.md). Enrichment is the judgment step, so it
lives here as a thin, dependency-free driver for whatever LLM the key unlocks.

It implements the okf-enrich procedure for an okf-fs bundle:
  * find concept docs whose `description` is still the connector placeholder
    ("File <name>" / "Directory <name>");
  * ask Gemini for a one-sentence, grounded description (grain + purpose);
  * rewrite ONLY the frontmatter `description:` line, byte-for-byte preserving
    everything else; never touch index.md / log.md.

Cost control: only placeholders are sent, batched. Combined with the
`.okf-metadata.yaml` cache in CI, steady-state runs enrich only new files.

Usage:
  GEMINI_KEY=...  python enrich_okf.py <bundle-dir> [--dry-run]

Env:
  GEMINI_KEY    (required unless --dry-run) Google AI Studio / Gemini API key.
  GEMINI_MODEL  Model id (default: gemini-2.0-flash) — pick a cheap "flash" tier.
  OKF_ENRICH_BATCH  Concepts per request (default: 25).

Exit code is always 0 on enrichment failure: a broken key or API hiccup must
publish placeholders, not break the site. Only argument/IO errors are fatal.
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

API_TMPL = "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={key}"

QUALITY_RULES = (
    "You are enriching an Open Knowledge Format (OKF) catalog of a software "
    "repository's files and directories. For each item write ONE concise "
    "description: state the grain (what the file/directory IS) then its purpose. "
    "Ground every claim ONLY in the path, name, extension and the metadata shown "
    "- never invent behaviour the evidence does not support. No trailing period "
    "is required. Keep each under 160 characters. Do NOT restate the obvious "
    "name; add meaning. Return STRICT JSON: an object mapping each item's \"id\" "
    "(as a string) to its description string. No prose, no markdown, JSON only."
)


def find_placeholders(bundle: Path):
    """Yield (path, lines, desc_idx) for concept docs needing enrichment."""
    for md in sorted(bundle.rglob("*.md")):
        rel = md.relative_to(bundle).as_posix()
        if rel in ("index.md", "log.md"):
            continue
        try:
            text = md.read_text(encoding="utf-8")
        except OSError as exc:  # unreadable file: skip, never crash
            print(f"  ! skip {rel}: {exc}", file=sys.stderr)
            continue
        lines = text.split("\n")
        ctype = title = None
        desc_idx = desc_val = None
        # Frontmatter only: scan the leading block bounded by '---'.
        seen_open = False
        for i, line in enumerate(lines):
            if line.strip() == "---":
                if not seen_open:
                    seen_open = True
                    continue
                break  # end of frontmatter
            if line.startswith("type:"):
                ctype = line.split(":", 1)[1].strip()
            elif line.startswith("title:"):
                title = line.split(":", 1)[1].strip()
            elif line.startswith("description:"):
                desc_idx = i
                desc_val = line.split(":", 1)[1].strip().strip('"').strip("'")
        if desc_idx is None or ctype is None or title is None:
            continue
        placeholder = f"{ctype} {title}"  # mirrors okf-fs produce defaults
        if desc_val == placeholder:
            yield md, lines, desc_idx, ctype, rel, "\n".join(lines)


def build_prompt(batch):
    items = []
    for item in batch:
        # item = (id, ctype, rel, body)
        items.append(
            {"id": str(item[0]), "type": item[1], "path": item[2], "doc": item[3]}
        )
    return (
        QUALITY_RULES
        + "\n\nItems:\n"
        + json.dumps(items, ensure_ascii=False, indent=0)
    )


def call_gemini(prompt: str, model: str, key: str) -> dict:
    body = json.dumps(
        {
            "contents": [{"parts": [{"text": prompt}]}],
            "generationConfig": {
                "temperature": 0.2,
                "responseMimeType": "application/json",
            },
        }
    ).encode("utf-8")
    req = urllib.request.Request(
        API_TMPL.format(model=model, key=key),
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=120) as resp:
        payload = json.loads(resp.read().decode("utf-8"))
    text = payload["candidates"][0]["content"]["parts"][0]["text"]
    return json.loads(text)


def yaml_quote(value: str) -> str:
    """Double-quoted YAML scalar; escape backslashes and quotes; one line."""
    v = value.replace("\\", "\\\\").replace('"', '\\"').replace("\n", " ").strip()
    return f'"{v}"'


def main() -> int:
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    flags = {a for a in sys.argv[1:] if a.startswith("--")}
    if len(args) != 1:
        print("usage: enrich_okf.py <bundle-dir> [--dry-run]", file=sys.stderr)
        return 2
    bundle = Path(args[0])
    if not bundle.is_dir():
        print(f"error: bundle dir not found: {bundle}", file=sys.stderr)
        return 2
    dry_run = "--dry-run" in flags

    targets = list(find_placeholders(bundle))
    print(f"Found {len(targets)} concept(s) with placeholder descriptions.")
    if not targets:
        return 0

    if dry_run:
        for md, _lines, _i, _t, rel, _body in targets:
            print(f"  would enrich: {rel}")
        return 0

    key = os.environ.get("GEMINI_KEY", "").strip()
    if not key:
        print("GEMINI_KEY not set; skipping enrichment (publishing placeholders).")
        return 0
    # An unset GitHub `vars.GEMINI_MODEL` arrives as "", not absent -> guard.
    model = os.environ.get("GEMINI_MODEL", "").strip() or "gemini-2.0-flash"
    batch_size = int(os.environ.get("OKF_ENRICH_BATCH", "").strip() or "25")

    # id -> (md path, lines, desc_idx)
    registry = {}
    for idx, (md, lines, desc_idx, ctype, rel, body) in enumerate(targets):
        registry[idx] = (md, lines, desc_idx)

    enriched = 0
    for start in range(0, len(targets), batch_size):
        chunk = targets[start : start + batch_size]
        batch = [
            (start + j, t[3], t[4], t[5]) for j, t in enumerate(chunk)
        ]  # (id, ctype, rel, body)
        try:
            result = call_gemini(build_prompt(batch), model, key)
        except (urllib.error.URLError, KeyError, ValueError, TimeoutError) as exc:
            print(f"  ! Gemini call failed for batch {start}: {exc}", file=sys.stderr)
            continue
        for sid, desc in result.items():
            try:
                rid = int(sid)
            except ValueError:
                continue
            if rid not in registry or not isinstance(desc, str) or not desc.strip():
                continue
            md, lines, desc_idx = registry[rid]
            lines[desc_idx] = "description: " + yaml_quote(desc)
            try:
                md.write_text("\n".join(lines), encoding="utf-8")
                enriched += 1
            except OSError as exc:
                print(f"  ! write failed {md}: {exc}", file=sys.stderr)

    print(f"Enriched {enriched}/{len(targets)} concept description(s).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
