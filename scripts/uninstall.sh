#!/usr/bin/env bash
# Uninstall helper. Data and secrets are kept unless --purge is given and typed.
set -euo pipefail
DIR="${EVECHAT_DIR:-/opt/evechat}"
PURGE="0"
[ "${1:-}" = "--purge" ] && PURGE="1"

if [ "$(id -u)" -ne 0 ]; then
  echo "run with sudo" >&2
  exit 1
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop evechat 2>/dev/null || true
  systemctl disable evechat 2>/dev/null || true
  rm -f /etc/systemd/system/evechat.service /etc/systemd/system/evechat-backup.service /etc/systemd/system/evechat-backup.timer
  systemctl daemon-reload || true
fi

rm -f "$DIR/evechat"
echo "✓ binary removed"

if [ "$PURGE" = "1" ]; then
  echo "This deletes $DIR/data and $DIR/.env permanently."
  printf "Type PURGE to confirm: "
  read -r answer
  if [ "$answer" = "PURGE" ]; then
    rm -rf "$DIR/data" "$DIR/.env" "$DIR/evechat.yaml"
    echo "✓ data purged"
  else
    echo "! not confirmed; data kept"
  fi
else
  echo "✓ data kept: $DIR/data and $DIR/.env"
fi
