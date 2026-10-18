#!/usr/bin/env bash
# EveChat one-line installer.
#
#   curl -fsSL https://raw.githubusercontent.com/aibedini/evechat/main/install.sh | bash
#   curl -fsSL ... | bash -s -- --version v1.1.0 --dir /opt/evechat --port 8080
#   curl -fsSL ... | bash -s -- --no-systemd
#   curl -fsSL ... | bash -s -- --uninstall [--purge]
#
# Everything below runs as the installing user; system paths need root.
set -euo pipefail

REPO="aibedini/evechat"
DEFAULT_DIR="/opt/evechat"
DIR="${EVECHAT_DIR:-$DEFAULT_DIR}"
VERSION=""
PORT=""
PUBLIC_URL=""
BASE_URL="https://github.com/${REPO}/releases/download"
API_URL="https://api.github.com/repos/${REPO}/releases"
NO_SYSTEMD="0"
UNINSTALL="0"
PURGE="0"
SERVICE_USER="evechat"
SKIP_SETUP="0"
BIN=""
OS=""
ARCH=""
TMPDIR_INSTALL=""
KEEP_ENV="0"

# ---------- output helpers ----------
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
  C_RESET='\033[0m'; C_BOLD='\033[1m'; C_GREEN='\033[32m'; C_YELLOW='\033[33m'; C_RED='\033[31m'; C_DIM='\033[2m'
else
  C_RESET=''; C_BOLD=''; C_GREEN=''; C_YELLOW=''; C_RED=''; C_DIM=''
fi
info() { printf "%b\n" "${C_BOLD}▸${C_RESET} $*"; }
ok()   { printf "%b\n" "${C_GREEN}✓${C_RESET} $*"; }
warn() { printf "%b\n" "${C_YELLOW}!${C_RESET} $*" >&2; }
err()  { printf "%b\n" "${C_RED}✗${C_RESET} $*" >&2; }
die()  { err "$*"; exit 1; }

usage() {
  cat <<'EOF'
نصب EveChat

گزینه‌ها:
  --version <v>     نسخه مورد نظر (پیش‌فرض: آخرین انتشار)
  --dir <path>      مسیر نصب (پیش‌فرض /opt/evechat)
  --port <n>        پورت سرویس (پیش‌فرض 3000)
  --public-url <u>  آدرس عمومی (پیش‌فرض http://IP:PORT)
  --base-url <url>  آینه دانلود (برای شبکه‌های محدود)
  --user <name>     کاربر سرویس (پیش‌فرض evechat)
  --no-systemd      بدون نصب سرویس systemd
  --skip-setup      فقط باینری؛ ویزارد اجرا نمی‌شود
  --uninstall       حذف نصب (داده‌ها حفظ می‌شوند)
  --purge           همراه با --uninstall: حذف کامل داده‌ها (تأیید تایپی)
  -h, --help        همین راهنما
EOF
}

# ---------- argument parsing ----------
while [ $# -gt 0 ]; do
  case "$1" in
    --version)    VERSION="$2"; shift 2 ;;
    --version=*)  VERSION="${1#*=}"; shift ;;
    --dir)        DIR="$2"; shift 2 ;;
    --dir=*)      DIR="${1#*=}"; shift ;;
    --port)       PORT="$2"; shift 2 ;;
    --port=*)     PORT="${1#*=}"; shift ;;
    --public-url) PUBLIC_URL="$2"; shift 2 ;;
    --public-url=*) PUBLIC_URL="${1#*=}"; shift ;;
    --base-url)   BASE_URL="$2"; shift 2 ;;
    --base-url=*) BASE_URL="${1#*=}"; shift ;;
    --user)       SERVICE_USER="$2"; shift 2 ;;
    --no-systemd) NO_SYSTEMD="1"; shift ;;
    --skip-setup) SKIP_SETUP="1"; shift ;;
    --uninstall)  UNINSTALL="1"; shift ;;
    --purge)      PURGE="1"; shift ;;
    -h|--help)    usage; exit 0 ;;
    *) die "گزینه ناشناخته: $1 (--help را ببینید)" ;;
  esac
done

need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    die "این عملیات به دسترسی root نیاز دارد. با sudo اجرا کنید."
  fi
}

as_root() {
  if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo "$@"; fi
}

# ---------- environment checks ----------
require_tools() {
  local missing=0
  command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || { err "curl یا wget لازم است."; missing=1; }
  command -v tar >/dev/null 2>&1 || { err "tar لازم است."; missing=1; }
  if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    err "sha256sum یا shasum لازم است."
    missing=1
  fi
  if [ "$missing" -ne 0 ]; then
    err "نصب پیش‌نیازها:"
    err "  Debian/Ubuntu: sudo apt-get update && sudo apt-get install -y curl tar coreutils"
    err "  RHEL/Fedora:   sudo dnf install -y curl tar coreutils"
    err "  macOS:         brew install curl coreutils"
    exit 1
  fi
}

detect_platform() {
  case "$(uname -s)" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *) die "سیستم‌عامل پشتیبانی نمی‌شود: $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l|armv7) ARCH="armv7" ;;
    *) die "معماری پشتیبانی نمی‌شود: $(uname -m)" ;;
  esac
  ok "سیستم شناسایی شد: ${OS}/${ARCH}"
}

fetch() {
  local url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 --connect-timeout 15 -o "$out" "$url"
  else
    wget -q -O "$out" "$url"
  fi
}

fetch_stdout() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 2 --connect-timeout 10 "$url"
  else
    wget -q -O - "$url"
  fi
}

resolve_version() {
  if [ -n "$VERSION" ]; then
    ok "نسخه انتخاب‌شده: $VERSION"
    return
  fi
  info "دریافت آخرین نسخه از GitHub…"
  local json
  if json="$(fetch_stdout "${API_URL}/latest" 2>/dev/null)"; then
    VERSION="$(printf '%s' "$json" | tr ',{}' '\n' | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  fi
  if [ -z "${VERSION:-}" ]; then
    die "دریافت آخرین نسخه ناموفق بود (دسترسی به GitHub محدود است؟).
     از --version v1.1.0 و در صورت نیاز --base-url https://mirror.example.ir/evechat استفاده کنید."
  fi
  ok "آخرین نسخه: $VERSION"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

download_release() {
  local ver_no_v="${VERSION#v}"
  local archive="evechat_${ver_no_v}_${OS}_${ARCH}.tar.gz"
  local url="${BASE_URL}/${VERSION}/${archive}"
  local sums="${BASE_URL}/${VERSION}/checksums.txt"
  TMPDIR_INSTALL="$(mktemp -d)"
  info "دانلود ${archive}…"
  if ! fetch "$url" "${TMPDIR_INSTALL}/${archive}"; then
    die "دانلود ناموفق بود: $url
     برای نصب آفلاین از install-offline.sh یا --base-url استفاده کنید."
  fi
  if fetch "$sums" "${TMPDIR_INSTALL}/checksums.txt" 2>/dev/null; then
    local expected actual
    expected="$(awk -v f="$archive" '$2==f || $2=="*"f {print $1}' "${TMPDIR_INSTALL}/checksums.txt" | head -n1)"
    actual="$(sha256_of "${TMPDIR_INSTALL}/${archive}")"
    if [ -z "$expected" ]; then
      warn "checksum برای ${archive} در checksums.txt نبود؛ ادامه بدون بررسی."
    elif [ "$expected" != "$actual" ]; then
      die "checksum مطابقت ندارد! انتظار: $expected دریافت: $actual"
    else
      ok "checksum تأیید شد"
    fi
  else
    warn "checksums.txt در دسترس نبود؛ بررسی checksum انجام نشد."
  fi
  tar -xzf "${TMPDIR_INSTALL}/${archive}" -C "${TMPDIR_INSTALL}"
  BIN="$(find "${TMPDIR_INSTALL}" -maxdepth 2 -type f -name evechat | head -n1)"
  [ -n "$BIN" ] || die "باینری در آرشیو پیدا نشد"
  chmod +x "$BIN"
}

random_b64() { openssl rand -base64 "$1" 2>/dev/null || head -c "$1" /dev/urandom | base64; }

install_service_user() {
  if [ "$OS" = "darwin" ]; then return; fi
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    ok "کاربر سرویس موجود است: $SERVICE_USER"
    return
  fi
  need_root
  useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER" 2>/dev/null \
    || adduser -S -H -s /sbin/nologin "$SERVICE_USER" \
    || warn "ساخت کاربر $SERVICE_USER ناموفق بود؛ از root استفاده می‌شود."
  ok "کاربر سرویس ساخته شد: $SERVICE_USER"
}

install_files() {
  need_root
  mkdir -p "$DIR"
  install -m 0755 "$BIN" "$DIR/evechat"
  if [ -f "${TMPDIR_INSTALL}/evechat.example.yaml" ]; then
    install -m 0644 "${TMPDIR_INSTALL}/evechat.example.yaml" "$DIR/evechat.example.yaml"
  elif [ -f "$(dirname "$0")/evechat.example.yaml" ]; then
    install -m 0644 "$(dirname "$0")/evechat.example.yaml" "$DIR/evechat.example.yaml"
  fi
  mkdir -p "$DIR/data" "$DIR/data/uploads"
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    chown -R "$SERVICE_USER:$SERVICE_USER" "$DIR/data"
  fi
  chmod 0750 "$DIR/data"
  ok "باینری در $DIR/evechat نصب شد"
}

write_env() {
  local env_file="$DIR/.env"
  if [ -f "$env_file" ]; then
    ok "فایل .env موجود است و بازنویسی نمی‌شود"
    KEEP_ENV="1"
    return
  fi
  local jwt key
  jwt="$(random_b64 48)"
  key="$(random_b64 32)"
  umask 077
  cat > "$env_file" <<EOF
# EveChat — تولیدشده در $(date -u +%Y-%m-%dT%H:%M:%SZ)
# این فایل را خصوصی نگه دارید (chmod 600).
EVECHAT_AUTH__JWT_SECRET=${jwt}
EVECHAT_ENCRYPTION_KEY_V1=${key}
EOF
  chmod 600 "$env_file"
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then chown "$SERVICE_USER:$SERVICE_USER" "$env_file"; fi
  ok "کلیدهای امنیتی ساخته شدند: $env_file"
  warn "کلید رمزنگاری را جداگانه پشتیبان بگیرید؛ بدون آن پیام‌ها قابل بازیابی نیستند."
}

write_config() {
  local cfg="$DIR/evechat.yaml"
  if [ -f "$cfg" ]; then
    ok "فایل تنظیمات موجود است و بازنویسی نمی‌شود"
    return
  fi
  if [ -z "$PORT" ]; then PORT="3000"; fi
  if [ -z "$PUBLIC_URL" ]; then
    local ip
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    [ -n "${ip:-}" ] || ip="127.0.0.1"
    PUBLIC_URL="http://${ip}:${PORT}"
  fi
  cat > "$cfg" <<EOF
# EveChat — تولیدشده در $(date -u +%Y-%m-%dT%H:%M:%SZ)
server:
  host: "0.0.0.0"
  port: ${PORT}
  public_url: "${PUBLIC_URL}"
  trust_proxy: false
  shutdown_timeout: "20s"

database:
  path: "${DIR}/data/evechat.db"
  max_open_conns: 1
  max_read_conns: 4

storage:
  uploads_dir: "${DIR}/data/uploads"
  max_file_size_mb: 10

auth:
  access_expiry: "24h"
  refresh_expiry: "720h"
  bcrypt_cost: 12

encryption:
  current_version: 1

hub:
  ping_interval_seconds: 30
  pong_wait_seconds: 90
  send_buffer_size: 256
  ring_buffer_size: 50
  max_message_bytes: 32768

security:
  pow_difficulty: 16

metrics:
  enabled: false

log:
  level: "info"
  max_size_mb: 50
  max_backups: 5
EOF
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then chown "$SERVICE_USER:$SERVICE_USER" "$cfg"; fi
  chmod 0640 "$cfg"
  ok "تنظیمات نوشته شد: $cfg"
}

install_systemd() {
  if [ "$NO_SYSTEMD" = "1" ]; then
    ok "نصب systemd رد شد (--no-systemd)"
    return
  fi
  if [ "$OS" != "linux" ] || ! command -v systemctl >/dev/null 2>&1; then
    warn "systemd در دسترس نیست؛ اجرای دستی: $DIR/evechat --config $DIR/evechat.yaml serve"
    return
  fi
  need_root
  local unit="/etc/systemd/system/evechat.service"
  local src=""
  for candidate in "$(dirname "$0")/deploy/systemd/evechat.service" "${TMPDIR_INSTALL}/deploy/systemd/evechat.service"; do
    [ -f "$candidate" ] && src="$candidate" && break
  done
  if [ -n "$src" ]; then
    sed "s#/opt/evechat#${DIR}#g; s/^User=evechat/User=${SERVICE_USER}/; s/^Group=evechat/Group=${SERVICE_USER}/" "$src" > "$unit"
  else
    cat > "$unit" <<EOF
[Unit]
Description=EveChat live support server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
WorkingDirectory=${DIR}
EnvironmentFile=${DIR}/.env
ExecStartPre=${DIR}/evechat --config ${DIR}/evechat.yaml migrate
ExecStart=${DIR}/evechat --config ${DIR}/evechat.yaml serve
Restart=always
RestartSec=3
TimeoutStopSec=30
LimitNOFILE=65535
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DIR}/data

[Install]
WantedBy=multi-user.target
EOF
  fi
  systemctl daemon-reload
  systemctl enable evechat >/dev/null 2>&1 || true
  systemctl restart evechat
  ok "سرویس systemd نصب و اجرا شد"
}

wait_health() {
  local deadline=$(( $(date +%s) + 20 ))
  local probe
  probe="http://127.0.0.1:${PORT}/api/v1/health"
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if fetch_stdout "$probe" >/dev/null 2>&1; then
      ok "سرویس پاسخ می‌دهد"
      return 0
    fi
    sleep 1
  done
  err "سرویس در ۲۰ ثانیه پاسخ نداد."
  if command -v journalctl >/dev/null 2>&1; then
    journalctl -u evechat -n 30 --no-pager || true
  fi
  exit 1
}

run_setup() {
  if [ "$SKIP_SETUP" = "1" ]; then return; fi
  local output
  set +e
  output="$(set -a; . "$DIR/.env"; set +a; "$DIR/evechat" --config "$DIR/evechat.yaml" setup --non-interactive --json 2>&1)"
  local code=$?
  set -e
  if [ $code -ne 0 ]; then
    warn "ویزارد نصب خودکار ناموفق بود:"
    printf '%s\n' "$output" >&2
    return
  fi
  printf '%s' "$output" > "$DIR/setup-result.json"
  local email password key dash
  email="$(printf '%s' "$output" | sed -n 's/.*"admin_email":"\([^"]*\)".*/\1/p')"
  password="$(printf '%s' "$output" | sed -n 's/.*"admin_password":"\([^"]*\)".*/\1/p')"
  key="$(printf '%s' "$output" | sed -n 's/.*"project_key":"\([^"]*\)".*/\1/p')"
  dash="$(printf '%s' "$output" | sed -n 's/.*"dashboard_url":"\([^"]*\)".*/\1/p')"
  [ -n "$dash" ] || dash="$PUBLIC_URL"

  local snippet="$DIR/embed-snippet.html"
  if [ ! -f "$snippet" ]; then
    cat > "$snippet" <<EOF
<script>
  window.EveChat = { projectKey: "${key}", apiUrl: "${dash}" };
</script>
<script async src="${dash}/widget.js"></script>
EOF
  fi
  cat <<EOF

╔══════════════════════════════════════════════════════════╗
║  EveChat نصب شد ✓                                        ║
╠══════════════════════════════════════════════════════════╣
║  پنل:        ${dash}
║  ایمیل:      ${email}
║  رمز عبور:   ${password}
║  کلید پروژه: ${key}
╚══════════════════════════════════════════════════════════╝
کد embed در فایل ${snippet} ذخیره شد.
EOF
}

do_uninstall() {
  need_root
  info "حذف نصب EveChat از ${DIR}"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop evechat 2>/dev/null || true
    systemctl disable evechat 2>/dev/null || true
    rm -f /etc/systemd/system/evechat.service
    systemctl daemon-reload 2>/dev/null || true
  fi
  rm -f "${DIR}/evechat"
  ok "باینری و سرویس حذف شدند"
  if [ "$PURGE" = "1" ]; then
    warn "این عمل داده‌ها و کلیدها را برای همیشه حذف می‌کند."
    printf "برای تأیید عبارت PURGE را تایپ کنید: "
    read -r answer
    if [ "$answer" = "PURGE" ]; then
      rm -rf "${DIR}/data" "${DIR}/.env" "${DIR}/evechat.yaml"
      ok "داده‌ها حذف شدند"
    else
      warn "تأیید نشد؛ داده‌ها حفظ شدند."
    fi
  else
    ok "داده‌ها حفظ شدند: ${DIR}/data و ${DIR}/.env"
  fi
}

main() {
  require_tools
  detect_platform
  if [ "$UNINSTALL" = "1" ]; then
    do_uninstall
    exit 0
  fi
  resolve_version
  download_release
  install_service_user
  install_files
  write_env
  write_config
  install_systemd
  if [ "$NO_SYSTEMD" != "1" ] && command -v systemctl >/dev/null 2>&1; then
    wait_health
  else
    "$DIR/evechat" --config "$DIR/evechat.yaml" migrate >/dev/null 2>&1 || die "مهاجرت‌ها ناموفق بودند"
  fi
  run_setup
  [ -n "${TMPDIR_INSTALL:-}" ] && rm -rf "$TMPDIR_INSTALL" || true
}

main "$@"
