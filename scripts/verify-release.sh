#!/usr/bin/env bash
#
# verify-release.sh — prove every released module installs/builds STANDALONE,
# i.e. without the go.work workspace, exactly as an end user's `go install
# <module>@vX.Y.Z` would resolve it.
#
# Why this exists: during development go.work shadows the intra-repo
# `require github.com/xSAVIKx/okf-skills/okf-go vX` pins with working-tree
# source, so a connector compiles even when its pin (or go.sum) does not match
# the okf-go version it actually uses. That mismatch is invisible to normal CI
# and only surfaces when a user installs the published module. This gate runs
# AFTER tags are pushed and reproduces the user's resolution path, so a broken
# release fails loudly here instead of in someone's `go install`.
#
# Usage:
#   scripts/verify-release.sh [path/to/.release-please-manifest.json]
#
# Reads each module's released version from the release-please manifest and
# verifies it. Exit non-zero if any module fails to resolve or build.
set -euo pipefail

MODULE_BASE="github.com/xSAVIKx/okf-skills"
MANIFEST="${1:-.release-please-manifest.json}"

if ! command -v jq >/dev/null 2>&1; then
  echo "verify-release: jq is required" >&2
  exit 2
fi
if [ ! -f "$MANIFEST" ]; then
  echo "verify-release: manifest not found: $MANIFEST" >&2
  exit 2
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

export GOBIN="$workdir/bin"
export GOWORK=off                 # the whole point: no workspace shadowing
export GOFLAGS="${GOFLAGS:--mod=mod}"
export GOPROXY="${GOPROXY:-direct}"   # fetch tags straight from VCS (no proxy indexing lag)
export GOSUMDB="${GOSUMDB:-off}"      # skip sum.golang.org (lags new tags); still verifies committed go.sum

fail=0

# Every binary module (a main package) — everything except the okf-go library.
# The library is verified separately below; it is also exercised transitively
# by each connector that imports it.
while IFS= read -r path; do
  path="${path%$'\r'}"   # tolerate CRLF from a Windows jq build
  version="$(jq -r --arg p "$path" '.[$p]' "$MANIFEST" | tr -d '\r')"
  ref="$MODULE_BASE/$path@v$version"
  echo "== verifying (install) $ref =="
  if go install "$ref"; then
    echo "  ok"
  else
    echo "  FAILED: $ref does not install standalone"
    fail=1
  fi
done < <(jq -r 'keys[] | select(. != "okf-go")' "$MANIFEST" | tr -d '\r')

# The okf-go library: resolve and compile it on its own in a throwaway module.
okfgo_version="$(jq -r '."okf-go"' "$MANIFEST" | tr -d '\r')"
ref="$MODULE_BASE/okf-go@v$okfgo_version"
echo "== verifying (library build) $ref =="
probe="$workdir/okfgo-probe"
mkdir -p "$probe"
if ( cd "$probe" \
      && go mod init okfgo.probe >/dev/null 2>&1 \
      && go get "$ref" >/dev/null 2>&1 \
      && go build "$MODULE_BASE/okf-go" ); then
  echo "  ok"
else
  echo "  FAILED: $ref does not build standalone"
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo "verify-release: one or more modules are not standalone-installable" >&2
  exit 1
fi
echo "verify-release: all modules install/build standalone"
