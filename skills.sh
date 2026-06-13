#!/usr/bin/env bash
# skills.sh — build all OKF skill binaries and install them into a directory.
#
# Usage:
#   ./skills.sh [INSTALL_DIR]
# INSTALL_DIR defaults to $OKF_INSTALL_DIR, else $HOME/.local/bin.
#
# After installing, point any MCP-capable harness at the okf-mcp server:
#   okf-mcp --skills-dir <INSTALL_DIR>     (or add INSTALL_DIR to PATH)
set -euo pipefail

SKILLS="okf-sqlite okf-mysql okf-postgresql okf-bigquery okf-fs okf-git okf-mcp okf-enrich"

ROOT="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${1:-${OKF_INSTALL_DIR:-$HOME/.local/bin}}"

EXT=""
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) EXT=".exe" ;;
esac

mkdir -p "$INSTALL_DIR"
MANIFEST="$INSTALL_DIR/okf-skills-manifest.txt"
: > "$MANIFEST"

echo "Installing OKF skills into $INSTALL_DIR"
count=0
for skill in $SKILLS; do
  src="$ROOT/skills/$skill"
  out="$INSTALL_DIR/$skill$EXT"
  echo "  building $skill ..."
  ( cd "$src" && go build -o "$out" . )
  echo "$skill" >> "$MANIFEST"
  count=$((count + 1))
done

echo "Installed $count skills."
echo "Manifest: $MANIFEST"
echo "Next: ensure $INSTALL_DIR is on your PATH, then run 'okf-mcp' (or 'okf-mcp --skills-dir $INSTALL_DIR')."
