#!/usr/bin/env bash
# Builds the full release matrix into dist/ without publishing.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:-dev}"
VER="${VERSION#v}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p dist stage

build() {
  local goos="$1" goarch="$2" label="$3" suffix="$4" extra="${5:-}"
  local name="evechat_${VER}_${goos}_${label}${suffix}"
  echo "==> ${name}"
  if [ -n "$extra" ]; then export "$extra"; fi
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o "dist/${name}" ./cmd/evechat
  rm -rf "stage/${name}"
  mkdir -p "stage/${name}"
  if [ "$goos" = "windows" ]; then
    cp "dist/${name}" "stage/${name}/evechat.exe"
  else
    cp "dist/${name}" "stage/${name}/evechat"
  fi
  cp evechat.example.yaml LICENSE README.md README.fa.md NOTICE "stage/${name}/"
  cp -r deploy "stage/${name}/deploy"
  if [ -n "$extra" ]; then unset "${extra%%=*}"; fi
  if [ "$goos" = "windows" ]; then
    (cd stage && zip -qr "../dist/${name}.zip" "${name}")
  else
    tar -C stage -czf "dist/${name}.tar.gz" "${name}"
    tar -tzf "dist/${name}.tar.gz" | grep -qxF "${name}/evechat"
  fi
  rm -rf "stage/${name}"
}

build linux amd64 amd64 ""
build linux arm64 arm64 ""
build linux arm armv7 "" "GOARM=7"
build darwin amd64 amd64 ""
build darwin arm64 arm64 ""
build windows amd64 amd64 ".exe"

cp embed/widget/widget.js dist/widget.js 2>/dev/null || true
(cd dist && sha256sum * > checksums.txt)
rm -rf stage
echo "==> dist/"
ls -lh dist
