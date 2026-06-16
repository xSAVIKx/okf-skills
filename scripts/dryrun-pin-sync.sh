#!/usr/bin/env bash
# Linux dry-run for strategy B (run inside a golang container, repo at /src:ro).
# Simulates release-please bumping the manifest to a new lockstep version, runs
# the real scripts/sync-intra-deps.sh, and asserts every okf-go consumer is
# re-pinned, has a refreshed go.sum, and builds standalone (GOWORK=off) — all
# against an okf-go tag that is NOT pushed anywhere.
set -euo pipefail

NEWVER="0.2.0"
WORK=/work
git clone -q /src "$WORK"
cd "$WORK"
git config user.email dryrun@example.com
git config user.name dryrun

echo "== simulate release-please: bump every module in the manifest to $NEWVER =="
tmp="$(mktemp)"
jq 'to_entries | map(.value = "'"$NEWVER"'") | from_entries' .release-please-manifest.json > "$tmp"
mv "$tmp" .release-please-manifest.json

echo "== run the real sync script (from the mounted, possibly-uncommitted copy) =="
tr -d '\r' < /src/scripts/sync-intra-deps.sh > /sync.sh
bash /sync.sh

echo "== assert every consumer is re-pinned, has go.sum, and builds standalone =="
export GOWORK=off GOFLAGS=-mod=mod GOPRIVATE=github.com/xSAVIKx/okf-skills
git config --global "url.file://$WORK.insteadOf" "https://github.com/xSAVIKx/okf-skills"
fail=0
while IFS= read -r gomod; do
  gomod="${gomod#./}"; dir="$(dirname "$gomod")"
  if ! grep -q "okf-skills/okf-go v$NEWVER" "$gomod"; then
    echo "  FAIL: $gomod not pinned to v$NEWVER"; fail=1; continue
  fi
  if ! grep -q "okf-skills/okf-go v$NEWVER" "$dir/go.sum"; then
    echo "  FAIL: $dir/go.sum missing okf-go v$NEWVER"; fail=1; continue
  fi
  if ( cd "$dir" && go build -o /dev/null ./... >/dev/null 2>&1 ); then
    echo "  PASS: $dir"
  else
    echo "  FAIL: $dir standalone build"; fail=1
  fi
done < <(grep -rl "okf-skills/okf-go v" --include=go.mod . | tr -d '\r')

[ "$fail" -eq 0 ] && echo "DRYRUN OK" || { echo "DRYRUN FAILED"; exit 1; }
