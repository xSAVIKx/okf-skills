#!/usr/bin/env bash
#
# sync-intra-deps.sh — bump the intra-repo okf-go pin in every consumer module
# to the lockstep release version and refresh each go.sum.
#
# Runs on the release-please PR branch (see release.yml / RELEASING.md strategy
# B). The okf-go tag for the new version does not exist on the remote yet, so we
# create it locally and route module resolution to this repo; because Go module
# hashes are content-based (not commit-SHA-based), the go.sum computed here stays
# valid for the real tag release-please creates on merge.
#
# Idempotent: with pins already at the target version this is a no-op (no diff).
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

MANIFEST=".release-please-manifest.json"
MODULE="github.com/xSAVIKx/okf-skills/okf-go"
OWNER_PATH="github.com/xSAVIKx/okf-skills"

if ! command -v jq >/dev/null 2>&1; then
  echo "sync-intra-deps: jq is required" >&2
  exit 2
fi

V="$(jq -r '."okf-go"' "$MANIFEST" | tr -d '\r')"
if [ -z "$V" ] || [ "$V" = "null" ]; then
  echo "sync-intra-deps: no okf-go version in $MANIFEST" >&2
  exit 2
fi
echo "Target okf-go version: v$V"

# Create the not-yet-pushed tag locally so `go mod tidy` can resolve okf-go@vV
# from this working tree. On merge, release-please tags this same commit.
if ! git rev-parse -q --verify "refs/tags/okf-go/v$V" >/dev/null 2>&1; then
  git tag "okf-go/v$V" HEAD
fi

# GOPRIVATE: fetch our modules 'direct' (bypass proxy + sumdb, which don't know
# the tag yet). insteadOf: redirect that direct git fetch to this local repo.
export GOPRIVATE="$OWNER_PATH"
export GOFLAGS=-mod=mod
export GOWORK=off
git config --global "url.file://$REPO_ROOT.insteadOf" "https://$OWNER_PATH"
trap 'git config --global --unset "url.file://$REPO_ROOT.insteadOf" >/dev/null 2>&1 || true' EXIT

# Every module that requires okf-go (okf-mcp + the skills; not okf-go itself).
count=0
while IFS= read -r gomod; do
  gomod="${gomod#./}"
  dir="$(dirname "$gomod")"
  echo "== $dir =="
  sed -i -E "s#(${MODULE}) v[0-9]+\.[0-9]+\.[0-9]+#\1 v${V}#" "$gomod"
  ( cd "$dir" && go mod tidy )
  count=$((count + 1))
done < <(grep -rl "${MODULE} v" --include=go.mod . | tr -d '\r')

echo "sync-intra-deps: okf-go pinned at v$V across $count module(s)"
