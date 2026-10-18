#!/usr/bin/env bash
# Full production build: widget -> dashboard -> setup wizard -> static binary.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-${1:-dev}}"

echo "==> fonts"
[ -f embed/assets/fonts/vazirmatn-variable.woff2 ] || { echo "missing fonts; run scripts/fetch-fonts.sh"; exit 1; }
FONT_BYTES=$(du -sb embed/assets/fonts 2>/dev/null | cut -f1 || du -sk embed/assets/fonts | awk '{print $1*1024}')
[ "$FONT_BYTES" -lt 400000 ] || { echo "fonts too big: $FONT_BYTES"; exit 1; }
echo "    fonts: ${FONT_BYTES} bytes"

echo "==> widget"
cd web/widget
npm ci --no-audit --no-fund
npm run build
SIZE=$(node -e "const z=require('zlib'),fs=require('fs');process.stdout.write(String(z.gzipSync(fs.readFileSync('dist/widget.js')).length))")
echo "    widget gzip: ${SIZE} bytes"
[ "$SIZE" -lt 16384 ] || { echo "widget too big: $SIZE (cap 16384)"; exit 1; }
cp dist/widget.js ../../embed/widget/widget.js
cd ../..

echo "==> dashboard"
cd web/dashboard
npm ci --no-audit --no-fund
npm run build
cd ../..

echo "==> setup wizard"
cd web/setup
npm ci --no-audit --no-fund
npm run build
cd ../..

echo "==> external URL audit"
node scripts/url-audit.mjs

echo "==> binary"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o evechat ./cmd/evechat
BINARY_BYTES=$(wc -c < evechat | tr -d ' ')
echo "    binary: ${BINARY_BYTES} bytes"
[ "$BINARY_BYTES" -lt 47185920 ] || { echo "binary too big"; exit 1; }
echo "==> built ./evechat"
