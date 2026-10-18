#!/usr/bin/env bash
# Cron/systemd friendly backup wrapper.
#   KEEP_DAYS=14 RCLONE_REMOTE=gdrive:evechat scripts/backup.sh
set -euo pipefail
cd "$(dirname "$0")/.."

DIR="${EVECHAT_DIR:-/opt/evechat}"
CFG="${EVECHAT_CONFIG:-$DIR/evechat.yaml}"
BIN="${EVECHAT_BIN:-$DIR/evechat}"
DEST="${EVECHAT_BACKUP_DIR:-/var/backups/evechat}"
KEEP_DAYS="${KEEP_DAYS:-14}"

if [ -f "$DIR/.env" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$DIR/.env"
  set +a
fi

mkdir -p "$DEST"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
BASE="$DEST/evechat-$STAMP"

log() { echo "$(date -u +%FT%TZ) $*"; }

if "$BIN" --config "$CFG" backup "$BASE" --with-uploads; then
  log "backup created: ${BASE}.tar.gz"
else
  log "backup FAILED"
  command -v logger >/dev/null 2>&1 && logger -t evechat-backup "backup failed"
  exit 1
fi

# Local retention.
find "$DEST" -type f \( -name '*.tar.gz' -o -name '*.db' -o -name '*.manifest.json' \) \
  -mtime "+${KEEP_DAYS}" -delete || true

# Optional offsite copy.
if [ -n "${RCLONE_REMOTE:-}" ] && command -v rclone >/dev/null 2>&1; then
  rclone copy "$BASE.tar.gz" "$RCLONE_REMOTE/" --quiet && log "copied to $RCLONE_REMOTE"
elif [ -n "${RSYNC_TARGET:-}" ] && command -v rsync >/dev/null 2>&1; then
  rsync -a "$BASE.tar.gz" "$RSYNC_TARGET/" && log "copied to $RSYNC_TARGET"
fi

command -v logger >/dev/null 2>&1 && logger -t evechat-backup "backup ok: ${BASE}.tar.gz"
log "done"
