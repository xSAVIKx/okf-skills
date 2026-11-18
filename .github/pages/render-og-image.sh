#!/usr/bin/env bash
# Render og-image.svg to og-image.png (1200x630) with headless Chromium.
# Run this script again after you change the SVG, and commit both files.
# The pages workflow uses the committed PNG. It does not run this script.
set -euo pipefail
dir="$(cd "$(dirname "$0")" && pwd)"
chrome="${CHROME:-$(command -v chromium || command -v chromium-browser || command -v google-chrome)}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/page.html" <<HTML
<!DOCTYPE html><html><head><style>html,body{margin:0;background:#11151b}img{display:block}</style></head>
<body><img src="file://$dir/og-image.svg" width="1200" height="630"></body></html>
HTML
"$chrome" --headless --disable-gpu --no-sandbox --hide-scrollbars --allow-file-access-from-files \
  --force-device-scale-factor=1 --window-size=1200,630 \
  --screenshot="$dir/og-image.png" "file://$tmp/page.html"
