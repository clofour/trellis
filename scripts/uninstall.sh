#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/trellis/data"
CONFIG_DIR="/etc/trellis"
SERVICE_FILE="/etc/systemd/system/trellis.service"
RUN_DIR="/run/trellis"

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33mwarning:\033[0m %s\n' "$*"; }
error() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

confirm() {
    local prompt="$1" default="${2:-y}"
    if [ "$default" = "y" ]; then
        prompt="$prompt [Y/n] "
    else
        prompt="$prompt [y/N] "
    fi
    printf '%s' "$prompt"
    read -r answer </dev/tty
    answer="${answer:-$default}"
    case "$answer" in
        [Yy]*) return 0 ;;
        *) return 1 ;;
    esac
}

# ── Preflight checks ──────────────────────────────────────

[ "$(uname -s)" = "Linux" ] || error "This script only supports Linux."
[ "$(id -u)" -eq 0 ] || error "Run this script as root (or with sudo)."

if [ ! -x "${INSTALL_DIR}/trellis" ] && [ ! -f "$SERVICE_FILE" ]; then
    error "Trellis does not appear to be installed on this system."
fi

echo
warn "This will remove Trellis from this node."
warn "The cluster token in ${CONFIG_DIR} and all local Raft/TLS state"
warn "under ${DATA_DIR} will be permanently deleted."
echo
confirm "Continue with uninstall?" "n" || { info "Aborted."; exit 0; }

# ── Stop and disable the service ───────────────────────────

if systemctl is-active --quiet trellis 2>/dev/null; then
    info "Stopping trellis service..."
    systemctl stop trellis
fi

if systemctl is-enabled --quiet trellis 2>/dev/null; then
    info "Disabling trellis service..."
    systemctl disable trellis
fi

# ── Optionally stop and remove managed containers ─────────

if command -v ctr >/dev/null 2>&1; then
    container_ids="$(ctr -n trellis containers ls -q 2>/dev/null || true)"
    if [ -n "$container_ids" ]; then
        count="$(echo "$container_ids" | wc -l)"
        echo
        warn "Found ${count} container(s) in the trellis containerd namespace."
        warn "These were started by Trellis and may still be running."
        echo
        if confirm "Stop and remove these containers?" "n"; then
            for cid in $container_ids; do
                info "Stopping container ${cid}..."
                ctr -n trellis tasks kill "$cid" -s SIGTERM 2>/dev/null || true
                sleep 1
                ctr -n trellis tasks kill "$cid" -s SIGKILL 2>/dev/null || true
                ctr -n trellis tasks delete "$cid" 2>/dev/null || true
                info "Removing container ${cid}..."
                ctr -n trellis containers rm "$cid" 2>/dev/null || true
            done
            info "All Trellis containers removed."
        else
            info "Keeping containers. Remove them manually with: ctr -n trellis containers rm <id>"
        fi
    fi
fi

# ── Remove the systemd unit ───────────────────────────────

if [ -f "$SERVICE_FILE" ]; then
    info "Removing systemd unit ${SERVICE_FILE}..."
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
    systemctl reset-failed 2>/dev/null || true
fi

# ── Remove binaries ──────────────────────────────────────

info "Removing binaries..."
rm -f "${INSTALL_DIR}/trellis"
rm -f "${INSTALL_DIR}/trellisctl"

# ── Remove trellisctl context ──────────────────────────────

operator_user="${SUDO_USER:-root}"
if [ "$operator_user" = "root" ]; then
    operator_config_home="/root/.config"
else
    operator_home="$(getent passwd "$operator_user" | cut -d: -f6 2>/dev/null || true)"
    operator_config_home="${operator_home}/.config"
fi
if [ -n "${operator_config_home:-}" ] && [ -d "${operator_config_home}/trellis" ]; then
    info "Removing trellisctl context directory ${operator_config_home}/trellis..."
    rm -rf "${operator_config_home}/trellis"
fi

# ── Remove config directory ────────────────────────────────

if [ -d "$CONFIG_DIR" ]; then
    info "Removing config directory ${CONFIG_DIR}..."
    rm -rf "$CONFIG_DIR"
fi

# ── Remove runtime files ───────────────────────────────────

if [ -d "$RUN_DIR" ]; then
    info "Removing runtime directory ${RUN_DIR}..."
    rm -rf "$RUN_DIR"
fi

# ── Remove persistent data ──────────────────────────────────

if [ -d "$DATA_DIR" ]; then
    echo
    warn "The data directory ${DATA_DIR} contains Raft state, TLS keys,"
    warn "WireGuard identity material, and any named container volumes"
    warn "that were managed by this node."
    echo
    if confirm "Delete all persistent data in ${DATA_DIR}?" "n"; then
        info "Removing data directory ${DATA_DIR}..."
        rm -rf "$DATA_DIR"
        # Remove the parent /var/lib/trellis if it is now empty.
        rmdir /var/lib/trellis 2>/dev/null || true
    else
        info "Keeping ${DATA_DIR}. Remove it manually when no longer needed."
    fi
fi

# ── Optional: remove apt packages installed by setup.sh ───────────

if command -v apt-get >/dev/null 2>&1; then
    removals=()

    if dpkg -l runsc >/dev/null 2>&1; then
        if confirm "Remove gVisor (runsc) installed by Trellis setup?" "n"; then
            removals+=(runsc)
        fi
    fi

    if dpkg -l containerd.io >/dev/null 2>&1; then
        if confirm "Remove containerd.io installed by Trellis setup?" "n"; then
            removals+=(containerd.io)
        fi
    fi

    if dpkg -l wireguard-tools >/dev/null 2>&1; then
        if confirm "Remove wireguard-tools installed by Trellis setup?" "n"; then
            removals+=(wireguard-tools)
        fi
    fi

    if [ "${#removals[@]}" -gt 0 ]; then
        info "Removing packages: ${removals[*]}..."
        apt-get remove -y "${removals[@]}"
    fi

    # Remove apt sources that setup.sh added, only if the matching package is gone.
    if ! dpkg -l containerd.io >/dev/null 2>&1; then
        if [ -f /etc/apt/keyrings/docker.asc ] \
            || [ -f /etc/apt/sources.list.d/docker.sources ]; then
            if confirm "Remove the Docker apt repository added by Trellis setup?" "n"; then
                rm -f /etc/apt/keyrings/docker.asc \
                      /etc/apt/sources.list.d/docker.sources
                apt-get update -qq
            fi
        fi
    fi

    if ! dpkg -l runsc >/dev/null 2>&1; then
        if [ -f /usr/share/keyrings/gvisor-archive-keyring.gpg ] \
            || [ -f /etc/apt/sources.list.d/gvisor.list ]; then
            if confirm "Remove the gVisor apt repository added by Trellis setup?" "n"; then
                rm -f /usr/share/keyrings/gvisor-archive-keyring.gpg \
                      /etc/apt/sources.list.d/gvisor.list
                apt-get update -qq
            fi
        fi
    fi
fi

echo
info "Trellis has been uninstalled."
