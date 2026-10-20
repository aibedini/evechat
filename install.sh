#!/usr/bin/env bash
# =============================================================================
#  EveChat installer
#
#  Interactive (recommended):
#    curl -fsSL https://raw.githubusercontent.com/aibedini/evechat/main/install.sh | sudo bash
#
#  Unattended:
#    curl -fsSL .../install.sh | sudo bash -s -- --yes --behind-proxy --domain chat.example.com --write-nginx
#
#  Design rules:
#    * Everything is in English.
#    * A TUI is shown when a terminal is available; flags still work in CI.
#    * Existing web servers (nginx/Apache/Caddy), EVE, Redis, Docker and every
#      other service are observed, never stopped or reconfigured.
#    * An nginx vhost is written to a dedicated file, validated with nginx -t,
#      and rolled back if nginx rejects it.
#    * Prompts read from /dev/tty on a dedicated fd, so `curl | bash` works.
# =============================================================================
# nginx variables such as $host are intentionally emitted literally.
# shellcheck disable=SC2016
set -Eeuo pipefail

INSTALLER_VERSION="2.0.0"
APP_NAME="EveChat"
REPO="aibedini/evechat"
GITHUB="https://github.com/${REPO}"
RELEASES_URL="${GITHUB}/releases"
API_URL="https://api.github.com/repos/${REPO}/releases"
DEFAULT_DIR="/opt/evechat"
DEFAULT_PORT=3000
DEFAULT_SERVICE_USER="evechat"
SERVICE_NAME="evechat"

DIR="${EVECHAT_DIR:-$DEFAULT_DIR}"
PORT="${EVECHAT_PORT:-}"
PUBLIC_URL="${EVECHAT_PUBLIC_URL:-}"
DOMAIN="${EVECHAT_DOMAIN:-}"
VERSION="${EVECHAT_VERSION:-}"
BASE_URL="${EVECHAT_BASE_URL:-${GITHUB}/releases/download}"
SERVICE_USER="${EVECHAT_SERVICE_USER:-$DEFAULT_SERVICE_USER}"
SERVICE_GROUP="$DEFAULT_SERVICE_USER"
NGINX_CONF="${EVECHAT_NGINX_CONF:-}"

ACTION=""
ASSUME_YES=0
NO_TUI=0
FORCE=0
PURGE=0
SKIP_SETUP=0
NO_SYSTEMD=0
WRITE_NGINX=""
NET_MODE=""
NET_MODE_EXPLICIT=0
PORT_SET=0
DIR_SET=0
USER_SET=0
PUBLIC_URL_SET=0
[ -z "$PORT" ] || PORT_SET=1
[ -z "${EVECHAT_DIR:-}" ] || DIR_SET=1
[ -z "${EVECHAT_SERVICE_USER:-}" ] || USER_SET=1
ADMIN_NAME=""
ADMIN_EMAIL=""
ADMIN_PASSWORD=""
HOST="0.0.0.0"
TRUST="false"
CERT_FILE=""
KEY_FILE=""
CERT_EXPIRY=""
CFG_BACKUP=""

OS=""
ARCH=""
TTY_FD=""
UI_OUT_TTY=0
UI_WIDTH=80
UI_UTF8=0
INTERACTIVE=0
PROMPT=0
COLOR_ENABLED=0
C_RESET=""
C_BOLD=""
C_DIM=""
C_ACCENT=""
C_TEXT=""
C_MUTED=""
C_OK=""
C_WARN=""
C_ERR=""
G_OK="OK"
G_ERR="x"
G_WARN="!"
G_ARROW=">"
G_RULE="-"
G_VERT="|"
G_BL="+"
G_DASH="-"
G_UP="Up"
G_DOWN="Down"

TMPDIR_INSTALL=""
RELEASE_BIN=""
STAGE_TOTAL=8
CHANGED_USER=""
CHANGED_SYSTEMD=""
CHANGED_NGINX=""
SETUP_FAILED=0
ALREADY_CONFIGURED=0
SETUP_EMAIL=""
SETUP_PASSWORD=""
SETUP_KEY=""
SETUP_URL=""
NGINX_FAILED=0
# -----------------------------------------------------------------------------
#  Output helpers
# -----------------------------------------------------------------------------
repeat_char() {
  local ch="$1" n="$2" out=""
  while [ "$n" -gt 0 ]; do
    out="${out}${ch}"
    n=$((n - 1))
  done
  printf '%s' "$out"
}

setup_colors() {
  local colors="8"
  C_RESET=""
  C_BOLD=""
  C_DIM=""
  C_ACCENT=""
  C_TEXT=""
  C_MUTED=""
  C_OK=""
  C_WARN=""
  C_ERR=""
  if [ "$COLOR_ENABLED" = 1 ]; then
    colors="$(tput colors 2>/dev/null || printf '8')"
    case "$colors" in
      ''|*[!0-9]*) colors=8 ;;
    esac
    if [ "$colors" -ge 256 ]; then
      C_ACCENT=$'\033[38;5;45m'
      C_TEXT=$'\033[38;5;252m'
      C_MUTED=$'\033[38;5;244m'
      C_OK=$'\033[38;5;42m'
      C_WARN=$'\033[38;5;214m'
      C_ERR=$'\033[38;5;203m'
    else
      C_ACCENT=$'\033[36m'
      C_TEXT=$'\033[37m'
      C_MUTED=$'\033[90m'
      C_OK=$'\033[32m'
      C_WARN=$'\033[33m'
      C_ERR=$'\033[31m'
    fi
    C_RESET=$'\033[0m'
    C_BOLD=$'\033[1m'
    C_DIM=$'\033[2m'
  fi
}

setup_glyphs() {
  G_OK="OK"
  G_ERR="x"
  G_WARN="!"
  G_ARROW=">"
  G_RULE="-"
  G_VERT="|"
  G_BL="+"
  G_DASH="-"
  G_UP="Up"
  G_DOWN="Down"
  if [ "$UI_UTF8" = 1 ]; then
    G_OK="✓"
    G_ERR="✗"
    G_WARN="!"
    G_ARROW="▸"
    G_RULE="─"
    G_VERT="│"
    G_BL="╰"
    G_DASH="—"
    G_UP="↑"
    G_DOWN="↓"
  fi
}

ui_init() {
  local cols="${COLUMNS:-}"
  if [ "$UI_OUT_TTY" = 1 ]; then
    if [ -z "$cols" ]; then
      cols="$(tput cols 2>/dev/null || true)"
    fi
  fi
  case "$cols" in
    ''|*[!0-9]*) cols=80 ;;
  esac
  [ "$cols" -ge 48 ] || cols=48
  [ "$cols" -le 110 ] || cols=110
  UI_WIDTH="$cols"

  UI_UTF8=0
  if locale charmap 2>/dev/null | tr '[:upper:]' '[:lower:]' | grep -Eq 'utf-?8'; then
    UI_UTF8=1
  fi
  setup_colors
  setup_glyphs
}

ui_clear() {
  if [ "$UI_OUT_TTY" = 1 ] && [ "$NO_TUI" != 1 ]; then
    printf '\033[H\033[2J'
  fi
}

ui_blank() { printf '\n'; }

ui_rule() {
  printf '  %s%s%s\n' "$C_DIM" "$(repeat_char "$G_RULE" $((UI_WIDTH - 4)))" "$C_RESET"
}

ui_banner() {
  if [ "$UI_UTF8" = 1 ] && [ "$UI_WIDTH" -ge 70 ]; then
    printf '  %s███████╗██╗   ██╗███████╗ ██████╗██╗  ██╗ █████╗ ████████╗%s\n' "$C_ACCENT" "$C_RESET"
    printf '  %s██╔════╝██║   ██║██╔════╝██╔════╝██║  ██║██╔══██╗╚══██╔══╝%s\n' "$C_ACCENT" "$C_RESET"
    printf '  %s█████╗  ██║   ██║█████╗  ██║     ███████║███████║   ██║%s\n' "$C_ACCENT" "$C_RESET"
    printf '  %s██╔══╝  ╚██╗ ██╔╝██╔══╝  ██║     ██╔══██║██╔══██║   ██║%s\n' "$C_ACCENT" "$C_RESET"
    printf '  %s███████╗ ╚████╔╝ ███████╗╚██████╗██║  ██║██║  ██║   ██║%s\n' "$C_ACCENT" "$C_RESET"
    printf '  %s╚══════╝  ╚═══╝  ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝%s\n' "$C_ACCENT" "$C_RESET"
    printf '  %sSelf-hosted customer support  %s·%s  interactive installer %s%s\n' "$C_MUTED" "$C_DIM" "$C_MUTED" "$INSTALLER_VERSION" "$C_RESET"
  else
    printf '\n  %sE V E C H A T%s\n' "$C_BOLD$C_ACCENT" "$C_RESET"
    printf '  %sSelf-hosted customer support  ·  installer %s%s\n' "$C_MUTED" "$INSTALLER_VERSION" "$C_RESET"
  fi
  ui_rule
}

ui_screen() {
  local title="$1" subtitle="${2:-}"
  ui_clear
  printf '\n  %s%s%s %s·%s %s%s%s\n' "$C_BOLD" "$APP_NAME" "$C_RESET" "$C_DIM" "$C_RESET" "$C_BOLD" "$title" "$C_RESET"
  [ -z "$subtitle" ] || printf '  %s%s%s\n' "$C_MUTED" "$subtitle" "$C_RESET"
  ui_rule
}

ui_heading() {
  printf '\n  %s%s%s\n' "$C_BOLD" "$1" "$C_RESET"
  printf '  %s%s%s\n' "$C_DIM" "$(repeat_char "$G_RULE" $((UI_WIDTH - 4)))" "$C_RESET"
}

ui_panel_open() {
  printf '\n  %s%s%s\n' "$C_ACCENT$C_BOLD" "$1" "$C_RESET"
}

ui_panel_close() {
  printf '  %s%s%s%s\n' "$C_DIM" "$G_BL" "$(repeat_char "$G_RULE" $((UI_WIDTH - 5)))" "$C_RESET"
}

ui_clip() {
  local s="$1" max=$((UI_WIDTH - 22)) tail="…"
  [ "$max" -ge 16 ] || max=16
  [ "$UI_UTF8" = 1 ] || tail="..."
  if [ "${#s}" -gt "$max" ]; then
    printf '%s%s' "${s:0:$((max - 1))}" "$tail"
  else
    printf '%s' "$s"
  fi
}

ui_kv() {
  local label="$1" value="${2:-$G_DASH}"
  [ -n "$value" ] || value="$G_DASH"
  printf '  %s%s%s %s%-16s%s %s\n' "$C_DIM" "$G_VERT" "$C_RESET" "$C_MUTED" "$label" "$C_RESET" "$(ui_clip "$value")"
}

ui_note() {
  printf '  %s%s%s\n' "$C_MUTED" "$*" "$C_RESET"
}

ui_ok() {
  printf '  %s%s%s  %s\n' "$C_OK" "$G_OK" "$C_RESET" "$*"
}

ui_warn() {
  printf '  %s%s%s  %s\n' "$C_WARN" "$G_WARN" "$C_RESET" "$*"
}

ui_err() {
  printf '  %s%s%s  %s\n' "$C_ERR" "$G_ERR" "$C_RESET" "$*" >&2
}

ui_step() {
  printf '  %s%s%s  %s\n' "$C_ACCENT" "$G_ARROW" "$C_RESET" "$*"
}

ui_stage() {
  printf '\n  %s[%s/%s]%s %s\n' "$C_MUTED" "$1" "$2" "$C_RESET" "$3"
}

ui_show_cursor() {
  if [ "$UI_OUT_TTY" = 1 ] && [ "$NO_TUI" != 1 ]; then
    printf '\033[?25h'
  fi
}

ui_hide_cursor() {
  if [ "$UI_OUT_TTY" = 1 ] && [ "$NO_TUI" != 1 ]; then
    printf '\033[?25l'
  fi
}

ui_footer() {
  local max="${1:-6}"
  if [ "$UI_UTF8" = 1 ]; then
    printf '\n  %s%s/%s or j/k move  ·  Enter select  ·  1-%s jump  ·  q quit%s\n' "$C_MUTED" "$G_UP" "$G_DOWN" "$max" "$C_RESET"
  else
    printf '\n  %sUp/Down or j/k move  ·  Enter select  ·  1-%s jump  ·  q quit%s\n' "$C_MUTED" "$max" "$C_RESET"
  fi
}

ui_menu_item() {
  local selected="$1" number="$2" label="$3" hint="$4"
  if [ "$selected" = 1 ]; then
    if [ "$UI_WIDTH" -ge 72 ]; then
      printf '  %s%s%s %s%d  %-30s%s %s%s%s\n' "$C_ACCENT" "$G_ARROW" "$C_RESET" "$C_ACCENT$C_BOLD" "$number" "$label" "$C_RESET" "$C_MUTED" "$hint" "$C_RESET"
    else
      printf '  %s%s%s %s%d  %s%s\n' "$C_ACCENT" "$G_ARROW" "$C_RESET" "$C_ACCENT$C_BOLD" "$number" "$label" "$C_RESET"
    fi
  else
    if [ "$UI_WIDTH" -ge 72 ]; then
      printf '    %s%d  %-30s%s %s%s%s\n' "$C_MUTED" "$number" "$label" "$C_RESET" "$C_DIM" "$hint" "$C_RESET"
    else
      printf '    %s%d  %s%s\n' "$C_MUTED" "$number" "$label" "$C_RESET"
    fi
  fi
}

die() {
  ui_err "$*"
  exit 1
}

usage() {
  cat <<'EOF'
EveChat installer 2.0

Usage:
  install.sh                    interactive menu
  install.sh --install          guided install
  install.sh --upgrade          replace the binary, keep data
  install.sh --nginx            write an nginx vhost for this install
  install.sh --status           show service, port, TLS and disk status
  install.sh --logs             show recent service logs
  install.sh --uninstall        remove the service and binary, keep data
  install.sh --uninstall --purge  also delete data and secrets (typed confirm)

Common options:
  --version <v>          release to install (default: latest)
  --dir <path>           install directory (default: /opt/evechat)
  --port <n>             service port (default: 3000, auto-picks a free one)
  --public-url <url>     public dashboard URL
  --behind-proxy         bind 127.0.0.1 and trust the local reverse proxy
  --direct               bind 0.0.0.0 and serve the port directly
  --local                bind 127.0.0.1 only
  --domain <name>        public domain, for example chat.example.com
  --write-nginx          write an nginx vhost (implies --behind-proxy)
  --no-nginx             never write an nginx vhost
  --nginx-conf <path>    explicit nginx vhost path
  --user <name>          service user (default: evechat)
  --no-systemd           do not install a systemd unit
  --skip-setup           install the binary and open /setup in the browser
  --admin-name <name>    administrator name for the first-run setup
  --admin-email <email>  administrator email for the first-run setup
  --admin-password <pw>  administrator password (default: generated)
  --base-url <url>       download mirror, .../releases/download by default
  --yes, -y              never ask (unattended)
  --force                reinstall even when the same version is installed
  --no-tui               plain output, no full-screen menu
  --debug                shell trace
  -h, --help             show this help

Examples:
  curl -fsSL .../install.sh | sudo bash
  curl -fsSL .../install.sh | sudo bash -s -- --yes --behind-proxy --domain chat.example.com --write-nginx
  sudo ./install.sh --port 3100 --behind-proxy --domain chat.example.com --write-nginx

Safety:
  * Port 3000 is used only if free; otherwise the installer suggests 3100+.
  * Ports 80/443 are detected and left to nginx/Apache/Caddy.
  * A vhost is written only to a file owned by this installer, only after
    nginx -t succeeds; a failed test restores the previous state.
  * Other services and their configuration are never modified.
EOF
}

# -----------------------------------------------------------------------------
#  Terminal input
# -----------------------------------------------------------------------------
open_tty() {
  TTY_FD=""
  if [ -t 0 ]; then
    TTY_FD=0
  elif [ -e /dev/tty ] && [ -r /dev/tty ]; then
    { exec 3</dev/tty; } 2>/dev/null || true
    if [ -t 3 ]; then
      TTY_FD=3
    fi
  fi
}

restore_echo() {
  case "${TTY_FD:-}" in
    0) stty echo 2>/dev/null || true ;;
    3) stty echo <&3 2>/dev/null || true ;;
  esac
}

read_input() {
  local __var="$1" __line=""
  if [ "$TTY_FD" = 0 ]; then
    IFS= read -r __line || true
  elif [ "$TTY_FD" = 3 ]; then
    IFS= read -r -u 3 __line || true
  fi
  printf -v "$__var" '%s' "$__line"
}

read_secret() {
  local __var="$1" __line=""
  case "${TTY_FD:-}" in
    0) stty -echo 2>/dev/null || true ;;
    3) stty -echo <&3 2>/dev/null || true ;;
  esac
  read_input __line
  restore_echo
  printf '\n'
  printf -v "$__var" '%s' "$__line"
}

tui_read_key() {
  local k="" rest="" tmo="0.15"
  if [ "${BASH_VERSINFO[0]:-3}" -lt 4 ]; then
    tmo="1"
  fi
  TUI_KEY=""
  if [ -z "$TTY_FD" ]; then
    TUI_KEY="quit"
    return 0
  fi
  if [ "$TTY_FD" = 0 ]; then
    IFS= read -rsn1 k || { TUI_KEY="quit"; return 0; }
  else
    IFS= read -rsn1 -u 3 k || { TUI_KEY="quit"; return 0; }
  fi
  case "$k" in
    $'\033')
      if [ "$TTY_FD" = 0 ]; then
        IFS= read -rsn2 -t "$tmo" rest || rest=""
      else
        IFS= read -rsn2 -t "$tmo" -u 3 rest || rest=""
      fi
      case "$rest" in
        '[A'|'OA') TUI_KEY="up" ;;
        '[B'|'OB') TUI_KEY="down" ;;
        *) TUI_KEY="quit" ;;
      esac
      ;;
    '')
      TUI_KEY="enter"
      ;;
    *)
      TUI_KEY="$k"
      ;;
  esac
}

ask_value() {
  local __var="$1" __label="$2" __default="${3:-}" __answer=""
  if [ "$PROMPT" != 1 ]; then
    printf -v "$__var" '%s' "$__default"
    return 0
  fi
  ui_blank
  printf '  %s%s%s\n' "$C_TEXT" "$__label" "$C_RESET"
  if [ -n "$__default" ]; then
    printf '  %s[%s]%s ' "$C_MUTED" "$__default" "$C_RESET"
  else
    printf '  %s>%s ' "$C_ACCENT" "$C_RESET"
  fi
  read_input __answer
  [ -n "$__answer" ] || __answer="$__default"
  printf -v "$__var" '%s' "$__answer"
}

ask_secret_value() {
  local __var="$1" __label="$2" __answer=""
  if [ "$PROMPT" != 1 ]; then
    printf -v "$__var" '%s' ""
    return 0
  fi
  ui_blank
  printf '  %s%s%s\n' "$C_TEXT" "$__label" "$C_RESET"
  printf '  %s>%s ' "$C_ACCENT" "$C_RESET"
  read_secret __answer
  printf -v "$__var" '%s' "$__answer"
}

confirm_yes() {
  local __label="$1" __default="${2:-y}" __answer="" __hint="[Y/n]"
  [ "$__default" = y ] || __hint="[y/N]"
  if [ "$PROMPT" != 1 ]; then
    [ "$__default" = y ]
    return $?
  fi
  printf '\n  %s%s%s %s%s%s ' "$C_TEXT" "$__label" "$C_RESET" "$C_MUTED" "$__hint" "$C_RESET"
  read_input __answer
  if [ -z "$__answer" ]; then
    __answer="$__default"
  fi
  case "$__answer" in
    y|Y|yes|YES|Yes) return 0 ;;
    n|N|no|NO|No) return 1 ;;
    *) [ "$__default" = y ]; return $? ;;
  esac
}

ui_pause() {
  local __ignore=""
  [ "$PROMPT" = 1 ] || return 0
  printf '\n  %sPress Enter to continue%s ' "$C_MUTED" "$C_RESET"
  read_input __ignore
}

# -----------------------------------------------------------------------------
#  Menu
# -----------------------------------------------------------------------------
SCREEN_TITLE="EVECHAT"
SCREEN_SUB=""
MENU_INDEX=0
MENU_CANCELLED=0
MENU_START=0
TUI_KEY=""

menu_pick() {
  local title="$1"
  shift
  local -a labels=()
  local -a hints=()
  local count=0 selected="${MENU_START:-0}" i=0

  while [ $# -ge 2 ]; do
    labels+=("$1")
    hints+=("$2")
    shift 2
  done
  count="${#labels[@]}"
  MENU_INDEX=0
  MENU_CANCELLED=0

  if [ "$count" -eq 0 ]; then
    return 1
  fi
  [ "$selected" -lt "$count" ] || selected=0
  if [ "$PROMPT" != 1 ]; then
    MENU_INDEX=0
    return 0
  fi

  ui_hide_cursor
  while :; do
    ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
    ui_heading "$title"
    i=0
    while [ "$i" -lt "$count" ]; do
      if [ "$i" -eq "$selected" ]; then
        ui_menu_item 1 "$((i + 1))" "${labels[$i]}" "${hints[$i]}"
      else
        ui_menu_item 0 "$((i + 1))" "${labels[$i]}" "${hints[$i]}"
      fi
      i=$((i + 1))
    done
    ui_footer "$count"
    tui_read_key
    case "$TUI_KEY" in
      up|k|K)
        selected=$(((selected + count - 1) % count))
        ;;
      down|j|J)
        selected=$(((selected + 1) % count))
        ;;
      enter)
        break
        ;;
      q|Q)
        MENU_CANCELLED=1
        ui_show_cursor
        return 0
        ;;
      [1-9])
        if [ "$TUI_KEY" -le "$count" ]; then
          selected=$((TUI_KEY - 1))
          break
        fi
        ;;
      *)
        ;;
    esac
  done
  ui_show_cursor
  MENU_INDEX="$selected"
  return 0
}

render_main_menu() {
  local selected="$1"
  local -a labels=(
    "Install EveChat"
    "Upgrade"
    "Reverse proxy (nginx)"
    "Health & status"
    "Uninstall"
    "Exit"
  )
  local -a hints=(
    "guided setup · about 2 minutes"
    "latest release · keeps data"
    "write a validated vhost"
    "service, port, TLS and disk"
    "remove service · keeps data"
    "leave everything as it is"
  )
  local i=0

  ui_clear
  ui_banner

  ui_panel_open "THIS SERVER"
  ui_kv "Operating system" "$ENV_OS"
  ui_kv "Host" "$ENV_HOST"
  ui_kv "Web server" "$ENV_WEB"
  [ -z "$ENV_NEIGHBORS" ] || ui_kv "Also running" "$ENV_NEIGHBORS"
  ui_kv "EveChat" "$ENV_INSTALL"
  ui_kv "Listen" "$ENV_LISTEN"
  ui_panel_close

  ui_heading "WHAT WOULD YOU LIKE TO DO?"
  while [ "$i" -lt 6 ]; do
    if [ "$i" -eq "$selected" ]; then
      ui_menu_item 1 "$((i + 1))" "${labels[$i]}" "${hints[$i]}"
    else
      ui_menu_item 0 "$((i + 1))" "${labels[$i]}" "${hints[$i]}"
    fi
    i=$((i + 1))
  done
  if [ "$(id -u)" -ne 0 ]; then
    ui_blank
    ui_warn "Changes need root. Re-run with: sudo bash install.sh"
  fi
  ui_footer 6
}

main_menu() {
  local selected=0
  while :; do
    detect_environment
    render_main_menu "$selected"
    tui_read_key
    case "$TUI_KEY" in
      up|k|K) selected=$(((selected + 5) % 6)) ;;
      down|j|J) selected=$(((selected + 1) % 6)) ;;
      enter) ;;
      [1-6]) selected=$((TUI_KEY - 1)) ;;
      q|Q) ui_clear; exit 0 ;;
      *) continue ;;
    esac
    case "$selected" in
      0|1|2|4)
        if [ "$(id -u)" -ne 0 ]; then
          ui_warn "This action needs root. Re-run with: sudo bash install.sh"
          ui_pause
          continue
        fi
        ;;
    esac
    case "$selected" in
      0) flow_install || true ;;
      1) flow_upgrade || true ;;
      2) flow_proxy || true ;;
      3) flow_status || true ;;
      4) flow_uninstall || true ;;
      5) ui_clear; exit 0 ;;
    esac
    ui_pause
  done
}

# -----------------------------------------------------------------------------
#  Platform and environment probes (read-only)
# -----------------------------------------------------------------------------
platform_detect() {
  if [ -n "${EVECHAT_TEST_PLATFORM:-}" ]; then
    OS="${EVECHAT_TEST_PLATFORM%%/*}"
    ARCH="${EVECHAT_TEST_PLATFORM##*/}"
    return 0
  fi
  case "$(uname -s 2>/dev/null || true)" in
    Linux) OS="linux" ;;
    Darwin) OS="darwin" ;;
    *) die "Unsupported operating system: $(uname -s 2>/dev/null || echo unknown). Use Docker or install.ps1 on Windows." ;;
  esac
  case "$(uname -m 2>/dev/null || true)" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l|armv7) ARCH="armv7" ;;
    *) die "Unsupported architecture: $(uname -m 2>/dev/null || echo unknown)." ;;
  esac
}

pretty_os() {
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    ( . /etc/os-release 2>/dev/null; printf '%s' "${PRETTY_NAME:-Linux}" )
  elif [ "$OS" = "darwin" ]; then
    printf 'macOS %s' "$(sw_vers -productVersion 2>/dev/null || echo unknown)"
  else
    printf '%s' "$(uname -s -r 2>/dev/null || echo unknown)"
  fi
}

cpu_count() {
  if command -v nproc >/dev/null 2>&1; then
    nproc 2>/dev/null || printf '1'
  elif command -v getconf >/dev/null 2>&1; then
    getconf _NPROCESSORS_ONLN 2>/dev/null || printf '1'
  else
    sysctl -n hw.ncpu 2>/dev/null || printf '1'
  fi
}

mem_summary() {
  if [ -r /proc/meminfo ]; then
    awk '/^MemTotal:/ {t=$2} /^MemAvailable:/ {a=$2} /^MemFree:/ {f=$2} END {
      if (a == "") a = f
      if (t > 0) printf "%.1f GB RAM · %.1f GB available", t/1048576, a/1048576
      else printf "memory unknown"
    }' /proc/meminfo
  elif [ "$OS" = "darwin" ]; then
    sysctl -n hw.memsize 2>/dev/null | awk '{printf "%.1f GB RAM", $1/1073741824}'
  else
    printf 'memory unknown'
  fi
}

disk_summary() {
  local target="/"
  [ -d "$DIR" ] && target="$DIR"
  df -Pm "$target" 2>/dev/null | awk 'NR==2 && $4 ~ /^[0-9]+$/ {printf "%.1f GB disk", $4/1024}'
}

primary_ip() {
  local ip=""
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [ -z "$ip" ] && command -v ip >/dev/null 2>&1; then
    ip="$(ip route get 1.1.1.1 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i == "src") {print $(i+1); exit}}')"
  fi
  if [ -z "$ip" ] && [ "$OS" = "darwin" ] && command -v ipconfig >/dev/null 2>&1; then
    ip="$(ipconfig getifaddr en0 2>/dev/null || true)"
  fi
  [ -n "$ip" ] || ip="127.0.0.1"
  printf '%s' "$ip"
}

port_taken() {
  local port="$1" hex
  if command -v ss >/dev/null 2>&1; then
    if ss -ltnH 2>/dev/null | awk '{print $4}' | grep -Eq "[:.]${port}$"; then
      return 0
    fi
  fi
  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | tail -n +2 | grep -q .; then
      return 0
    fi
  fi
  hex="$(printf '%04X' "$port" 2>/dev/null || true)"
  if [ -n "$hex" ] && [ -r /proc/net/tcp ]; then
    if awk -v p=":${hex}" '$2 ~ p"$" && $4 == "0A" {found=1} END {exit found ? 0 : 1}' /proc/net/tcp 2>/dev/null; then
      return 0
    fi
  fi
  if [ -n "$hex" ] && [ -r /proc/net/tcp6 ]; then
    if awk -v p=":${hex}" '$2 ~ p"$" && $4 == "0A" {found=1} END {exit found ? 0 : 1}' /proc/net/tcp6 2>/dev/null; then
      return 0
    fi
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -z 127.0.0.1 "$port" >/dev/null 2>&1 && return 0
  fi
  return 1
}

port_owner() {
  local port="$1" line="" name=""
  if command -v ss >/dev/null 2>&1; then
    line="$(ss -ltnp 2>/dev/null | awk -v p=":${port}" '$4 ~ p"$" {print; exit}')"
  fi
  if [ -z "$line" ] && command -v lsof >/dev/null 2>&1; then
    line="$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null | awk 'NR==2 {print $1 " (pid " $2 ")"; exit}')"
  fi
  if [ -n "$line" ]; then
    name="$(printf '%s' "$line" | sed -n 's/.*users:(("\([^"]*\)".*/\1/p')"
    if [ -n "$name" ]; then
      printf '%s' "$name"
      return 0
    fi
    printf '%s' "$(printf '%s' "$line" | awk '{print $1}')"
    return 0
  fi
  printf 'another process'
}

first_free_port() {
  local p=3100
  while [ "$p" -le 3199 ]; do
    if ! port_taken "$p"; then
      printf '%s' "$p"
      return 0
    fi
    p=$((p + 1))
  done
  return 1
}

http_health() {
  local port="$1" url="http://127.0.0.1:${1}/api/v1/health"
  if command -v curl >/dev/null 2>&1; then
    curl -fsS --max-time 3 --connect-timeout 2 "$url" >/dev/null 2>&1
  elif command -v wget >/dev/null 2>&1; then
    wget -q -T 3 -O /dev/null "$url" >/dev/null 2>&1
  else
    return 1
  fi
}

systemd_available() {
  [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1
}

service_state() {
  if ! systemd_available; then
    printf 'manual'
    return 0
  fi
  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    printf 'running'
  else
    printf 'stopped'
  fi
}

service_enabled() {
  systemd_available || { printf 'no'; return 0; }
  if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
    printf 'yes'
  else
    printf 'no'
  fi
}

nginx_installed() {
  command -v nginx >/dev/null 2>&1
}

nginx_version() {
  nginx -v 2>&1 | sed -n 's#.*nginx/\([0-9][0-9.]*\).*#\1#p' | head -n1
}

web_server_summary() {
  local out="" v=""
  if nginx_installed; then
    v="$(nginx_version)"
    out="nginx${v:+ ${v}}"
  elif command -v apache2 >/dev/null 2>&1; then
    out="apache2"
  elif command -v httpd >/dev/null 2>&1; then
    out="httpd"
  elif command -v caddy >/dev/null 2>&1; then
    out="caddy"
  fi
  if [ -n "$out" ]; then
    if port_taken 80 || port_taken 443; then
      out="${out} · listening on 80/443"
    else
      out="${out} · not listening"
    fi
  else
    out="none detected"
  fi
  printf '%s' "$out"
}

neighbor_summary() {
  local out=""
  if command -v pgrep >/dev/null 2>&1; then
    pgrep -x redis-server >/dev/null 2>&1 && out="${out:+$out, }Redis"
    pgrep -x dockerd >/dev/null 2>&1 && out="${out:+$out, }Docker"
    pgrep -x mysqld >/dev/null 2>&1 && out="${out:+$out, }MySQL"
    pgrep -x postgres >/dev/null 2>&1 && out="${out:+$out, }PostgreSQL"
  fi
  if systemd_available; then
    if systemctl list-units --type=service --state=running --no-legend 2>/dev/null \
      | awk '{print $1}' | grep -Ev '^evechat' | grep -Eq '(^|[-_.])eve([-_.]|\.service$)'; then
      out="${out:+$out, }EVE"
    fi
  fi
  printf '%s' "$out"
}

version_of_binary() {
  if [ -x "$DIR/evechat" ]; then
    "$DIR/evechat" --version 2>/dev/null | head -n1 | awk '{print $2}' || true
  fi
}

detect_environment() {
  [ -n "$OS" ] || platform_detect
  local cfg="$DIR/evechat.yaml" p="" h=""
  ENV_OS="$(pretty_os) (${OS}/${ARCH})"
  ENV_HOST="$(cpu_count) vCPU · $(mem_summary) · $(disk_summary)"
  ENV_WEB="$(web_server_summary)"
  ENV_NEIGHBORS="$(neighbor_summary)"

  if [ -f "$cfg" ]; then
    p="$(cfg_get "$cfg" port)"
    h="$(cfg_get "$cfg" host)"
    PUBLIC_URL="$(cfg_get "$cfg" public_url)"
  fi
  [ -n "$p" ] || p="$DEFAULT_PORT"
  case "$p" in
    ''|*[!0-9]*) p="$DEFAULT_PORT" ;;
  esac

  if is_installed; then
    local v="" s=""
    v="$(version_of_binary)"
    s="$(service_state)"
    ENV_INSTALL="${v:-unknown} · ${s}"
  elif [ -x "$DIR/evechat" ] || [ -f "$DIR/evechat.yaml" ] || [ -f "$DIR/.evechat-install" ]; then
    ENV_INSTALL="incomplete install"
  else
    ENV_INSTALL="not installed"
  fi

  if port_taken "$p"; then
    if http_health "$p"; then
      ENV_LISTEN="127.0.0.1:${p} · EveChat"
    else
      ENV_LISTEN="${p} · used by $(port_owner "$p")"
    fi
  else
    ENV_LISTEN="${p} · free"
  fi
}

# -----------------------------------------------------------------------------
#  Configuration helpers
# -----------------------------------------------------------------------------
discover_install_dir() {
  [ "$DIR_SET" = 1 ] && return 0
  [ -x "$DIR/evechat" ] && return 0
  [ -f "$DIR/evechat.yaml" ] && return 0

  local unit="/etc/systemd/system/${SERVICE_NAME}.service" exec="" candidate=""
  if [ -f "$unit" ]; then
    exec="$(sed -n 's/^ExecStart=//p' "$unit" | head -n1)"
    case "$exec" in
      *" --config "*) candidate="${exec%% --config *}" ;;
    esac
    if [ -n "$candidate" ]; then
      candidate="$(dirname "$candidate")"
      if [ -f "$candidate/evechat.yaml" ] || [ -x "$candidate/evechat" ]; then
        DIR="$candidate"
        return 0
      fi
    fi
  fi
  candidate="$(command -v evechat 2>/dev/null || true)"
  if [ -n "$candidate" ]; then
    DIR="$(dirname "$candidate")"
  fi
}

is_installed() {
  [ -x "$DIR/evechat" ] && [ -f "$DIR/evechat.yaml" ]
}

cfg_get() {
  local file="$1" key="$2"
  [ -f "$file" ] || return 0
  awk -v key="$key" '
    /^[[:space:]]*#/ { next }
    /^[^[:space:]]/ { in_server = ($0 ~ /^server:[[:space:]]*$/); next }
    in_server && $1 == key":" {
      sub(/^[^:]*:[[:space:]]*/, "")
      sub(/[[:space:]]*#.*$/, "")
      gsub(/^["'\'']|["'\'']$/, "")
      print
      exit
    }
  ' "$file"
}

url_host() {
  printf '%s' "$1" | sed -E 's#^[a-zA-Z][a-zA-Z0-9+.-]*://##; s#/.*$##; s#:[0-9]+$##'
}

unit_belongs() {
  local unit="/etc/systemd/system/${SERVICE_NAME}.service"
  [ -f "$unit" ] || return 1
  grep -qF "Managed by EveChat installer" "$unit" && return 0
  grep -qF "ExecStart=${DIR}/evechat " "$unit" && return 0
  return 1
}

unit_service_user() {
  local unit="/etc/systemd/system/${SERVICE_NAME}.service" user=""
  [ -f "$unit" ] || return 0
  user="$(sed -n 's/^User=//p' "$unit" | head -n1)"
  printf '%s' "$user"
}

unit_service_group() {
  local unit="/etc/systemd/system/${SERVICE_NAME}.service" group=""
  [ -f "$unit" ] || return 0
  group="$(sed -n 's/^Group=//p' "$unit" | head -n1)"
  printf '%s' "$group"
}

load_existing() {
  discover_install_dir
  if ! is_installed; then
    ui_err "No EveChat installation was found in ${DIR}."
    ui_note "Use --dir to point at an existing install, or choose Install."
    return 1
  fi
  if [ ! -f "$DIR/evechat.yaml" ]; then
    ui_err "Missing configuration file: ${DIR}/evechat.yaml"
    return 1
  fi

  local p="" h="" t="" u="" had_url=0
  p="$(cfg_get "$DIR/evechat.yaml" port)"
  h="$(cfg_get "$DIR/evechat.yaml" host)"
  t="$(cfg_get "$DIR/evechat.yaml" trust_proxy)"
  u="$(cfg_get "$DIR/evechat.yaml" public_url)"
  [ -n "$u" ] && had_url=1
  [ -n "$p" ] || p="$DEFAULT_PORT"
  [ -n "$h" ] || h="0.0.0.0"
  [ -n "$t" ] || t="false"
  [ -n "$u" ] || u="http://127.0.0.1:${p}"

  PORT="$p"
  HOST="$h"
  TRUST="$t"
  PUBLIC_URL="${u%/}"
  PUBLIC_URL_SET="$had_url"
  DOMAIN="$(url_host "$PUBLIC_URL")"
  [ -n "$DOMAIN" ] || DOMAIN=""

  local unit_user="" unit_group=""
  unit_user="$(unit_service_user)"
  unit_group="$(unit_service_group)"
  if [ -n "$unit_user" ] && [ "$USER_SET" != 1 ]; then
    SERVICE_USER="$unit_user"
  fi
  if [ -n "$unit_group" ]; then
    SERVICE_GROUP="$unit_group"
  else
    SERVICE_GROUP="$(id -gn "$SERVICE_USER" 2>/dev/null || printf '%s' "$SERVICE_USER")"
  fi
  if [ "$TRUST" = true ] || [ "$HOST" = "127.0.0.1" ]; then
    NET_MODE="proxy"
  else
    NET_MODE="direct"
  fi
  return 0
}

random_b64() {
  local n="$1"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 "$n" 2>/dev/null | tr -d '\n'
  else
    head -c "$n" /dev/urandom | base64 | tr -d '\n'
  fi
}

ensure_env_key() {
  local file="$1" key="$2" value="$3" current="" tmp=""
  [ -f "$file" ] || return 0
  current="$(sed -n "s/^${key}=//p" "$file" | head -n1)"
  [ -n "$current" ] && return 0
  tmp="$(mktemp "${file}.XXXXXX")"
  awk -v k="$key" -v v="$value" '
    index($0, k "=") == 1 { print k "=" v; found=1; next }
    { print }
    END { if (!found) print k "=" v }
  ' "$file" > "$tmp"
  cat "$tmp" > "$file"
  rm -f "$tmp"
}

write_env() {
  local env_file="$DIR/.env" jwt="" key="" backup=""
  if [ -f "$env_file" ]; then
    backup="${env_file}.bak.$(date +%s)"
    cp -p "$env_file" "$backup" 2>/dev/null || true
    ensure_env_key "$env_file" "EVECHAT_AUTH__JWT_SECRET" "$(random_b64 48)"
    ensure_env_key "$env_file" "EVECHAT_ENCRYPTION_KEY_V1" "$(random_b64 32)"
    chmod 600 "$env_file"
    if id -u "$SERVICE_USER" >/dev/null 2>&1; then
      chown "$SERVICE_USER" "$env_file" 2>/dev/null || true
    fi
    ui_ok "Secrets kept in ${env_file}"
    return 0
  fi

  jwt="$(random_b64 48)"
  key="$(random_b64 32)"
  old_umask="$(umask)"
  umask 077
  {
    printf '# EveChat — generated by install.sh on %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf '# Keep this file private (chmod 600). Back it up separately.\n'
    printf 'EVECHAT_AUTH__JWT_SECRET=%s\n' "$jwt"
    printf 'EVECHAT_ENCRYPTION_KEY_V1=%s\n' "$key"
  } > "$env_file"
  umask "$old_umask"
  chmod 600 "$env_file"
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    chown "$SERVICE_USER" "$env_file" 2>/dev/null || true
  fi
  ui_ok "Security keys generated in ${env_file}"
}

write_config() {
  local cfg="$DIR/evechat.yaml"
  if [ -f "$cfg" ]; then
    local old_port=""
    old_port="$(cfg_get "$cfg" port)"
    if [ -n "$old_port" ] && [ "$old_port" != "$PORT" ]; then
      ui_warn "The existing config listens on ${old_port}; keeping that port."
      PORT="$old_port"
    fi
    chmod 0640 "$cfg"
    if id -g "$SERVICE_USER" >/dev/null 2>&1; then
      chown "root:${SERVICE_GROUP}" "$cfg" 2>/dev/null || true
    fi
    ui_ok "Configuration kept: ${cfg}"
    return 0
  fi

  old_umask="$(umask)"
  umask 027
  {
    printf '# EveChat — generated by install.sh on %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'server:\n'
    printf '  host: "%s"\n' "$HOST"
    printf '  port: %s\n' "$PORT"
    printf '  public_url: "%s"\n' "$PUBLIC_URL"
    printf '  trust_proxy: %s\n' "$TRUST"
    if [ "$TRUST" = true ]; then
      printf '  trusted_proxies:\n'
      printf '    - "127.0.0.1/32"\n'
      printf '    - "::1/128"\n'
    fi
    printf '  shutdown_timeout: "20s"\n'
    printf '\n'
    printf 'database:\n'
    printf '  path: "%s/data/evechat.db"\n' "$DIR"
    printf '  max_open_conns: 1\n'
    printf '  max_read_conns: 4\n'
    printf '\n'
    printf 'storage:\n'
    printf '  uploads_dir: "%s/data/uploads"\n' "$DIR"
    printf '  max_file_size_mb: 10\n'
    printf '\n'
    printf 'auth:\n'
    printf '  access_expiry: "24h"\n'
    printf '  refresh_expiry: "720h"\n'
    printf '  bcrypt_cost: 12\n'
    printf '\n'
    printf 'encryption:\n'
    printf '  current_version: 1\n'
    printf '\n'
    printf 'hub:\n'
    printf '  ping_interval_seconds: 30\n'
    printf '  pong_wait_seconds: 90\n'
    printf '  send_buffer_size: 256\n'
    printf '  ring_buffer_size: 50\n'
    printf '  max_message_bytes: 32768\n'
    printf '\n'
    printf 'security:\n'
    printf '  pow_difficulty: 16\n'
    printf '\n'
    printf 'metrics:\n'
    printf '  enabled: false\n'
    printf '\n'
    printf 'log:\n'
    printf '  level: "info"\n'
    printf '  max_size_mb: 50\n'
    printf '  max_backups: 5\n'
  } > "$cfg"
  umask "$old_umask"
  chmod 0640 "$cfg"
  if id -g "$SERVICE_USER" >/dev/null 2>&1; then
    chown "root:${SERVICE_GROUP}" "$cfg" 2>/dev/null || true
  fi
  ui_ok "Configuration written: ${cfg}"
}

cfg_set_server() {
  local cfg="$1" host="$2" port="$3" url="$4" trust="$5" tmp=""
  [ -f "$cfg" ] || return 1
  CFG_BACKUP="${cfg}.bak.$(date +%s)"
  cp -p "$cfg" "$CFG_BACKUP"
  tmp="$(mktemp "${cfg}.XXXXXX")"
  awk -v host="$host" -v port="$port" -v url="$url" -v trust="$trust" '
    function flush_missing() {
      if (!h) print "  host: \"" host "\""
      if (!p) print "  port: " port
      if (!u) print "  public_url: \"" url "\""
      if (!t) print "  trust_proxy: " trust
    }
    /^server:[[:space:]]*$/ { in_server=1; print; next }
    in_server && /^[^[:space:]#]/ { flush_missing(); in_server=0; h=p=u=t=0 }
    in_server && /^[[:space:]]+host:[[:space:]]/ { print "  host: \"" host "\""; h=1; next }
    in_server && /^[[:space:]]+port:[[:space:]]/ { print "  port: " port; p=1; next }
    in_server && /^[[:space:]]+public_url:[[:space:]]/ { print "  public_url: \"" url "\""; u=1; next }
    in_server && /^[[:space:]]+trust_proxy:[[:space:]]/ { print "  trust_proxy: " trust; t=1; next }
    { print }
    END { if (in_server) flush_missing() }
  ' "$cfg" > "$tmp"
  cat "$tmp" > "$cfg"
  rm -f "$tmp"
  chmod 0640 "$cfg"
  if id -g "$SERVICE_USER" >/dev/null 2>&1; then
    chown "root:${SERVICE_GROUP}" "$cfg" 2>/dev/null || true
  fi
  return 0
}

write_marker() {
  {
    printf '# EveChat installation marker. Do not edit.\n'
    printf 'installer=%s\n' "$INSTALLER_VERSION"
    printf 'version=%s\n' "${VERSION:-unknown}"
    printf 'installed_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'service=%s\n' "$SERVICE_NAME"
  } > "$DIR/.evechat-install"
  chmod 0644 "$DIR/.evechat-install"
}

# -----------------------------------------------------------------------------
#  Release download
# -----------------------------------------------------------------------------
fetch_stdout() {
  local url="$1" timeout="${2:-20}"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 2 --connect-timeout 10 --max-time "$timeout" "$url"
  else
    wget -q -T "$timeout" -O - "$url"
  fi
}

fetch_file() {
  local url="$1" out="$2" mode="${3:-}"
  if command -v curl >/dev/null 2>&1; then
    if [ "$UI_OUT_TTY" = 1 ] && [ "$NO_TUI" != 1 ] && [ "$mode" != "quiet" ]; then
      curl -fL --retry 3 --connect-timeout 15 --progress-bar -o "$out" "$url"
    else
      curl -fsSL --retry 3 --connect-timeout 15 -o "$out" "$url"
    fi
  else
    wget -q -O "$out" "$url"
  fi
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    VERSION=""
  fi
  if [ -n "$VERSION" ]; then
    case "$VERSION" in
      v*) ;;
      V[0-9]*) VERSION="v${VERSION#V}" ;;
      [0-9]*) VERSION="v${VERSION}" ;;
    esac
    case "$VERSION" in
      *[!A-Za-z0-9._+-]*) die "Invalid release version: ${VERSION}" ;;
    esac
    ui_ok "Release selected: ${VERSION}"
    return 0
  fi

  ui_step "Resolving the latest release"
  local tag="" json="" final=""
  if json="$(fetch_stdout "${API_URL}/latest" 15 2>/dev/null)"; then
    tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  fi
  if [ -z "$tag" ] && command -v curl >/dev/null 2>&1; then
    final="$(curl -fsSL --max-time 15 -o /dev/null -w '%{url_effective}' "${RELEASES_URL}/latest" 2>/dev/null || true)"
    tag="${final##*/}"
    case "$tag" in
      v[0-9]*) ;;
      *) tag="" ;;
    esac
  fi
  if [ -z "$tag" ]; then
    die "Could not resolve the latest release. Check network access, or pass --version v1.2.6 / --base-url <mirror>."
  fi
  VERSION="$tag"
  ui_ok "Latest release: ${VERSION}"
}

find_release_binary() {
  local root="$1" cand="" desc="" size=0
  local -a candidates=()

  for cand in "$root/evechat" "$root"/*/evechat; do
    [ -f "$cand" ] && candidates+=("$cand")
  done

  if [ "${#candidates[@]}" -eq 0 ]; then
    while IFS= read -r cand; do
      [ -f "$cand" ] || continue
      case "$cand" in
        *.tar.gz|*.tgz|*.zip|*.txt|*.md|*.yaml|*.yml|*.json|*.service|*.timer|*.conf) continue ;;
      esac
      case "$cand" in
        */deploy/*|*/fonts/*) continue ;;
      esac
      candidates+=("$cand")
    done < <(find "$root" -type f 2>/dev/null)
  fi

  for cand in ${candidates[@]+"${candidates[@]}"}; do
    size="$(wc -c < "$cand" 2>/dev/null | tr -d ' ')"
    case "$size" in
      ''|*[!0-9]*) size=0 ;;
    esac
    [ "$size" -ge 500000 ] || continue
    desc="$(file -b "$cand" 2>/dev/null || true)"
    case "$desc" in
      *ELF*|*Mach-O*|*PE32*) ;;
      '') ;;
      *) continue ;;
    esac
    RELEASE_BIN="$cand"
    return 0
  done

  return 1
}

validate_release_binary() {
  local bin="$1" out="" desc=""
  chmod +x "$bin" 2>/dev/null || true
  desc="$(file -b "$bin" 2>/dev/null || true)"
  if ! out="$("$bin" --version 2>&1)"; then
    ui_err "The downloaded binary cannot run on this server (${OS}/${ARCH})."
    [ -z "$desc" ] || ui_note "Detected file type: ${desc}"
    ui_note "Check --version and --base-url, then try again."
    return 1
  fi
  case "$out" in
    *evechat*|*EveChat*) ;;
    *)
      ui_err "The downloaded file does not look like the EveChat binary."
      return 1
      ;;
  esac
  ui_ok "Binary verified: $(printf '%s' "$out" | head -n1)"
  return 0
}

download_release() {
  local ver="${VERSION#v}"
  local archive="evechat_${ver}_${OS}_${ARCH}.tar.gz"
  local url="${BASE_URL}/${VERSION}/${archive}"
  local github_url="${GITHUB}/releases/download/${VERSION}/${archive}"
  local used_base="$BASE_URL" downloaded=0
  local checksums="" expected="" actual="" test_out=""

  if [ -z "$VERSION" ]; then
    die "Internal error: version is not resolved."
  fi
  TMPDIR_INSTALL="$(mktemp -d)" || die "Could not create a temporary directory."
  ui_step "Downloading ${archive}"

  if fetch_file "$url" "${TMPDIR_INSTALL}/${archive}"; then
    downloaded=1
  elif [ "$BASE_URL" != "${GITHUB}/releases/download" ] && fetch_file "$github_url" "${TMPDIR_INSTALL}/${archive}"; then
    used_base="${GITHUB}/releases/download"
    downloaded=1
  fi
  if [ "$downloaded" != 1 ]; then
    die "Download failed: ${url}
     Try --version or --base-url <mirror>; offline installs use install-offline.sh."
  fi

  if fetch_file "${used_base}/${VERSION}/checksums.txt" "${TMPDIR_INSTALL}/checksums.txt" quiet 2>/dev/null; then
    checksums="${TMPDIR_INSTALL}/checksums.txt"
    expected="$(tr -d '\r' < "$checksums" | awk -v f="$archive" '
      { n=$2; sub(/^\*/, "", n); sub(/^\.\//, "", n); if (n == f) { print $1; exit } }
    ')"
    actual="$(sha256_of "${TMPDIR_INSTALL}/${archive}")"
    if [ -z "$expected" ]; then
      ui_warn "No checksum for ${archive} in checksums.txt; continuing without a match."
    elif [ "$expected" != "$actual" ]; then
      die "Checksum mismatch for ${archive}.
     expected: ${expected}
     actual:   ${actual}"
    else
      ui_ok "SHA-256 checksum verified"
    fi
  else
    ui_warn "checksums.txt is unavailable; install integrity was not verified."
  fi

  if ! test_out="$(tar -tzf "${TMPDIR_INSTALL}/${archive}" 2>&1 >/dev/null)"; then
    die "The downloaded archive is not a valid .tar.gz:
     ${test_out}"
  fi
  if ! tar -xzf "${TMPDIR_INSTALL}/${archive}" -C "${TMPDIR_INSTALL}"; then
    die "Could not extract ${archive}."
  fi
  if ! find_release_binary "$TMPDIR_INSTALL"; then
    ui_err "No EveChat binary was found inside ${archive}."
    ui_note "Archive contents:"
    tar -tzf "${TMPDIR_INSTALL}/${archive}" 2>/dev/null | head -n 20 | sed 's/^/    /' >&2
    return 1
  fi
  validate_release_binary "$RELEASE_BIN"
}

# -----------------------------------------------------------------------------
#  Service account, files and systemd
# -----------------------------------------------------------------------------
install_service_user() {
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    SERVICE_GROUP="$(id -gn "$SERVICE_USER" 2>/dev/null || printf '%s' "$SERVICE_USER")"
    ui_ok "Service user exists: ${SERVICE_USER}"
    return 0
  fi
  if [ "$OS" = "darwin" ]; then
    SERVICE_USER="$(id -un)"
    SERVICE_GROUP="$(id -gn "$SERVICE_USER" 2>/dev/null || printf '%s' "$SERVICE_USER")"
    return 0
  fi

  local shell="/usr/sbin/nologin"
  [ -x "$shell" ] || shell="/sbin/nologin"
  [ -x "$shell" ] || shell="/bin/false"
  if useradd --system --no-create-home --home-dir "$DIR" --shell "$shell" "$SERVICE_USER" 2>/dev/null; then
    :
  elif adduser -S -H -s "$shell" "$SERVICE_USER" 2>/dev/null; then
    :
  else
    die "Could not create the service user '${SERVICE_USER}'. Check /etc/passwd permissions."
  fi
  SERVICE_GROUP="$(id -gn "$SERVICE_USER" 2>/dev/null || printf '%s' "$SERVICE_USER")"
  CHANGED_USER="$SERVICE_USER"
  ui_ok "Service user created: ${SERVICE_USER}"
}

install_files() {
  local src_deploy="" candidate=""
  install -d -m 0755 "$DIR"
  if [ -f "$DIR/evechat" ]; then
    cp -p "$DIR/evechat" "$DIR/evechat.prev" 2>/dev/null || true
  fi
  install -m 0755 "$RELEASE_BIN" "$DIR/.evechat.new"
  mv -f "$DIR/.evechat.new" "$DIR/evechat"

  if [ -f "${TMPDIR_INSTALL}/evechat.example.yaml" ]; then
    install -m 0644 "${TMPDIR_INSTALL}/evechat.example.yaml" "$DIR/evechat.example.yaml"
  else
    for candidate in "${TMPDIR_INSTALL}"/*/evechat.example.yaml; do
      [ -f "$candidate" ] || continue
      install -m 0644 "$candidate" "$DIR/evechat.example.yaml"
      break
    done
  fi

  for candidate in "${TMPDIR_INSTALL}"/deploy "${TMPDIR_INSTALL}"/*/deploy; do
    if [ -d "$candidate" ]; then
      src_deploy="$candidate"
      break
    fi
  done
  if [ -n "$src_deploy" ]; then
    install -d -m 0755 "$DIR/deploy"
    cp -R "${src_deploy}/." "$DIR/deploy/" 2>/dev/null || true
  fi

  install -d -m 0750 "$DIR/data" "$DIR/data/uploads"
  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    chown -R "$SERVICE_USER" "$DIR/data" 2>/dev/null || true
  fi
  chmod 0750 "$DIR/data"
  ui_ok "Binary installed in ${DIR}/evechat"
}

run_as_service() {
  if [ "$(id -u)" -eq 0 ] && command -v runuser >/dev/null 2>&1; then
    runuser -u "$SERVICE_USER" -- "$@"
  elif [ "$(id -u)" -eq 0 ] && command -v sudo >/dev/null 2>&1; then
    sudo -u "$SERVICE_USER" -- "$@"
  else
    "$@"
  fi
}

run_migrations() {
  local env_file="$DIR/.env" cfg="$DIR/evechat.yaml"
  if [ ! -f "$env_file" ]; then
    die "Missing environment file: ${env_file}"
  fi
  # shellcheck disable=SC1090
  if ! ( set -a; . "$env_file"; set +a; run_as_service "$DIR/evechat" --config "$cfg" migrate ); then
    die "Database migration failed."
  fi
  ui_ok "Database schema is up to date"
}

install_systemd() {
  if [ "$NO_SYSTEMD" = 1 ] || ! systemd_available; then
    NO_SYSTEMD=1
    ui_warn "systemd is not available; no service was installed."
    run_migrations
    return 0
  fi

  local unit="/etc/systemd/system/${SERVICE_NAME}.service" protect_home="true"
  case "$DIR" in
    /home/*|/root/*) protect_home="false" ;;
  esac
  if [ -f "$unit" ] && ! unit_belongs; then
    if [ "$FORCE" = 1 ]; then
      cp -p "$unit" "${unit}.bak.$(date +%s)"
    else
      die "Another unit already exists at ${unit}.
     Refusing to overwrite a service this installer did not create.
     Move it away, or re-run with --force."
    fi
  fi

  cat > "$unit" <<EOF
# Managed by EveChat installer (install.sh) — do not edit by hand.
[Unit]
Description=EveChat live support server
Documentation=${GITHUB}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${DIR}
EnvironmentFile=${DIR}/.env
ExecStartPre=${DIR}/evechat --config ${DIR}/evechat.yaml migrate
ExecStart=${DIR}/evechat --config ${DIR}/evechat.yaml serve
Restart=always
RestartSec=3
TimeoutStopSec=30
KillSignal=SIGTERM
LimitNOFILE=65535
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectSystem=strict
ProtectHome=${protect_home}
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
ReadWritePaths=${DIR}/data

[Install]
WantedBy=multi-user.target
EOF
  chmod 0644 "$unit"
  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || ui_warn "Could not enable ${SERVICE_NAME} at boot."
  if ! systemctl restart "$SERVICE_NAME"; then
    ui_err "The service failed to start."
    systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null | sed 's/^/    /' >&2 || true
    journalctl -u "$SERVICE_NAME" -n 30 --no-pager 2>/dev/null | sed 's/^/    /' >&2 || true
    return 1
  fi
  CHANGED_SYSTEMD="${unit}"
  ui_ok "systemd service installed and started"
  return 0
}

wait_health() {
  local deadline=$(( $(date +%s) + 30 ))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if http_health "$PORT"; then
      ui_ok "Health check passed on 127.0.0.1:${PORT}"
      return 0
    fi
    sleep 1
  done
  ui_err "EveChat did not answer on 127.0.0.1:${PORT} within 30 seconds."
  if systemd_available; then
    systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null | sed 's/^/    /' >&2 || true
    journalctl -u "$SERVICE_NAME" -n 40 --no-pager 2>/dev/null | sed 's/^/    /' >&2 || true
  fi
  return 1
}

json_field() {
  local field="$1" json="$2"
  printf '%s' "$json" | sed -n 's/.*"'"$field"'":"\([^"]*\)".*/\1/p' | head -n1
}

run_setup() {
  if [ "$SKIP_SETUP" = 1 ]; then
    SETUP_URL="${PUBLIC_URL}/setup"
    ui_warn "First-run setup skipped. Open ${SETUP_URL} to finish."
    return 0
  fi

  local output="" status=0 dash="" snippet=""
  case "$ADMIN_PASSWORD" in
    *\"*|*\\*)
      ui_warn "The admin password contains a quote or backslash; a generated password will be used."
      ADMIN_PASSWORD=""
      ;;
  esac
  local -a setup_args=(--config "$DIR/evechat.yaml" setup --non-interactive --json)
  [ -z "$ADMIN_EMAIL" ] || setup_args+=(--admin-email "$ADMIN_EMAIL")
  [ -z "$ADMIN_NAME" ] || setup_args+=(--admin-name "$ADMIN_NAME")
  [ -z "$ADMIN_PASSWORD" ] || setup_args+=(--admin-password "$ADMIN_PASSWORD")

  set +e
  # shellcheck disable=SC1091
  output="$( ( set -a; . "$DIR/.env"; set +a; "$DIR/evechat" "${setup_args[@]}" ) 2>&1 )"
  status=$?
  set -e

  if [ "$status" -ne 0 ]; then
    printf '%s\n' "$output" > "$DIR/setup-error.log"
    chmod 0600 "$DIR/setup-error.log"
    SETUP_FAILED=1
    SETUP_URL="${PUBLIC_URL}/setup"
    ui_warn "The first-run setup did not finish automatically."
    ui_note "Open ${PUBLIC_URL}/setup to complete it in the browser."
    ui_note "Details: ${DIR}/setup-error.log"
    return 0
  fi

  printf '%s\n' "$output" > "$DIR/setup-result.json"
  chmod 0600 "$DIR/setup-result.json"
  SETUP_EMAIL="$(json_field admin_email "$output")"
  SETUP_PASSWORD="$(json_field admin_password "$output")"
  SETUP_KEY="$(json_field project_key "$output")"
  dash="$(json_field dashboard_url "$output")"
  [ -n "$dash" ] || dash="$PUBLIC_URL"
  SETUP_URL="$dash"

  case "$output" in
    *'"already_configured":true'*)
      ALREADY_CONFIGURED=1
      SETUP_PASSWORD=""
      ;;
  esac

  snippet="$DIR/embed-snippet.html"
  if [ ! -f "$snippet" ] && [ -n "$SETUP_KEY" ]; then
    old_umask="$(umask)"
    umask 027
    {
      printf '<script>\n'
      printf '  window.EveChat = { projectKey: "%s", apiUrl: "%s" };\n' "$SETUP_KEY" "$dash"
      printf '</script>\n'
      printf '<script async src="%s/widget.js"></script>\n' "$dash"
    } > "$snippet"
    umask "$old_umask"
  fi

  # setup may rewrite evechat.yaml; restore the installer's permissions.
  if [ -f "$DIR/evechat.yaml" ]; then
    chmod 0640 "$DIR/evechat.yaml"
    if id -g "$SERVICE_USER" >/dev/null 2>&1; then
      chown "root:${SERVICE_GROUP}" "$DIR/evechat.yaml" 2>/dev/null || true
    fi
  fi

  if [ "$ALREADY_CONFIGURED" = 1 ]; then
    ui_ok "Existing administrator and project were kept"
  else
    ui_ok "Administrator, project and widget key created"
  fi
  return 0
}

# -----------------------------------------------------------------------------
#  nginx
# -----------------------------------------------------------------------------
nginx_conf_target() {
  if [ -n "$NGINX_CONF" ]; then
    printf '%s' "$NGINX_CONF"
    return 0
  fi
  if [ -d /etc/nginx/conf.d ]; then
    if [ ! -f /etc/nginx/nginx.conf ] || grep -qE 'include[[:space:]]+/etc/nginx/conf\.d/\*\.conf' /etc/nginx/nginx.conf; then
      printf '%s' "/etc/nginx/conf.d/evechat.conf"
      return 0
    fi
  fi
  if [ -d /etc/nginx/sites-available ]; then
    printf '%s' "/etc/nginx/sites-available/evechat.conf"
    return 0
  fi
  printf '%s' "/etc/nginx/conf.d/evechat.conf"
}

nginx_conf_ours() {
  local file="$1"
  [ -f "$file" ] || return 1
  grep -qF "Managed by EveChat installer" "$file" && return 0
  if grep -qF "EveChat" "$file" && grep -qF "generated by install.sh" "$file"; then
    return 0
  fi
  return 1
}

nginx_target_included() {
  local target="$1"
  case "$target" in
    /etc/nginx/conf.d/*)
      [ -f /etc/nginx/nginx.conf ] && grep -qE 'include[[:space:]]+.*conf\.d/\*\.conf' /etc/nginx/nginx.conf
      ;;
    /etc/nginx/sites-available/*)
      [ -f /etc/nginx/nginx.conf ] && grep -qE 'include[[:space:]]+.*sites-enabled/\*' /etc/nginx/nginx.conf
      ;;
    *)
      return 0
      ;;
  esac
}

nginx_domain_conflicts() {
  local domain="$1" target="$2" escaped="" hits="" link=""
  escaped="$(printf '%s' "$domain" | sed 's/\./\\./g')"
  link="/etc/nginx/sites-enabled/$(basename "$target")"
  hits="$(grep -RIsE --include='*.conf' "server_name[^;]*[[:space:]]${escaped}([[:space:]]|;)" /etc/nginx 2>/dev/null \
    | grep -vF "${target}:" | grep -vF "${link}:" || true)"
  [ -z "$hits" ] || printf '%s' "$hits"
}

detect_cert() {
  local domain="$1" exact="" candidate="" key=""
  CERT_FILE=""
  KEY_FILE=""
  CERT_EXPIRY=""
  [ -n "$domain" ] || return 1

  exact="/etc/letsencrypt/live/${domain}/fullchain.pem"
  if [ -f "$exact" ] && [ -f "/etc/letsencrypt/live/${domain}/privkey.pem" ]; then
    CERT_FILE="$exact"
    KEY_FILE="/etc/letsencrypt/live/${domain}/privkey.pem"
  elif command -v openssl >/dev/null 2>&1; then
    for candidate in /etc/letsencrypt/live/*/fullchain.pem; do
      [ -f "$candidate" ] || continue
      if openssl x509 -in "$candidate" -noout -checkhost "$domain" 2>/dev/null | grep -q "does match"; then
        key="$(dirname "$candidate")/privkey.pem"
        [ -f "$key" ] || continue
        CERT_FILE="$candidate"
        KEY_FILE="$key"
        break
      fi
    done
  fi
  if [ -n "$CERT_FILE" ] && command -v openssl >/dev/null 2>&1; then
    CERT_EXPIRY="$(openssl x509 -in "$CERT_FILE" -noout -enddate 2>/dev/null | sed 's/^notAfter=//')"
    if ! openssl x509 -in "$CERT_FILE" -noout -checkend 0 >/dev/null 2>&1; then
      ui_warn "The certificate for ${domain} has expired; using HTTP only."
      CERT_FILE=""
      KEY_FILE=""
    fi
  fi
  [ -n "$CERT_FILE" ]
}

dns_get_ip() {
  local domain="$1" ip=""
  [ -n "$domain" ] || return 0
  if command -v getent >/dev/null 2>&1; then
    ip="$(getent ahosts "$domain" 2>/dev/null | awk 'NR==1 {print $1}')"
  elif command -v dig >/dev/null 2>&1; then
    ip="$(dig +short "$domain" A 2>/dev/null | grep -E '^[0-9.]+$' | head -n1)"
  fi
  [ -z "$ip" ] || printf '%s' "$ip"
}

write_nginx_conf() {
  local target="" dir="" backup="" tmp="" domain="$DOMAIN" test_out="" listen_ssl=0
  local port="$PORT" link="" had_link=0
  nginx_installed || { ui_warn "nginx is not installed; no vhost was written."; return 1; }
  [ -n "$domain" ] || { ui_warn "No domain was given; no vhost was written."; return 1; }

  target="$(nginx_conf_target)"
  dir="$(dirname "$target")"
  if ! nginx_target_included "$target"; then
    ui_err "${dir} is not included by /etc/nginx/nginx.conf."
    ui_note "Add the include, or pass --nginx-conf with a path nginx already loads."
    return 1
  fi
  install -d -m 0755 "$dir" 2>/dev/null || true

  if [ -f "$target" ] && ! nginx_conf_ours "$target"; then
    ui_err "Refusing to overwrite ${target}; it was not created by this installer."
    ui_note "Move it away, or choose another file with --nginx-conf."
    return 1
  fi

  local conflicts=""
  conflicts="$(nginx_domain_conflicts "$domain" "$target")"
  if [ -n "$conflicts" ]; then
    ui_err "The domain ${domain} is already used by another nginx config:"
    printf '%s\n' "$conflicts" | sed 's/^/    /' >&2
    ui_note "Use another domain, or remove that server block first."
    return 1
  fi

  if [ -f "$target" ]; then
    backup="${target}.bak.$(date +%s)"
    cp -p "$target" "$backup" 2>/dev/null || true
  fi

  [ -n "$CERT_FILE" ] && [ -n "$KEY_FILE" ] && listen_ssl=1

  tmp="$(mktemp "${dir}/.evechat.XXXXXX")"
  {
    printf '# Managed by EveChat installer (install.sh).\n'
    printf '# Regenerate with: sudo install.sh --nginx --domain %s\n' "$domain"
    printf '# The service is expected on 127.0.0.1:%s (server.trust_proxy: true).\n\n' "$port"
    printf 'map $http_upgrade $evechat_connection_upgrade {\n'
    printf '    default upgrade;\n'
    printf "    ''      close;\n"
    printf '}\n\n'

    if [ "$listen_ssl" = 1 ]; then
      printf 'server {\n'
      printf '    listen 80;\n'
      printf '    server_name %s;\n' "$domain"
      printf '    return 301 https://$host$request_uri;\n'
      printf '}\n\n'
      printf 'server {\n'
      printf '    listen 443 ssl http2;\n'
      printf '    server_name %s;\n' "$domain"
      printf '    ssl_certificate     %s;\n' "$CERT_FILE"
      printf '    ssl_certificate_key %s;\n' "$KEY_FILE"
      printf '    ssl_protocols TLSv1.2 TLSv1.3;\n'
      printf '    ssl_session_cache shared:EveChatTLS:10m;\n'
      printf '    ssl_session_timeout 1d;\n'
    else
      printf 'server {\n'
      printf '    listen 80;\n'
      printf '    server_name %s;\n' "$domain"
    fi
    printf '    client_max_body_size 10m;\n'
    printf '    add_header X-Content-Type-Options nosniff always;\n'
    printf '    add_header Referrer-Policy strict-origin-when-cross-origin always;\n\n'

    printf '    location = /widget.js {\n'
    printf '        proxy_pass http://127.0.0.1:%s;\n' "$port"
    printf '        proxy_set_header Host $host;\n'
    printf '        proxy_set_header X-Forwarded-For $remote_addr;\n'
    printf '        expires 5m;\n'
    printf '        add_header Cache-Control "public, max-age=300";\n'
    printf '    }\n\n'

    printf '    location /assets/ {\n'
    printf '        proxy_pass http://127.0.0.1:%s;\n' "$port"
    printf '        proxy_set_header Host $host;\n'
    printf '        expires 1y;\n'
    printf '        add_header Cache-Control "public, immutable";\n'
    printf '    }\n\n'

    printf '    location /api/widget/v1/events {\n'
    printf '        proxy_pass http://127.0.0.1:%s;\n' "$port"
    printf '        proxy_http_version 1.1;\n'
    printf '        proxy_set_header Host $host;\n'
    printf '        proxy_set_header X-Real-IP $remote_addr;\n'
    printf '        proxy_set_header X-Forwarded-For $remote_addr;\n'
    printf '        proxy_set_header X-Forwarded-Proto $scheme;\n'
    printf '        proxy_buffering off;\n'
    printf '        proxy_cache off;\n'
    printf '        proxy_read_timeout 3600s;\n'
    printf '        proxy_send_timeout 3600s;\n'
    printf '    }\n\n'

    printf '    location /ws/ {\n'
    printf '        proxy_pass http://127.0.0.1:%s;\n' "$port"
    printf '        proxy_http_version 1.1;\n'
    printf '        proxy_set_header Upgrade $http_upgrade;\n'
    printf '        proxy_set_header Connection $evechat_connection_upgrade;\n'
    printf '        proxy_set_header Host $host;\n'
    printf '        proxy_set_header X-Real-IP $remote_addr;\n'
    printf '        proxy_set_header X-Forwarded-For $remote_addr;\n'
    printf '        proxy_set_header X-Forwarded-Proto $scheme;\n'
    printf '        proxy_read_timeout 3600s;\n'
    printf '        proxy_send_timeout 3600s;\n'
    printf '    }\n\n'

    printf '    location /api/widget/v1/poll {\n'
    printf '        proxy_pass http://127.0.0.1:%s;\n' "$port"
    printf '        proxy_http_version 1.1;\n'
    printf '        proxy_set_header Host $host;\n'
    printf '        proxy_set_header X-Real-IP $remote_addr;\n'
    printf '        proxy_set_header X-Forwarded-For $remote_addr;\n'
    printf '        proxy_buffering off;\n'
    printf '        proxy_read_timeout 120s;\n'
    printf '    }\n\n'

    printf '    location / {\n'
    printf '        proxy_pass http://127.0.0.1:%s;\n' "$port"
    printf '        proxy_http_version 1.1;\n'
    printf '        proxy_set_header Host $host;\n'
    printf '        proxy_set_header X-Real-IP $remote_addr;\n'
    printf '        proxy_set_header X-Forwarded-For $remote_addr;\n'
    printf '        proxy_set_header X-Forwarded-Proto $scheme;\n'
    printf '        proxy_read_timeout 3600s;\n'
    printf '        proxy_send_timeout 3600s;\n'
    printf '    }\n'
    printf '}\n'
  } > "$tmp"

  mv -f "$tmp" "$target"
  chmod 0644 "$target"
  if [ "$(basename "$dir")" = "sites-available" ]; then
    link="/etc/nginx/sites-enabled/$(basename "$target")"
    [ -L "$link" ] && had_link=1
    install -d -m 0755 /etc/nginx/sites-enabled
    ln -sfn "$target" "$link"
  fi

  if ! test_out="$(nginx -t 2>&1)"; then
    rm -f "$target"
    [ -z "$link" ] || rm -f "$link"
    if [ -n "$backup" ] && [ -f "$backup" ]; then
      mv -f "$backup" "$target"
      if [ "$had_link" = 1 ] && [ -n "$link" ]; then
        ln -sfn "$target" "$link"
      fi
      ui_note "The previous vhost was restored."
    fi
    ui_err "nginx rejected the generated configuration:"
    printf '%s\n' "$test_out" | sed 's/^/    /' >&2
    ui_note "nginx was not reloaded; nothing changed."
    return 1
  fi

  CHANGED_NGINX="$target"
  if systemctl is-active --quiet nginx 2>/dev/null; then
    if ! systemctl reload nginx; then
      ui_warn "nginx -t passed, but reload failed. Run: sudo systemctl reload nginx"
      return 0
    fi
  elif nginx -s reload >/dev/null 2>&1; then
    :
  else
    ui_warn "nginx is not running; the vhost will apply when nginx starts."
  fi

  ui_ok "nginx vhost written and loaded: ${target}"
  [ "$listen_ssl" = 1 ] || ui_warn "TLS is not configured yet (HTTP only). Add a certificate, then re-run --nginx."
  return 0
}

remove_nginx_conf() {
  local f="" removed=0 test_out="" backup_dir="" link=""
  local -a removed_files=()
  backup_dir="/etc/nginx/evechat-removed.$(date +%s)"
  for f in /etc/nginx/conf.d/evechat*.conf /etc/nginx/sites-available/evechat*.conf ${NGINX_CONF:+"$NGINX_CONF"}; do
    [ -f "$f" ] || continue
    if nginx_conf_ours "$f"; then
      install -d -m 0700 "$backup_dir"
      cp -p "$f" "${backup_dir}/$(basename "$f")"
      rm -f "$f"
      link="/etc/nginx/sites-enabled/$(basename "$f")"
      if [ -L "$link" ]; then
        rm -f "$link"
      fi
      removed_files+=("$f")
      removed=1
    fi
  done
  [ "$removed" = 1 ] || return 0
  if ! nginx_installed; then
    ui_ok "EveChat nginx vhost removed (nginx is not installed)."
    CHANGED_NGINX="removed"
    return 0
  fi

  if ! test_out="$(nginx -t 2>&1)"; then
    for f in ${removed_files[@]+"${removed_files[@]}"}; do
      cp -p "${backup_dir}/$(basename "$f")" "$f"
      if [ "$(basename "$(dirname "$f")")" = "sites-available" ]; then
        ln -sfn "$f" "/etc/nginx/sites-enabled/$(basename "$f")"
      fi
    done
    ui_err "nginx rejected the configuration after removing the EveChat vhost:"
    printf '%s\n' "$test_out" | sed 's/^/    /' >&2
    ui_warn "The vhost was restored; nginx was not reloaded."
    return 1
  fi
  if systemctl is-active --quiet nginx 2>/dev/null; then
    systemctl reload nginx >/dev/null 2>&1 || ui_warn "nginx reload failed after removing the vhost."
  elif nginx -s reload >/dev/null 2>&1; then
    :
  fi
  ui_ok "EveChat nginx vhost removed (backup: ${backup_dir})"
  CHANGED_NGINX="removed"
  return 0
}

# -----------------------------------------------------------------------------
#  Install flow
# -----------------------------------------------------------------------------
install_dir_safe() {
  case "$DIR" in
    /*) ;;
    *) ui_err "Use an absolute path, for example /opt/evechat."; return 1 ;;
  esac
  case "$DIR" in
    *[[:space:]]*) ui_err "The install path must not contain spaces."; return 1 ;;
  esac
  case "$DIR" in
    /|/bin|/boot|/dev|/etc|/home|/lib|/lib64|/opt|/proc|/root|/run|/sbin|/srv|/sys|/tmp|/usr|/var)
      ui_err "Refusing to install directly into ${DIR}."
      return 1
      ;;
    /root/*)
      ui_err "Do not install under /root; the service user cannot traverse it."
      return 1
      ;;
  esac
  case "/${DIR}/" in
    */../*) ui_err "The install path must not contain '..'."; return 1 ;;
  esac
  [ "${#DIR}" -ge 6 ] || { ui_err "The install path is too short."; return 1; }
  if [ -L "$DIR" ]; then
    ui_err "The install path must not be a symbolic link."
    return 1
  fi
  return 0
}

step_location() {
  SCREEN_TITLE="INSTALL"
  SCREEN_SUB="Step 1 of 6 · Install location"
  if [ "$PROMPT" != 1 ]; then
    install_dir_safe || return 1
    return 0
  fi
  while :; do
    ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
    ui_heading "WHERE SHOULD EVECHAT LIVE?"
    ui_note "The binary, configuration, database and uploads live here."
    ui_note "Existing files are never overwritten silently."
    ask_value DIR "Install directory" "$DIR"
    DIR="${DIR%/}"
    [ -n "$DIR" ] || DIR="/"
    install_dir_safe || continue

    if [ -e "$DIR" ] && [ -n "$(find "$DIR" -mindepth 1 -maxdepth 1 2>/dev/null | head -n1)" ]; then
      local reuse_default="n"
      if [ -f "$DIR/.evechat-install" ]; then
        ui_warn "This directory already contains an EveChat install."
        reuse_default="y"
      else
        ui_warn "This directory is not empty."
      fi
      if ! confirm_yes "Reuse ${DIR} anyway?" "$reuse_default"; then
        continue
      fi
    fi
    break
  done
  return 0
}

step_network() {
  SCREEN_TITLE="INSTALL"
  SCREEN_SUB="Step 2 of 6 · Network mode"
  if [ -n "$NET_MODE" ]; then
    case "$NET_MODE" in
      proxy) HOST="127.0.0.1"; TRUST="true" ;;
      direct) HOST="0.0.0.0"; TRUST="false" ;;
      local) HOST="127.0.0.1"; TRUST="false" ;;
    esac
    return 0
  fi

  local selected=0

  if [ "$PROMPT" != 1 ]; then
    # Unattended installs stay reachable even when nginx is present. Proxy
    # mode is opt-in through --behind-proxy / --write-nginx.
    NET_MODE="direct"
    HOST="0.0.0.0"
    TRUST="false"
    return 0
  fi

  SCREEN_SUB="Step 2 of 6 · Network mode"
  MENU_START=0
  if ! nginx_installed && ! port_taken 80 && ! port_taken 443; then
    MENU_START=1
  fi
  menu_pick "How should EveChat be exposed?" \
    "Behind a reverse proxy" "recommended · binds 127.0.0.1" \
    "Direct public port" "public · http://server:port" \
    "Loopback only" "local only · add a proxy later" || return 1
  MENU_START=0
  [ "$MENU_CANCELLED" = 1 ] && return 1
  selected="$MENU_INDEX"
  case "$selected" in
    0) NET_MODE="proxy"; HOST="127.0.0.1"; TRUST="true" ;;
    1) NET_MODE="direct"; HOST="0.0.0.0"; TRUST="false" ;;
    2) NET_MODE="local"; HOST="127.0.0.1"; TRUST="false" ;;
  esac
  return 0
}

step_port() {
  SCREEN_TITLE="INSTALL"
  SCREEN_SUB="Step 3 of 6 · Service port"
  local candidate="${PORT:-$DEFAULT_PORT}" suggested="" owner="" answer="" our_port=0
  case "$candidate" in
    ''|*[!0-9]*) ui_err "Invalid port: ${candidate}"; return 1 ;;
  esac
  if [ "$candidate" -lt 1 ] || [ "$candidate" -gt 65535 ]; then
    ui_err "Invalid port: ${candidate} (expected 1-65535)."
    return 1
  fi
  if [ "$NET_MODE" = "proxy" ] && { [ "$candidate" = 80 ] || [ "$candidate" = 443 ]; }; then
    ui_err "Port ${candidate} belongs to the web server in proxy mode."
    ui_note "Use an internal port such as 3000 or 3100."
    return 1
  fi

  if http_health "$candidate"; then
    our_port=1
  fi
  if port_taken "$candidate" && [ "$our_port" != 1 ]; then
    owner="$(port_owner "$candidate")"
    if [ "$PORT_SET" = 1 ]; then
      ui_err "Port ${candidate} is already used by ${owner}."
      ui_note "Choose another port with --port."
      return 1
    fi
    suggested="$(first_free_port || true)"
    if [ -z "$suggested" ]; then
      ui_err "No free port was found between 3100 and 3199."
      ui_note "Pass --port with a free port."
      return 1
    fi
    ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
    ui_warn "Port ${candidate} is used by ${owner}; suggesting ${suggested}."
    candidate="$suggested"
  fi

  if [ "$PROMPT" != 1 ]; then
    PORT="$candidate"
    PORT_SET=1
    return 0
  fi

  while :; do
    ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
    ui_heading "WHICH PORT SHOULD EVECHAT LISTEN ON?"
    ui_note "The dashboard and widget will be served on this port."
    if [ "$NET_MODE" = "proxy" ]; then
      ui_note "nginx will connect to 127.0.0.1:${PORT}."
    fi
    ask_value answer "Service port" "$candidate"
    case "$answer" in
      ''|*[!0-9]*) ui_warn "Enter a number between 1 and 65535."; continue ;;
    esac
    if [ "$answer" -lt 1 ] || [ "$answer" -gt 65535 ]; then
      ui_warn "Enter a number between 1 and 65535."
      continue
    fi
    if [ "$NET_MODE" = "proxy" ] && { [ "$answer" = 80 ] || [ "$answer" = 443 ]; }; then
      ui_warn "Port ${answer} belongs to nginx; choose an internal port."
      continue
    fi
    if port_taken "$answer" && ! http_health "$answer"; then
      ui_warn "Port ${answer} is already used by $(port_owner "$answer")."
      continue
    fi
    PORT="$answer"
    PORT_SET=1
    break
  done
  return 0
}

step_domain() {
  SCREEN_TITLE="INSTALL"
  SCREEN_SUB="Step 4 of 6 · Public address"
  local answer=""

  if [ "$NET_MODE" != "proxy" ]; then
    if [ -z "$PUBLIC_URL" ]; then
      if [ "$NET_MODE" = "local" ]; then
        PUBLIC_URL="http://127.0.0.1:${PORT}"
      else
        PUBLIC_URL="http://$(primary_ip):${PORT}"
      fi
    fi
    WRITE_NGINX=0
    ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
    ui_panel_open "PUBLIC ACCESS"
    if [ "$NET_MODE" = "local" ]; then
      ui_kv "Mode" "loopback only"
    else
      ui_kv "Mode" "direct port"
    fi
    ui_kv "URL" "$PUBLIC_URL"
    ui_panel_close
    return 0
  fi

  if [ -z "$DOMAIN" ] && [ -n "$PUBLIC_URL" ]; then
    DOMAIN="$(url_host "$PUBLIC_URL")"
  fi
  if [ -z "$DOMAIN" ] && [ "$PROMPT" = 1 ]; then
    ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
    ui_heading "PUBLIC DOMAIN"
    ui_note "Use a subdomain you control, for example chat.example.com."
    ui_note "Leave it blank to keep EveChat on loopback and add a proxy later."
    ask_value DOMAIN "Public domain" ""
  fi

  if [ -n "$DOMAIN" ]; then
    case "$DOMAIN" in
      *[!A-Za-z0-9.-]*|.*|*..*)
        ui_err "Invalid domain: ${DOMAIN}"
        return 1
        ;;
    esac
    DOMAIN="$(printf '%s' "$DOMAIN" | tr '[:upper:]' '[:lower:]')"
    detect_cert "$DOMAIN" || true
    if [ "$PUBLIC_URL_SET" != 1 ]; then
      if [ -n "$CERT_FILE" ]; then
        PUBLIC_URL="https://${DOMAIN}"
      else
        PUBLIC_URL="http://${DOMAIN}"
      fi
    fi
    local dns_ip=""
    dns_ip="$(dns_get_ip "$DOMAIN")"
    if [ -n "$dns_ip" ] && [ "$dns_ip" != "$(primary_ip)" ]; then
      ui_warn "${DOMAIN} resolves to ${dns_ip}; this server is $(primary_ip)."
      ui_note "A mismatched DNS record will break TLS and the widget."
    fi
    if nginx_installed; then
      if [ -z "$WRITE_NGINX" ]; then
        if [ "$PROMPT" = 1 ]; then
          WRITE_NGINX=1
          if ! confirm_yes "Write a dedicated nginx vhost for ${DOMAIN}?" "y"; then
            WRITE_NGINX=0
          fi
        else
          WRITE_NGINX=0
        fi
      fi
    else
      WRITE_NGINX=0
      ui_warn "nginx is not installed; add your own reverse proxy for ${DOMAIN}."
    fi
    if [ "$WRITE_NGINX" != 1 ] && [ "$PROMPT" != 1 ] && nginx_installed; then
      ui_warn "No vhost was requested; pass --write-nginx to let the installer create one."
    fi
  else
    if [ "$NET_MODE_EXPLICIT" != 1 ]; then
      if [ "$PROMPT" != 1 ] || confirm_yes "No domain. Expose EveChat directly on port ${PORT} instead?" "y"; then
        NET_MODE="direct"
        HOST="0.0.0.0"
        TRUST="false"
        PUBLIC_URL="http://$(primary_ip):${PORT}"
        ui_warn "No domain: falling back to a direct port (${PORT})."
      else
        PUBLIC_URL="http://127.0.0.1:${PORT}"
        ui_warn "No domain: EveChat stays on loopback until you add a proxy."
      fi
    else
      PUBLIC_URL="http://127.0.0.1:${PORT}"
      ui_warn "No domain: EveChat stays on loopback until you add a proxy."
    fi
    WRITE_NGINX=0
  fi

  ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
  ui_panel_open "PUBLIC ACCESS"
  if [ "$NET_MODE" = "proxy" ]; then
    ui_kv "Mode" "behind a reverse proxy"
  else
    ui_kv "Mode" "direct port"
  fi
  ui_kv "Domain" "${DOMAIN:-none}"
  ui_kv "URL" "$PUBLIC_URL"
  if [ "$NET_MODE" != "proxy" ]; then
    ui_kv "TLS" "not used"
  elif [ -n "$CERT_FILE" ]; then
    ui_kv "TLS" "certificate found"
  else
    ui_kv "TLS" "not configured (HTTP first)"
  fi
  ui_kv "nginx vhost" "$([ "$WRITE_NGINX" = 1 ] && printf 'will be written' || printf 'not written')"
  ui_panel_close
  return 0
}

step_admin() {
  SCREEN_TITLE="INSTALL"
  SCREEN_SUB="Step 5 of 6 · Administrator"
  if [ "$SKIP_SETUP" = 1 ]; then
    return 0
  fi
  local default_email="admin@evechat.local"
  [ -n "$DOMAIN" ] && default_email="admin@${DOMAIN}"
  [ -n "$ADMIN_NAME" ] || ADMIN_NAME="Administrator"
  [ -n "$ADMIN_EMAIL" ] || ADMIN_EMAIL="$default_email"

  if [ "$PROMPT" != 1 ]; then
    return 0
  fi

  while :; do
    ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
    ui_heading "FIRST ADMINISTRATOR"
    ui_note "These credentials open the dashboard after the install."
    ui_note "You can change them later from Settings."
    ask_value ADMIN_NAME "Administrator name" "$ADMIN_NAME"
    ask_value ADMIN_EMAIL "Administrator email" "$ADMIN_EMAIL"
    case "$ADMIN_EMAIL" in
      *@*.*) break ;;
      *) ui_warn "Enter a valid email address." ;;
    esac
  done
  ui_blank
  ask_secret_value ADMIN_PASSWORD "Administrator password (blank = generate a strong one)"
  return 0
}

step_review() {
  SCREEN_TITLE="INSTALL"
  SCREEN_SUB="Step 6 of 6 · Review"
  local mode="direct" nginx_line="not written" tls_line="not used" svc="manual"
  [ "$NET_MODE" = "proxy" ] && mode="behind a reverse proxy (127.0.0.1)"
  [ "$NET_MODE" = "local" ] && mode="loopback only"
  [ "$WRITE_NGINX" = 1 ] && nginx_line="$(nginx_conf_target)"
  if [ "$NET_MODE" = "proxy" ]; then
    if [ -n "$CERT_FILE" ]; then tls_line="certificate found"; else tls_line="HTTP only (no certificate)"; fi
  fi
  if [ "$NO_SYSTEMD" != 1 ] && systemd_available; then
    svc="systemd · ${SERVICE_NAME}.service"
  fi

  ui_screen "$SCREEN_TITLE" "$SCREEN_SUB"
  ui_panel_open "REVIEW"
  ui_kv "Install dir" "$DIR"
  ui_kv "Release" "${VERSION:-latest}"
  ui_kv "Network" "$mode"
  ui_kv "Port" "$PORT"
  ui_kv "Public URL" "$PUBLIC_URL"
  ui_kv "TLS" "$tls_line"
  ui_kv "nginx" "$nginx_line"
  ui_kv "Service" "$svc"
  ui_kv "Service user" "$SERVICE_USER"
  if [ "$SKIP_SETUP" = 1 ]; then
    ui_kv "Admin" "finish in the browser"
  else
    ui_kv "Admin" "$ADMIN_EMAIL ($([ -n "$ADMIN_PASSWORD" ] && printf 'password set' || printf 'password generated'))"
  fi
  ui_kv "Data" "${DIR}/data"
  ui_panel_close

  ui_blank
  ui_note "EveChat uses its own service, user, database and port."
  ui_note "nginx, EVE, Redis, Docker and every other service are left untouched."
  if ! confirm_yes "Proceed with installation?" "y"; then
    return 1
  fi
  return 0
}

report_changes() {
  ui_panel_open "WHAT CHANGED ON THIS SERVER"
  ui_kv "Install dir" "$DIR"
  ui_kv "Service user" "${CHANGED_USER:-unchanged}"
  ui_kv "systemd" "${CHANGED_SYSTEMD:-not installed}"
  ui_kv "nginx" "${CHANGED_NGINX:-not written}"
  ui_kv "Listen" "${HOST}:${PORT}"
  ui_kv "Public URL" "$PUBLIC_URL"
  ui_kv "Other services" "not modified"
  ui_panel_close
}

show_success() {
  ui_clear
  ui_banner
  ui_panel_open "EVECHAT IS LIVE"
  ui_kv "Dashboard" "$SETUP_URL"
  if [ "$SKIP_SETUP" = 1 ] || [ "$SETUP_FAILED" = 1 ]; then
    ui_kv "Setup" "${PUBLIC_URL}/setup"
  elif [ "$ALREADY_CONFIGURED" = 1 ]; then
    ui_kv "Admin" "existing account (unchanged)"
    ui_kv "Password" "unchanged"
  else
    ui_kv "Email" "${SETUP_EMAIL:-$ADMIN_EMAIL}"
    ui_kv "Password" "${SETUP_PASSWORD:-$ADMIN_PASSWORD}"
  fi
  [ -n "$SETUP_KEY" ] && ui_kv "Project key" "$SETUP_KEY"
  [ -f "$DIR/embed-snippet.html" ] && ui_kv "Widget snippet" "${DIR}/embed-snippet.html"
  ui_panel_close

  ui_heading "NEXT STEPS"
  printf '  %s1%s  Open the dashboard and sign in.\n' "$C_ACCENT" "$C_RESET"
  printf '  %s2%s  Add the widget snippet to your site (or to EVE).\n' "$C_ACCENT" "$C_RESET"
  printf '  %s3%s  Add your EVE domain under Settings → Widget.\n' "$C_ACCENT" "$C_RESET"
  if [ -n "$DOMAIN" ] && [ -z "$CERT_FILE" ] && [ "$WRITE_NGINX" = 1 ]; then
    ui_blank
    ui_warn "TLS is not configured yet. After adding a certificate, run:"
    printf '      sudo certbot --nginx -d %s\n' "$DOMAIN"
    printf '      curl -fsSL %s/raw/main/install.sh | sudo bash -s -- --nginx --domain %s\n' "$GITHUB" "$DOMAIN"
  fi
  if [ "$SETUP_FAILED" = 1 ]; then
    ui_warn "Finish the first-run setup at ${PUBLIC_URL}/setup"
  fi
  if [ "$NGINX_FAILED" = 1 ]; then
    ui_warn "The nginx vhost was not applied. See the message above."
  fi
  if [ "$NO_SYSTEMD" = 1 ]; then
    ui_blank
    ui_note "Start EveChat manually with:"
    printf '    %s/evechat --config %s/evechat.yaml serve\n' "$DIR" "$DIR"
  fi
  if [ "$NET_MODE" = "direct" ] && command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    ui_note "ufw is active; allow the port with: sudo ufw allow ${PORT}/tcp"
  fi
  ui_blank
  ui_panel_open "USEFUL COMMANDS"
  printf '  %ssystemctl status %s%s\n' "$C_MUTED" "$SERVICE_NAME" "$C_RESET"
  printf '  %sjournalctl -u %s -f%s\n' "$C_MUTED" "$SERVICE_NAME" "$C_RESET"
  printf '  %s%s/evechat --config %s/evechat.yaml doctor --verbose%s\n' "$C_MUTED" "$DIR" "$DIR" "$C_RESET"
  ui_panel_close
}

execute_install() {
  ui_stage 1 "$STAGE_TOTAL" "Resolving release"
  resolve_version

  ui_stage 2 "$STAGE_TOTAL" "Downloading and verifying"
  download_release

  ui_stage 3 "$STAGE_TOTAL" "Creating the service account"
  install_service_user

  ui_stage 4 "$STAGE_TOTAL" "Installing files"
  install_files

  ui_stage 5 "$STAGE_TOTAL" "Writing configuration and secrets"
  write_env
  write_config
  write_marker

  ui_stage 6 "$STAGE_TOTAL" "Starting the service"
  install_systemd || die "The service could not be started."
  if [ "$NO_SYSTEMD" != 1 ]; then
    wait_health || die "The service is not healthy."
  fi

  ui_stage 7 "$STAGE_TOTAL" "Creating the first administrator"
  run_setup
  if [ "$NO_SYSTEMD" != 1 ] && [ "$SETUP_FAILED" != 1 ]; then
    if ! systemctl restart "$SERVICE_NAME"; then
      systemctl status "$SERVICE_NAME" --no-pager -l 2>/dev/null | sed 's/^/    /' >&2 || true
      die "The service did not restart after setup."
    fi
    wait_health || die "The service did not come back after setup."
  fi

  ui_stage 8 "$STAGE_TOTAL" "Configuring the reverse proxy"
  if [ "$WRITE_NGINX" = 1 ]; then
    write_nginx_conf || NGINX_FAILED=1
  else
    ui_note "No nginx vhost was requested."
  fi

  report_changes
  show_success
}

flow_install() {
  need_root
  require_tools
  platform_detect
  discover_install_dir
  detect_environment
  CHANGED_USER=""
  CHANGED_SYSTEMD=""
  CHANGED_NGINX=""

  if is_installed; then
    ui_screen "INSTALL" "Existing installation detected"
    ui_panel_open "EXISTING INSTALLATION"
    ui_kv "Directory" "$DIR"
    ui_kv "Version" "$(version_of_binary)"
    ui_kv "Service" "$(service_state)"
    ui_kv "Public URL" "$(cfg_get "$DIR/evechat.yaml" public_url)"
    ui_panel_close
    if [ "$PROMPT" = 1 ]; then
      SCREEN_TITLE="INSTALL"
      SCREEN_SUB="Existing installation"
      menu_pick "EveChat is already installed." \
        "Upgrade keeping settings" "download the latest binary" \
        "Cancel" "return to the menu" || return 0
      [ "$MENU_CANCELLED" = 1 ] && return 0
      if [ "$MENU_INDEX" = 0 ]; then
        flow_upgrade || return 0
      fi
    else
      ui_warn "Already installed; switching to upgrade mode."
      flow_upgrade
    fi
    return 0
  fi

  # A previous failed run may have left a config without a binary. Keep its
  # server settings instead of silently changing ports or bind addresses.
  if [ -f "$DIR/evechat.yaml" ] && ! is_installed; then
    local old_port="" old_host="" old_trust="" old_url=""
    old_port="$(cfg_get "$DIR/evechat.yaml" port)"
    old_host="$(cfg_get "$DIR/evechat.yaml" host)"
    old_trust="$(cfg_get "$DIR/evechat.yaml" trust_proxy)"
    old_url="$(cfg_get "$DIR/evechat.yaml" public_url)"
    [ -n "$old_port" ] && PORT="$old_port"
    [ -n "$old_host" ] && HOST="$old_host"
    [ -n "$old_trust" ] && TRUST="$old_trust"
    if [ -n "$old_url" ]; then
      PUBLIC_URL="${old_url%/}"
      PUBLIC_URL_SET=1
    fi
    DOMAIN="$(url_host "$PUBLIC_URL")"
    local old_user=""
    old_user="$(unit_service_user)"
    if [ -n "$old_user" ] && [ "$USER_SET" != 1 ]; then
      SERVICE_USER="$old_user"
      SERVICE_GROUP="$(unit_service_group)"
      [ -n "$SERVICE_GROUP" ] || SERVICE_GROUP="$(id -gn "$SERVICE_USER" 2>/dev/null || printf '%s' "$SERVICE_USER")"
    fi
    if [ "$TRUST" = true ] || [ "$HOST" = "127.0.0.1" ]; then
      NET_MODE="proxy"
    else
      NET_MODE="direct"
    fi
    ui_warn "An existing config was found; its server settings will be kept."
  fi

  ui_screen "INSTALL" "Guided setup"
  ui_heading "PREFLIGHT"
  ui_panel_open "THIS SERVER"
  ui_kv "Platform" "${OS}/${ARCH}"
  ui_kv "Memory" "$(mem_summary)"
  ui_kv "Disk" "$(disk_summary)"
  ui_kv "Web server" "$ENV_WEB"
  ui_kv "Port" "$ENV_LISTEN"
  ui_panel_close
  if nginx_installed; then
    ui_note "nginx is present on this server. EveChat can get its own vhost"
    ui_note "and a loopback port; other sites stay untouched."
  fi
  if ! systemd_available; then
    ui_warn "systemd is not available; the service will run in manual mode."
  fi

  step_location || return 0
  step_network || return 0
  step_port || return 1
  step_domain || return 1
  step_admin
  if ! step_review; then
    ui_clear
    ui_note "Installation cancelled. Nothing was changed."
    return 0
  fi
  execute_install
}

flow_upgrade() {
  need_root
  require_tools
  platform_detect
  discover_install_dir
  load_existing || return 1
  detect_environment
  CHANGED_USER=""
  CHANGED_SYSTEMD=""
  CHANGED_NGINX=""

  ui_screen "UPGRADE" "Keep data and configuration"
  ui_panel_open "CURRENT INSTALLATION"
  ui_kv "Directory" "$DIR"
  ui_kv "Version" "$(version_of_binary)"
  ui_kv "Service" "$(service_state)"
  ui_kv "Port" "$PORT"
  ui_panel_close

  resolve_version
  local current=""
  current="$(version_of_binary)"
  if [ "$current" = "$VERSION" ] && [ "$FORCE" != 1 ]; then
    if [ "$PROMPT" = 1 ]; then
      if ! confirm_yes "Version ${VERSION} is already installed. Reinstall it?" "n"; then
        ui_note "Nothing changed."
        return 0
      fi
    else
      ui_ok "Already on ${VERSION}; nothing to do."
      return 0
    fi
  fi

  ui_stage 1 4 "Downloading and verifying ${VERSION}"
  download_release
  ui_stage 2 4 "Replacing the binary"
  install_service_user
  install_files
  write_env
  write_config
  write_marker
  ui_stage 3 4 "Restarting the service"
  install_systemd || die "The service could not be restarted."
  if [ "$NO_SYSTEMD" != 1 ]; then
    wait_health || die "The upgraded service is not healthy."
  fi
  ui_stage 4 4 "Finishing"
  report_changes
  ui_clear
  ui_banner
  ui_panel_open "UPGRADE COMPLETE"
  ui_kv "Version" "$(version_of_binary)"
  ui_kv "Directory" "$DIR"
  ui_kv "Service" "$(service_state)"
  ui_kv "Data" "kept"
  ui_panel_close
  ui_ok "EveChat was upgraded; configuration and data were not changed."
}

flow_proxy() {
  need_root
  platform_detect
  discover_install_dir
  load_existing || return 1
  detect_environment
  CHANGED_USER=""
  CHANGED_SYSTEMD=""
  CHANGED_NGINX=""

  ui_screen "REVERSE PROXY" "Write a dedicated nginx vhost"
  if ! nginx_installed; then
    ui_err "nginx is not installed on this server."
    ui_note "Sample configs: ${DIR}/deploy/nginx/evechat.conf"
    ui_note "Caddy: ${DIR}/deploy/caddy/Caddyfile"
    return 1
  fi

  local default_domain="$DOMAIN" new_domain=""
  ui_panel_open "CURRENT"
  ui_kv "Port" "$PORT"
  ui_kv "Public URL" "$PUBLIC_URL"
  ui_kv "nginx" "$(nginx_version)"
  ui_kv "vhost" "$([ -f "$(nginx_conf_target)" ] && printf '%s' "$(nginx_conf_target)" || printf 'not written')"
  ui_panel_close

  new_domain="$default_domain"
  ask_value new_domain "Public domain" "$default_domain"
  if [ -z "$new_domain" ]; then
    ui_warn "A domain is required for an nginx vhost."
    return 1
  fi
  case "$new_domain" in
    *[!A-Za-z0-9.-]*|.*|*..*) ui_err "Invalid domain: ${new_domain}"; return 1 ;;
  esac
  DOMAIN="$(printf '%s' "$new_domain" | tr '[:upper:]' '[:lower:]')"
  if [ "$(url_host "$PUBLIC_URL")" != "$DOMAIN" ]; then
    PUBLIC_URL_SET=0
  fi
  detect_cert "$DOMAIN" || true
  if [ -n "$CERT_FILE" ]; then
    PUBLIC_URL="https://${DOMAIN}"
    ui_ok "Certificate found for ${DOMAIN}"
  else
    [ "$PUBLIC_URL_SET" = 1 ] || PUBLIC_URL="http://${DOMAIN}"
    ui_warn "No certificate found; writing an HTTP-only vhost."
  fi
  WRITE_NGINX=1
  if ! write_nginx_conf; then
    return 1
  fi

  ui_blank
  if confirm_yes "Move EveChat to 127.0.0.1 and trust proxy headers?" "y"; then
    local backup=""
    backup="${DIR}/evechat.yaml.bak.$(date +%s)"
    cp -p "$DIR/evechat.yaml" "$backup"
    if ! cfg_set_server "$DIR/evechat.yaml" "127.0.0.1" "$PORT" "$PUBLIC_URL" "true"; then
      ui_err "Could not update ${DIR}/evechat.yaml; the bind address was not changed."
      return 1
    fi
    HOST="127.0.0.1"
    TRUST="true"
    if systemd_available; then
      if ! systemctl restart "$SERVICE_NAME" || ! wait_health; then
        ui_err "Rolling back the configuration change."
        mv -f "$backup" "$DIR/evechat.yaml"
        systemctl restart "$SERVICE_NAME" >/dev/null 2>&1 || true
        return 1
      fi
    fi
    ui_ok "EveChat now listens on 127.0.0.1:${PORT} behind nginx"
  else
    ui_warn "Kept the current bind address; proxy headers may be ignored."
  fi

  ui_clear
  ui_banner
  ui_panel_open "REVERSE PROXY READY"
  ui_kv "Dashboard" "$PUBLIC_URL"
  ui_kv "vhost" "$(nginx_conf_target)"
  ui_kv "Upstream" "127.0.0.1:${PORT}"
  ui_panel_close
  ui_note "Test it: curl -I ${PUBLIC_URL}"
}

flow_status() {
  platform_detect
  discover_install_dir
  detect_environment
  ui_screen "STATUS" "Read-only checks"

  if ! is_installed; then
    ui_panel_open "EVECHAT"
    ui_kv "State" "not installed"
    ui_kv "Directory" "$DIR"
    ui_kv "Web server" "$ENV_WEB"
    ui_panel_close
    ui_note "Run the installer and choose Install EveChat."
    return 0
  fi

  local cfg="$DIR/evechat.yaml" health="no response" db="" uploads="" vhost="" cert_line="none" p="" h="" t="" u=""
  p="$(cfg_get "$cfg" port)"
  h="$(cfg_get "$cfg" host)"
  t="$(cfg_get "$cfg" trust_proxy)"
  u="$(cfg_get "$cfg" public_url)"
  [ -n "$p" ] || p="$DEFAULT_PORT"
  [ -n "$h" ] || h="127.0.0.1"
  [ -n "$t" ] || t="false"
  [ -n "$u" ] || u="http://127.0.0.1:${p}"
  if http_health "$p"; then health="OK (HTTP 200)"; fi
  [ -d "$DIR/data" ] && db="$(du -sh "$DIR/data" 2>/dev/null | awk '{print $1}')"
  [ -d "$DIR/data/uploads" ] && uploads="$(du -sh "$DIR/data/uploads" 2>/dev/null | awk '{print $1}')"
  if [ -f "$(nginx_conf_target)" ] && nginx_conf_ours "$(nginx_conf_target)"; then
    vhost="$(nginx_conf_target)"
  elif [ -f "$(nginx_conf_target)" ]; then
    vhost="foreign file (not touched)"
  else
    vhost="not written"
  fi
  if detect_cert "$(url_host "$u")" >/dev/null 2>&1; then
    cert_line="${CERT_EXPIRY:-present}"
  fi

  ui_panel_open "EVECHAT"
  ui_kv "Directory" "$DIR"
  ui_kv "Version" "$(version_of_binary)"
  ui_kv "Service" "$(service_state) · enabled: $(service_enabled)"
  ui_kv "Listen" "${h}:${p}"
  ui_kv "Public URL" "$u"
  ui_kv "Health" "$health"
  ui_kv "Trust proxy" "$t"
  ui_kv "Database" "${db:-unknown}"
  ui_kv "Uploads" "${uploads:-unknown}"
  ui_kv "nginx vhost" "$vhost"
  ui_kv "TLS" "$cert_line"
  ui_kv "Disk" "$(disk_summary)"
  ui_panel_close

  if [ "$PROMPT" = 1 ]; then
    if confirm_yes "Show the last 40 log lines?" "n"; then
      if systemd_available; then
        journalctl -u "$SERVICE_NAME" -n 40 --no-pager 2>/dev/null || true
      elif [ -f "$DIR/evechat.log" ]; then
        tail -n 40 "$DIR/evechat.log"
      fi
    fi
  fi
  return 0
}

flow_logs() {
  discover_install_dir
  if systemd_available; then
    journalctl -u "$SERVICE_NAME" -n 80 --no-pager 2>/dev/null || true
  elif [ -f "$DIR/evechat.log" ]; then
    tail -n 80 "$DIR/evechat.log"
  else
    ui_err "No service and no log file were found."
    return 1
  fi
}

flow_uninstall() {
  need_root
  platform_detect
  discover_install_dir
  if ! is_installed && [ ! -d "$DIR" ]; then
    ui_err "No EveChat installation was found in ${DIR}."
    return 1
  fi
  install_dir_safe || return 1
  if systemd_available && [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ] && ! unit_belongs; then
    ui_err "The unit ${SERVICE_NAME}.service was not created by this installer."
    ui_note "Refusing to stop or remove it. Remove it manually if it is yours."
    return 1
  fi

  ui_screen "UNINSTALL" "Remove EveChat, keep data"
  ui_panel_open "THIS WILL"
  ui_kv "Stop" "${SERVICE_NAME}.service"
  ui_kv "Remove" "service unit and binary"
  ui_kv "Remove" "EveChat nginx vhost (if generated)"
  ui_kv "Keep" "$DIR/data, .env and evechat.yaml"
  ui_panel_close
  ui_note "Use --purge to delete data and secrets as well."

  local answer="" had_marker=0
  if [ -f "$DIR/.evechat-install" ]; then
    had_marker=1
  fi
  if [ "$ASSUME_YES" = 1 ]; then
    ui_warn "Unattended mode: removing EveChat from ${DIR}."
  else
    ask_value answer "Type 'remove' to continue" ""
    if [ "$answer" != "remove" ]; then
      ui_note "Cancelled. Nothing changed."
      return 0
    fi
  fi

  if systemd_available; then
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    if unit_belongs; then
      rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
    fi
    systemctl daemon-reload 2>/dev/null || true
  fi

  remove_nginx_conf || true
  rm -f "$DIR/evechat" "$DIR/evechat.prev" "$DIR/.evechat.new" "$DIR/.evechat-install" "$DIR/evechat.example.yaml"
  ui_ok "Service and binary removed"

  if [ "$PURGE" = 1 ]; then
    answer=""
    if [ "$ASSUME_YES" != 1 ]; then
      ask_value answer "Type PURGE to delete all data and secrets" ""
    fi
    if [ "$answer" = "PURGE" ] || [ "$ASSUME_YES" = 1 ]; then
      case "$DIR" in
        /|/bin|/boot|/dev|/etc|/home|/lib|/lib64|/opt|/proc|/root|/run|/sbin|/srv|/sys|/tmp|/usr|/var)
          die "Refusing to purge ${DIR}."
          ;;
      esac
      if [ "$had_marker" = 1 ]; then
        rm -rf -- "$DIR"
        PUBLIC_URL=""
        PUBLIC_URL_SET=0
        PORT=""
        PORT_SET=0
        DOMAIN=""
        NET_MODE=""
        WRITE_NGINX=""
        ui_ok "Data, configuration and secrets deleted"
      else
        die "No install marker in ${DIR}; refusing to delete it."
      fi
    else
      ui_warn "Purge not confirmed; data was kept."
    fi
  else
    ui_ok "Data kept: ${DIR}/data, ${DIR}/.env, ${DIR}/evechat.yaml"
  fi
  return 0
}

# -----------------------------------------------------------------------------
#  Root checks, argument parsing and entrypoint
# -----------------------------------------------------------------------------
need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    ui_err "Root privileges are required for this action."
    ui_note "Re-run the same command with sudo, for example:"
    printf '    sudo bash install.sh\n'
    ui_note "Or:  curl -fsSL ${GITHUB}/raw/main/install.sh | sudo bash"
    exit 1
  fi
}

require_tools() {
  local missing=0
  command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || { ui_err "curl or wget is required."; missing=1; }
  command -v tar >/dev/null 2>&1 || { ui_err "tar is required."; missing=1; }
  command -v install >/dev/null 2>&1 || { ui_err "install (coreutils) is required."; missing=1; }
  command -v mktemp >/dev/null 2>&1 || { ui_err "mktemp is required."; missing=1; }
  if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
    ui_err "sha256sum or shasum is required."
    missing=1
  fi
  if [ "$missing" -ne 0 ]; then
    ui_note "Debian/Ubuntu: apt-get update && apt-get install -y curl tar coreutils"
    ui_note "RHEL/Fedora:   dnf install -y curl tar coreutils"
    exit 1
  fi
}

set_action() {
  if [ -n "$ACTION" ] && [ "$ACTION" != "$1" ]; then
    die "Choose only one action (got ${ACTION} and $1)."
  fi
  ACTION="$1"
}

parse_args() {
  local arg_count=$#
  while [ $# -gt 0 ]; do
    case "$1" in
      --install) set_action "install"; shift ;;
      --upgrade) set_action "upgrade"; shift ;;
      --nginx)
        set_action "nginx"
        WRITE_NGINX=1
        shift
        ;;
      --write-nginx)
        WRITE_NGINX=1
        shift
        ;;
      --no-nginx) WRITE_NGINX=0; shift ;;
      --status) set_action "status"; shift ;;
      --logs) set_action "logs"; shift ;;
      --uninstall) set_action "uninstall"; shift ;;
      --purge) PURGE=1; set_action "uninstall"; shift ;;
      --behind-proxy) NET_MODE="proxy"; NET_MODE_EXPLICIT=1; shift ;;
      --direct) NET_MODE="direct"; NET_MODE_EXPLICIT=1; shift ;;
      --local) NET_MODE="local"; NET_MODE_EXPLICIT=1; shift ;;
      --version) VERSION="${2:-}"; [ -n "$VERSION" ] || die "--version needs a value."; shift 2 ;;
      --version=*) VERSION="${1#*=}"; shift ;;
      --dir) DIR="${2:-}"; [ -n "$DIR" ] || die "--dir needs a value."; DIR_SET=1; shift 2 ;;
      --dir=*) DIR="${1#*=}"; [ -n "$DIR" ] || die "--dir needs a value."; DIR_SET=1; shift ;;
      --port) PORT="${2:-}"; [ -n "$PORT" ] || die "--port needs a value."; PORT_SET=1; shift 2 ;;
      --port=*) PORT="${1#*=}"; [ -n "$PORT" ] || die "--port needs a value."; PORT_SET=1; shift ;;
      --public-url) PUBLIC_URL="${2:-}"; [ -n "$PUBLIC_URL" ] || die "--public-url needs a value."; shift 2 ;;
      --public-url=*) PUBLIC_URL="${1#*=}"; shift ;;
      --domain) DOMAIN="${2:-}"; [ -n "$DOMAIN" ] || die "--domain needs a value."; shift 2 ;;
      --domain=*) DOMAIN="${1#*=}"; shift ;;
      --nginx-conf) NGINX_CONF="${2:-}"; [ -n "$NGINX_CONF" ] || die "--nginx-conf needs a value."; shift 2 ;;
      --nginx-conf=*) NGINX_CONF="${1#*=}"; shift ;;
      --user) SERVICE_USER="${2:-}"; [ -n "$SERVICE_USER" ] || die "--user needs a value."; USER_SET=1; shift 2 ;;
      --user=*) SERVICE_USER="${1#*=}"; [ -n "$SERVICE_USER" ] || die "--user needs a value."; USER_SET=1; shift ;;
      --base-url) BASE_URL="${2:-}"; [ -n "$BASE_URL" ] || die "--base-url needs a value."; shift 2 ;;
      --base-url=*) BASE_URL="${1#*=}"; [ -n "$BASE_URL" ] || die "--base-url needs a value."; shift ;;
      --admin-name) ADMIN_NAME="${2:-}"; shift 2 ;;
      --admin-name=*) ADMIN_NAME="${1#*=}"; shift ;;
      --admin-email) ADMIN_EMAIL="${2:-}"; shift 2 ;;
      --admin-email=*) ADMIN_EMAIL="${1#*=}"; shift ;;
      --admin-password) ADMIN_PASSWORD="${2:-}"; shift 2 ;;
      --admin-password=*) ADMIN_PASSWORD="${1#*=}"; shift ;;
      --yes|-y) ASSUME_YES=1; shift ;;
      --force) FORCE=1; shift ;;
      --skip-setup) SKIP_SETUP=1; shift ;;
      --no-systemd) NO_SYSTEMD=1; shift ;;
      --no-tui) NO_TUI=1; shift ;;
      --debug) set -x; shift ;;
      -h|--help) usage; exit 0 ;;
      --) shift; break ;;
      *) die "Unknown option: $1 (see --help)" ;;
    esac
  done
  if [ $# -gt 0 ]; then
    die "Unexpected argument: $1"
  fi
  # Flags such as --port or --domain are install requests, not a request for
  # the menu. Only a bare `install.sh` opens the menu.
  if [ -z "$ACTION" ] && [ "$arg_count" -gt 0 ]; then
    ACTION="install"
  fi
  BASE_URL="${BASE_URL%/}"
  [ -z "$PUBLIC_URL" ] || PUBLIC_URL="${PUBLIC_URL%/}"
  [ -z "$PUBLIC_URL" ] || PUBLIC_URL_SET=1
  [ "$DIR" = "/" ] || DIR="${DIR%/}"
  if [ -n "$NGINX_CONF" ]; then
    case "$NGINX_CONF" in
      /*) ;;
      *) die "--nginx-conf must be an absolute path." ;;
    esac
  fi
  if [ -n "$PUBLIC_URL" ]; then
    case "$PUBLIC_URL" in
      http://*|https://*) ;;
      *) die "--public-url must start with http:// or https://." ;;
    esac
  fi
  if [ -n "$PORT" ]; then
    case "$PORT" in
      ''|*[!0-9]*) die "Invalid port: ${PORT}" ;;
    esac
    if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
      die "Invalid port: ${PORT} (expected 1-65535)."
    fi
  fi
  case "$DIR" in
    /*) ;;
    *) die "The install directory must be absolute: ${DIR}" ;;
  esac
  # A domain is served through a reverse proxy; --write-nginx needs loopback.
  if [ -z "$NET_MODE" ] && [ -n "$PUBLIC_URL" ]; then
    case "$PUBLIC_URL" in
      https://*) NET_MODE="proxy" ;;
    esac
  fi
  if [ -n "$DOMAIN" ] && [ -z "$NET_MODE" ]; then
    NET_MODE="proxy"
  fi
  if [ -n "$DOMAIN" ] && [ "$NET_MODE" != "proxy" ]; then
    die "--domain requires proxy mode (use --behind-proxy, not --direct/--local)."
  fi
  if [ "$WRITE_NGINX" = 1 ]; then
    if [ -n "$NET_MODE" ] && [ "$NET_MODE" != "proxy" ]; then
      die "--write-nginx requires --behind-proxy (not --direct or --local)."
    fi
    NET_MODE="proxy"
    NET_MODE_EXPLICIT=1
  fi
}

cleanup() {
  local code=$?
  trap - EXIT
  restore_echo
  ui_show_cursor
  if [ -n "${TMPDIR_INSTALL:-}" ] && [ -d "$TMPDIR_INSTALL" ]; then
    rm -rf -- "$TMPDIR_INSTALL"
  fi
  exit "$code"
}

main() {
  open_tty
  UI_OUT_TTY=0
  [ -t 1 ] && UI_OUT_TTY=1
  if [ "${EVECHAT_FORCE_COLOR:-0}" = 1 ]; then
    UI_OUT_TTY=1
  fi
  if [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ] && [ "$UI_OUT_TTY" = 1 ]; then
    COLOR_ENABLED=1
  fi
  ui_init
  parse_args "$@"

  if [ "$NO_TUI" = 1 ]; then
    UI_OUT_TTY=0
    COLOR_ENABLED=0
    setup_colors
    setup_glyphs
  fi

  if [ -n "$TTY_FD" ] && [ "$UI_OUT_TTY" = 1 ] && [ "$NO_TUI" != 1 ]; then
    INTERACTIVE=1
  fi
  if [ "$INTERACTIVE" = 1 ] && [ "$ASSUME_YES" != 1 ]; then
    PROMPT=1
  fi

  if [ "${EVECHAT_UI_DEMO:-0}" = 1 ]; then
    platform_detect
    detect_environment
    render_main_menu 0
    return 0
  fi

  platform_detect

  if [ -z "$ACTION" ]; then
    if [ "$INTERACTIVE" = 1 ] && [ "$ASSUME_YES" != 1 ] && [ "$NO_TUI" != 1 ]; then
      main_menu
      return 0
    fi
    ACTION="install"
    ui_note "Non-interactive mode: installing with defaults. Use --help for flags."
  fi

  case "$ACTION" in
    install) flow_install ;;
    upgrade) flow_upgrade ;;
    nginx) flow_proxy ;;
    status) flow_status ;;
    logs) flow_logs ;;
    uninstall) flow_uninstall ;;
    *) die "Unknown action: ${ACTION}" ;;
  esac
}

trap cleanup EXIT
trap 'exit 130' INT TERM

if [ "${EVECHAT_LIB_ONLY:-0}" != 1 ]; then
  main "$@"
fi
