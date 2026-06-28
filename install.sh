#!/usr/bin/env bash
# install.sh — build all OKF skill binaries and install them into a directory.
#
# Usage:
#   ./install.sh [INSTALL_DIR]
# INSTALL_DIR defaults to $OKF_INSTALL_DIR, else $HOME/.local/bin.
#
# Each binary is stamped with its version (read from the skill's SKILL.md and
# injected via -ldflags), so `okf-sqlite --version` reports it. A human-readable
# manifest records every installed skill's name, version, and CHANGELOG path.
#
# After installing, point any MCP-capable harness at the okf-mcp server:
#   okf-mcp --skills-dir <INSTALL_DIR>     (or add INSTALL_DIR to PATH)
set -euo pipefail

SKILLS="okf-sqlite okf-mysql okf-postgresql okf-bigquery okf-fs okf-git okf-viz okf-lint okf-csv okf-openapi"

ROOT="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${1:-${OKF_INSTALL_DIR:-$HOME/.local/bin}}"

EXT=""
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) EXT=".exe" ;;
esac

# skillVersion reads metadata.version from a skill's SKILL.md, defaulting to
# "dev" if absent. This SKILL.md value is the single source of truth, kept in
# sync with releases (see RELEASING.md).
skillVersion() {
  local skill_md="$1"
  local v
  v="$(grep -E '^[[:space:]]*version:[[:space:]]*' "$skill_md" 2>/dev/null \
        | head -1 | sed -E 's/.*version:[[:space:]]*"?([^"#]*)"?.*/\1/' \
        | tr -d '[:space:]')"
  echo "${v:-dev}"
}

mkdir -p "$INSTALL_DIR"
MANIFEST="$INSTALL_DIR/okf-skills-manifest.txt"
{
  echo "# OKF skills installed by install.sh"
  echo "# columns: name<TAB>version<TAB>changelog"
} > "$MANIFEST"

echo "Installing OKF skills into $INSTALL_DIR"
count=0
for skill in $SKILLS; do
  src="$ROOT/skills/$skill"
  out="$INSTALL_DIR/$skill$EXT"
  ver="$(skillVersion "$src/SKILL.md")"
  echo "  building $skill $ver ..."
  ( cd "$src" && go build -ldflags "-X main.version=$ver" -o "$out" . )
  changelog="skills/$skill/CHANGELOG.md"
  [ -f "$ROOT/$changelog" ] || changelog="-"
  printf '%s\t%s\t%s\n' "$skill" "$ver" "$changelog" >> "$MANIFEST"
  count=$((count + 1))
done

# Build and install the okf-mcp server: the host that discovers and exposes the
# skills over MCP. It is NOT a skill itself, so it lives at the repo top level
# (not under skills/) and is recorded as a comment, not a skill row.
mcpver="$(skillVersion "$ROOT/okf-mcp/SKILL.md")"
echo "  building okf-mcp (server) $mcpver ..."
( cd "$ROOT/okf-mcp" && go build -ldflags "-X main.version=$mcpver" -o "$INSTALL_DIR/okf-mcp$EXT" . )
mcpchangelog="okf-mcp/CHANGELOG.md"
[ -f "$ROOT/$mcpchangelog" ] || mcpchangelog="-"
printf '# okf-mcp (server)\t%s\t%s\n' "$mcpver" "$mcpchangelog" >> "$MANIFEST"

echo "Installed $count skills + the okf-mcp server."
echo "Manifest: $MANIFEST"
echo "Next: ensure $INSTALL_DIR is on your PATH, then run 'okf-mcp' (or 'okf-mcp --skills-dir $INSTALL_DIR')."
