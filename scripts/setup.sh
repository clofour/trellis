#!/usr/bin/env bash
set -euo pipefail

REPO="clofour/trellis-experimental"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/trellis/data"
CONFIG_DIR="/etc/trellis"
ENV_FILE="${CONFIG_DIR}/trellis.env"
SERVICE_FILE="/etc/systemd/system/trellis-node.service"
UI_DIR="/opt/trellis/ui"
UI_SERVICE_FILE="/etc/systemd/system/trellis-ui.service"
UI_USER="trellis-ui"

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

prompt() {
    local var_name="$1" prompt_text="$2" default="$3"
    printf '%s [%s] ' "$prompt_text" "$default"
    read -r value </dev/tty
    value="${value:-$default}"
    eval "$var_name=\$value"
}

# ── Preflight checks ────────────────────────────────────────────────

[ "$(uname -s)" = "Linux" ] || error "This script only supports Linux."
[ "$(uname -m)" = "x86_64" ] || error "This script only supports x86_64 (amd64)."
[ "$(id -u)" -eq 0 ] || error "Run this script as root (or with sudo)."

for cmd in curl tar systemctl; do
    command -v "$cmd" >/dev/null 2>&1 || error "Required command not found: $cmd"
done

if ! systemctl is-active --quiet containerd; then
    error "containerd is not running. Install and start containerd before running this script."
fi

# ── Download binaries from latest release ────────────────────────────

info "Fetching latest release from GitHub..."
release_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")" \
    || error "Failed to find a release. Make sure the repository has at least one tagged release."

bin_url="$(printf '%s' "$release_json" \
    | grep -oP '"browser_download_url":\s*"\K[^"]*trellis_linux_x64\.tar\.gz')" \
    || error "Release is missing the trellis_linux_x64.tar.gz asset."

info "Downloading ${bin_url}..."
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fSL -o "${tmp}/trellis_linux_x64.tar.gz" "$bin_url"

info "Installing binaries to ${INSTALL_DIR}..."
tar -xzf "${tmp}/trellis_linux_x64.tar.gz" -C "$tmp"
install -m 0755 "${tmp}/trellis-node" "${INSTALL_DIR}/trellis-node"
install -m 0755 "${tmp}/trellis"      "${INSTALL_DIR}/trellis"

# ── WireGuard ────────────────────────────────────────────────────────

install_wireguard=false
if confirm "Enable WireGuard namespace networking?" "n"; then
    install_wireguard=true
    info "Installing WireGuard dependencies..."
    apt-get update -qq
    apt-get install -y -qq wireguard-tools iproute2 iptables >/dev/null
    info "WireGuard dependencies installed."
    if ! command -v containerd-shim-runsc-v1 >/dev/null 2>&1; then
        warn "containerd-shim-runsc-v1 (gVisor) not found."
        warn "WireGuard networking requires the gVisor containerd shim."
        warn "Install it from https://gvisor.dev/docs/user_guide/install/ before using WireGuard jobs."
    fi
fi

# ── Data directory and token ─────────────────────────────────────────

info "Creating data directory at ${DATA_DIR}..."
install -d -m 0750 "$DATA_DIR"
install -d -m 0750 "$CONFIG_DIR"

if [ -f "$ENV_FILE" ]; then
    info "Existing token found at ${ENV_FILE}, keeping it."
else
    info "Generating cluster token..."
    token="$(head -c 32 /dev/urandom | base64)"
    printf 'TRELLIS_TOKEN=%s\n' "$token" > "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    info "Token written to ${ENV_FILE}."
    info "Copy this token to every node in the cluster."
fi

# ── Advertise addresses ──────────────────────────────────────────────

default_hostname="$(hostname)"
prompt advertise_host "Advertise hostname or IP (reachable by other nodes)" "$default_hostname"

# ── Join an existing cluster? ────────────────────────────────────────

join_flag=""
if confirm "Join an existing cluster?" "n"; then
    prompt join_addr "Address of an existing cluster node (host:8128)" ""
    if [ -n "$join_addr" ]; then
        join_flag="--join ${join_addr}"
    fi
fi

# ── systemd unit ─────────────────────────────────────────────────────

wireguard_flags=""
if [ "$install_wireguard" = true ]; then
    wireguard_flags="--wireguard-pool 10.64.0.0/10 \\\\\n  --wireguard-endpoint ${advertise_host}:51820 \\\\\n  --wireguard-port 51820"
fi

info "Writing systemd unit to ${SERVICE_FILE}..."
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Trellis node
After=containerd.service network-online.target
Wants=containerd.service network-online.target

[Service]
EnvironmentFile=${ENV_FILE}
ExecStart=${INSTALL_DIR}/trellis-node \\
  --data-dir ${DATA_DIR} \\
  --agent-advertise ${advertise_host}:8127 \\
  --server-advertise ${advertise_host}:8128 \\
  --raft-advertise ${advertise_host}:8129 \\
  --cluster-token \${TRELLIS_TOKEN}$([ -n "$join_flag" ] && printf ' \\\n  %s' "$join_flag")
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

if confirm "Start trellis-node now?" "y"; then
    systemctl enable --now trellis-node
    info "trellis-node is running."
    echo
    info "Check status with: sudo journalctl -u trellis-node -f"
else
    systemctl enable trellis-node
    info "trellis-node is enabled but not started."
    info "Start it with: sudo systemctl start trellis-node"
fi

# ── Dashboard (optional) ─────────────────────────────────────────────

install_ui=false
if confirm "Install the web dashboard?" "n"; then
    if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
        error "Node.js and npm are required for the dashboard. Install Node.js 20+ and re-run."
    fi

    node_major="$(node -e 'process.stdout.write(process.versions.node.split(".")[0])')"
    if [ "$node_major" -lt 20 ]; then
        error "Node.js 20 or later is required (found v$(node --version))."
    fi

    ui_url="$(printf '%s' "$release_json" \
        | grep -oP '"browser_download_url":\s*"\K[^"]*trellis_ui\.tar\.gz')" \
        || error "Release is missing the trellis_ui.tar.gz asset."

    info "Downloading dashboard source..."
    curl -fSL -o "${tmp}/trellis_ui.tar.gz" "$ui_url"

    info "Installing dashboard to ${UI_DIR}..."
    install -d -m 0755 "$UI_DIR"
    tar -xzf "${tmp}/trellis_ui.tar.gz" -C "$UI_DIR"

    info "Installing npm dependencies and building..."
    (cd "$UI_DIR" && npm ci --ignore-scripts --no-audit --no-fund && npm run build) \
        || error "Dashboard build failed."

    prompt api_url "Trellis leader API URL" "http://${advertise_host}:8128"

    cluster_token="$(grep -oP '(?<=TRELLIS_TOKEN=).+' "$ENV_FILE")"
    cat > "${UI_DIR}/.env.local" <<ENVEOF
TRELLIS_API_URL=${api_url}
TRELLIS_API_TOKEN=${cluster_token}
ENVEOF

    if ! id "$UI_USER" >/dev/null 2>&1; then
        useradd --system --no-create-home --shell /usr/sbin/nologin "$UI_USER"
    fi
    chown -R "${UI_USER}:${UI_USER}" "$UI_DIR"
    chmod 600 "${UI_DIR}/.env.local"

    info "Writing systemd unit to ${UI_SERVICE_FILE}..."
    cat > "$UI_SERVICE_FILE" <<EOF
[Unit]
Description=Trellis dashboard
After=network-online.target trellis-node.service
Wants=network-online.target

[Service]
Type=simple
User=${UI_USER}
Group=${UI_USER}
WorkingDirectory=${UI_DIR}
Environment=NODE_ENV=production
ExecStart=/usr/bin/npm run start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload

    if confirm "Start the dashboard now?" "y"; then
        systemctl enable --now trellis-ui
        info "Dashboard is running at http://localhost:3000"
    else
        systemctl enable trellis-ui
        info "Dashboard is enabled but not started."
        info "Start it with: sudo systemctl start trellis-ui"
    fi

    install_ui=true
fi

echo
info "Setup complete!"
info "CLI usage: trellis --server-addr ${advertise_host}:8128 --cluster-token \"\$(. ${ENV_FILE} && echo \$TRELLIS_TOKEN)\" nodes list"
if [ "$install_ui" = true ]; then
    info "Dashboard: http://localhost:3000"
fi
