#!/usr/bin/env bash
set -euo pipefail

REPO="clofour/trellis-experimental"
INSTALL_DIR="/usr/local/bin"
SERVICE="trellis"

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33mwarning:\033[0m %s\n' "$*"; }
error() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# ── Preflight ────────────────────────────────────────────────────────

[ "$(uname -s)" = "Linux" ] || error "This script only supports Linux."
[ "$(uname -m)" = "x86_64" ] || error "This script only supports x86_64 (amd64)."
[ "$(id -u)" -eq 0 ] || error "Run this script as root (or with sudo)."

for cmd in curl tar systemctl; do
    command -v "$cmd" >/dev/null 2>&1 || error "Required command not found: $cmd"
done

[ -x "${INSTALL_DIR}/trellis" ] \
    || error "trellis is not installed at ${INSTALL_DIR}/trellis. Run setup.sh first."

# ── Current version ──────────────────────────────────────────────────

current_version="$("${INSTALL_DIR}/trellis" --version 2>/dev/null | awk '{print $NF}')" \
    || current_version="unknown"

# ── Latest release ───────────────────────────────────────────────────

info "Fetching latest release from GitHub..."
release_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")" \
    || error "Failed to find a release. Make sure the repository has at least one tagged release."

release_tag="$(printf '%s' "$release_json" \
    | grep -oP '"tag_name":\s*"\K[^"]+')" \
    || error "Latest release is missing a tag name."

bin_url="$(printf '%s' "$release_json" \
    | grep -oP '"browser_download_url":\s*"\K[^"]*trellis_linux_x64\.tar\.gz')" \
    || error "Release is missing the trellis_linux_x64.tar.gz asset."

# ── Version check ────────────────────────────────────────────────────

if [ "$current_version" = "$release_tag" ]; then
    info "Already at ${release_tag}; nothing to do."
    exit 0
fi

info "Upgrading trellis ${current_version} → ${release_tag}."
warn "For a cluster upgrade, drain this node before continuing:"
warn "  trellisctl nodes drain <id>"
warn "Undrain it after this script finishes:"
warn "  trellisctl nodes undrain <id>"
echo

# ── Download ─────────────────────────────────────────────────────────

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

info "Downloading ${bin_url}..."
curl -fSL -o "${tmp}/trellis_linux_x64.tar.gz" "$bin_url"
tar -xzf "${tmp}/trellis_linux_x64.tar.gz" -C "$tmp"

# ── Swap binaries ────────────────────────────────────────────────────

if systemctl is-active --quiet "$SERVICE" 2>/dev/null; then
    info "Stopping ${SERVICE}..."
    systemctl stop "$SERVICE"
    was_running=true
else
    was_running=false
fi

info "Installing binaries to ${INSTALL_DIR}..."
install -m 0755 "${tmp}/trellis"    "${INSTALL_DIR}/trellis"
install -m 0755 "${tmp}/trellisctl" "${INSTALL_DIR}/trellisctl"

if [ "$was_running" = true ]; then
    info "Starting ${SERVICE}..."
    systemctl start "$SERVICE"
    info "Check status with: sudo journalctl -u ${SERVICE} -f"
fi

info "Upgrade complete: ${current_version} → ${release_tag}."
