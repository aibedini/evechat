#!/usr/bin/env bash
# Offline (air-gapped) installer. Extracts everything from this bundle; makes no
# network requests. Same flags as install.sh except --base-url is ignored.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
DIR="/opt/evechat"
PORT="3000"
PUBLIC_URL=""
NO_SYSTEMD="0"
SERVICE_USER="evechat"

while [ $# -gt 0 ]; do
  case "$1" in
    --dir) DIR="$2"; shift 2 ;;
    --port) PORT="$2"; shift 2 ;;
    --public-url) PUBLIC_URL="$2"; shift 2 ;;
    --no-systemd) NO_SYSTEMD="1"; shift ;;
    --user) SERVICE_USER="$2"; shift 2 ;;
    -h|--help) echo "usage: ./install-offline.sh [--dir DIR] [--port N] [--public-url URL] [--no-systemd]"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

[ "$(id -u)" -eq 0 ] || { echo "run with sudo" >&2; exit 1; }
BIN="$HERE/evechat"
[ -x "$BIN" ] || BIN="$HERE/bin/evechat"
[ -x "$BIN" ] || { echo "binary not found next to this script" >&2; exit 1; }

hostip="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "${hostip:-}" ] || hostip="127.0.0.1"
[ -n "$PUBLIC_URL" ] || PUBLIC_URL="http://${hostip}:${PORT}"

mkdir -p "$DIR/data" "$DIR/data/uploads"
install -m 0755 "$BIN" "$DIR/evechat"
[ -f "$HERE/evechat.example.yaml" ] && install -m 0644 "$HERE/evechat.example.yaml" "$DIR/evechat.example.yaml"
[ -f "$HERE/deploy/systemd/evechat.service" ] && install -m 0644 "$HERE/deploy/systemd/evechat.service" /tmp/evechat.service.offline || true

if ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER" 2>/dev/null || true
fi
chown -R "$SERVICE_USER:$SERVICE_USER" "$DIR/data"

if [ ! -f "$DIR/.env" ]; then
  umask 077
  {
    echo "# Offline install $(date -u +%FT%TZ)"
    echo "EVECHAT_AUTH__JWT_SECRET=$(openssl rand -base64 48)"
    echo "EVECHAT_ENCRYPTION_KEY_V1=$(openssl rand -base64 32)"
  } > "$DIR/.env"
  chmod 600 "$DIR/.env"
  chown "$SERVICE_USER:$SERVICE_USER" "$DIR/.env"
fi

if [ ! -f "$DIR/evechat.yaml" ]; then
  cat > "$DIR/evechat.yaml" <<EOF
server:
  host: "0.0.0.0"
  port: ${PORT}
  public_url: "${PUBLIC_URL}"
database:
  path: "${DIR}/data/evechat.db"
storage:
  uploads_dir: "${DIR}/data/uploads"
encryption:
  current_version: 1
log:
  level: "info"
EOF
  chown "$SERVICE_USER:$SERVICE_USER" "$DIR/evechat.yaml"
  chmod 0640 "$DIR/evechat.yaml"
fi

if [ "$NO_SYSTEMD" != "1" ] && command -v systemctl >/dev/null 2>&1; then
  if [ -f "$HERE/deploy/systemd/evechat.service" ]; then
    sed "s#/opt/evechat#${DIR}#g; s/^User=evechat/User=${SERVICE_USER}/; s/^Group=evechat/Group=${SERVICE_USER}/" \
      "$HERE/deploy/systemd/evechat.service" > /etc/systemd/system/evechat.service
  else
    cat > /etc/systemd/system/evechat.service <<EOF
[Unit]
Description=EveChat
After=network-online.target
[Service]
User=${SERVICE_USER}
WorkingDirectory=${DIR}
EnvironmentFile=${DIR}/.env
ExecStart=${DIR}/evechat --config ${DIR}/evechat.yaml serve
Restart=always
LimitNOFILE=65535
[Install]
WantedBy=multi-user.target
EOF
  fi
  systemctl daemon-reload
  systemctl enable evechat >/dev/null 2>&1 || true
  systemctl restart evechat
fi

set -a; . "$DIR/.env"; set +a
"$DIR/evechat" --config "$DIR/evechat.yaml" migrate >/dev/null
"$DIR/evechat" --config "$DIR/evechat.yaml" setup --non-interactive --print-credentials

echo
echo "EveChat offline install complete: $PUBLIC_URL"
echo "Run it manually with: $DIR/evechat --config $DIR/evechat.yaml serve"
