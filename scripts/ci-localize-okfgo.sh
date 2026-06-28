#!/usr/bin/env bash
#
# ci-localize-okfgo.sh — make an as-yet-unpublished intra-repo okf-go pin
# resolvable during normal (workspace-mode) CI on the release-please PR.
#
# The problem: on the release PR, sync-intra-deps.sh has already rewritten every
# consumer's `require okf-go vNEW` to the lockstep release version, but the
# `okf-go/vNEW` tag does not exist on the remote yet — release-please creates it
# only when the PR merges. `go.work` does NOT hide this from the build: even in
# workspace mode Go still runs module-graph version selection across all `use`d
# modules, selects okf-go@vNEW, and tries to read its go.mod from the tag —
# failing with "unknown revision okf-go/vNEW". That breaks `go vet`, `make
# build`, and the integration suite on the release PR. See RELEASING.md.
#
# The fix mirrors the resolution sync-intra-deps.sh already relies on: create the
# vNEW tag *locally* at HEAD (Go module hashes are content-addressed, so it is
# identical to the real tag cut on merge) and redirect direct VCS fetches of our
# module path to this checkout. Subsequent steps then build inside go.work
# exactly as usual, with okf-go@vNEW resolvable from the working tree.
#
# Idempotent and a safe no-op off the release PR: if okf-go/vNEW is already
# published on origin (every normal branch), this does nothing.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

OWNER_PATH="github.com/xSAVIKx/okf-skills"
MANIFEST=".release-please-manifest.json"

if ! command -v jq >/dev/null 2>&1; then
  echo "ci-localize-okfgo: jq is required" >&2
  exit 2
fi

V="$(jq -r '."okf-go" // empty' "$MANIFEST" | tr -d '\r')"
if [ -z "$V" ]; then
  echo "ci-localize-okfgo: no okf-go version in $MANIFEST — nothing to do"
  exit 0
fi
TAG="okf-go/v$V"

# If the pinned version is already published, the workspace resolves it normally.
if git ls-remote --tags --exit-code origin "refs/tags/$TAG" >/dev/null 2>&1; then
  echo "ci-localize-okfgo: $TAG is published on origin — no localization needed"
  exit 0
fi

echo "ci-localize-okfgo: $TAG is not published yet — localizing okf-go for CI"

# Create the not-yet-pushed tag locally so the workspace can resolve okf-go@vNEW
# from this checkout. On merge, release-please tags this same content.
if ! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null 2>&1; then
  git tag "$TAG" HEAD
fi

# Route 'direct' VCS fetches of our module path to this local checkout, and mark
# the path private so the proxy and checksum DB (which do not know the tag yet)
# are bypassed. GOPRIVATE alone implies GONOSUMDB + GONOPROXY for these paths;
# the committed go.sum (refreshed by sync-intra-deps.sh) still gets verified.
git config --global "url.file://$REPO_ROOT.insteadOf" "https://$OWNER_PATH"

if [ -n "${GITHUB_ENV:-}" ]; then
  echo "GOPRIVATE=$OWNER_PATH" >> "$GITHUB_ENV"
fi
export GOPRIVATE="$OWNER_PATH"

echo "ci-localize-okfgo: created local $TAG and redirected $OWNER_PATH -> $REPO_ROOT"
