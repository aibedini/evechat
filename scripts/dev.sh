#!/usr/bin/env bash
# Local development: Go API, Vite dashboard and widget watcher.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ ! -f evechat.yaml ]; then
  echo "evechat.yaml is missing; copy evechat.example.yaml and set the secrets."
  exit 1
fi
if [ -z "${EVECHAT_ENCRYPTION_KEY_V1:-}" ]; then
  echo "EVECHAT_ENCRYPTION_KEY_V1 is not set."
  echo "Generate one with: openssl rand -base64 32"
  exit 1
fi

trap 'kill 0' EXIT
go run ./cmd/evechat --config evechat.yaml --seed &
(cd web/dashboard && npm run dev) &
(cd web/widget && npm run dev) &
wait
