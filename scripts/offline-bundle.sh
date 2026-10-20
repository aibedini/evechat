#!/usr/bin/env bash
# Builds the air-gapped install bundle.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:-dev}"
NAME="evechat-offline-${VERSION}-linux-amd64"
rm -rf "dist/${NAME}"
mkdir -p "dist/${NAME}/deploy"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X main.version=${VERSION}" -o "dist/${NAME}/evechat" ./cmd/evechat

cp evechat.example.yaml LICENSE README.fa.md NOTICE "dist/${NAME}/"
cp install.sh install-offline.sh "dist/${NAME}/"
cp -r deploy/systemd deploy/caddy deploy/nginx "dist/${NAME}/deploy/"
cp -r embed/assets/fonts "dist/${NAME}/fonts"
(cd "dist/${NAME}" && sha256sum evechat > checksums.txt)

tar -C dist -czf "dist/${NAME}.tar.gz" "${NAME}"
rm -rf "dist/${NAME}"
echo "==> dist/${NAME}.tar.gz"
ls -lh "dist/${NAME}.tar.gz"
