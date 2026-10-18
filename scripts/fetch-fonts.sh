#!/usr/bin/env bash
# Fetch Vazirmatn (OFL-1.1) without npm, for restricted networks.
#   scripts/fetch-fonts.sh [--mirror <base-url>]
# Verifies every file against scripts/fonts.sha256.
set -euo pipefail
cd "$(dirname "$0")/.."
DEST="embed/assets/fonts"
VERSION="33.0.3"
MIRROR="https://github.com/rastikerdar/vazirmatn/releases/download/v${VERSION}"
mkdir -p "$DEST"

while [ $# -gt 0 ]; do
  case "$1" in
    --mirror) MIRROR="$2"; shift 2 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

need() {
  local name="$1" url="$2"
  if [ -f "$DEST/$name" ]; then
    echo "✓ $name already present"
    return
  fi
  echo "▸ downloading $name"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 -o "$DEST/$name" "$url"
  else
    wget -q -O "$DEST/$name" "$url"
  fi
}

need vazirmatn-variable.woff2 "$MIRROR/Vazirmatn%5Bwght%5D.woff2"
need vazirmatn-regular.woff2  "$MIRROR/Vazirmatn-Regular.woff2"
need vazirmatn-bold.woff2     "$MIRROR/Vazirmatn-Bold.woff2"
need OFL.txt                  "$MIRROR/OFL.txt"

echo "▸ verifying checksums"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$DEST" && sha256sum -c ../../scripts/fonts.sha256)
else
  (cd "$DEST" && shasum -a 256 -c ../../scripts/fonts.sha256)
fi
echo "✓ fonts ready ($(du -sh "$DEST" | cut -f1))"
