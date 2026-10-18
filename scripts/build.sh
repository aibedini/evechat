#!/usr/bin/env bash
# Full production build: widget -> dashboard -> static binary.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-dev}"

echo "==> widget"
cd web/widget
npm ci
npm run build
SIZE=$(node -e "const z=require('zlib'),fs=require('fs');process.stdout.write(String(z.gzipSync(fs.readFileSync('dist/widget.js')).length))")
echo "    widget gzip: ${SIZE} bytes"
[ "$SIZE" -lt 12288 ] || { echo "widget too big: $SIZE"; exit 1; }
cp dist/widget.js ../../embed/widget/widget.js
cd ../..

echo "==> dashboard"
cd web/dashboard
npm ci
npm run build
cd ../..

echo "==> binary"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o evechat ./cmd/evechat
echo "==> built ./evechat ($(du -h evechat | cut -f1))"
