#!/usr/bin/env bash
# Safe updater: backup -> download -> verify -> swap -> restart -> health check
# with automatic rollback. Prefer `evechat update`, which does the same in Go.
set -euo pipefail
cd "$(dirname "$0")/.."

DIR="${EVECHAT_DIR:-/opt/evechat}"
BIN="${EVECHAT_BIN:-$DIR/evechat}"
CFG="${EVECHAT_CONFIG:-$DIR/evechat.yaml}"
TARGET_VERSION="${1:-}"

info() { echo "▸ $*"; }
ok() { echo "✓ $*"; }
die() { echo "✗ $*" >&2; exit 1; }

[ -x "$BIN" ] || die "binary not found: $BIN"

if [ -n "$TARGET_VERSION" ]; then
  info "updating to $TARGET_VERSION"
  "$BIN" --config "$CFG" update --version "$TARGET_VERSION"
else
  "$BIN" --config "$CFG" update
fi
ok "update finished; health:"
"$BIN" --config "$CFG" healthcheck && ok "healthy"
