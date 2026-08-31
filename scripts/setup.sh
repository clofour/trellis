#!/usr/bin/env bash
set -euo pipefail

REPO="clofour/trellis-experimental"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/trellis/data"
CONFIG_DIR="/etc/trellis"
ENV_FILE="${CONFIG_DIR}/trellis.env"
SERVICE_FILE="/etc/systemd/system/trellis-node.service"

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

is_ipv4() {
    local value="$1" octet
    local -a octets
    IFS=. read -r -a octets <<< "$value"
    [ "${#octets[@]}" -eq 4 ] || return 1
    for octet in "${octets[@]}"; do
        [[ "$octet" =~ ^[0-9]+$ ]] || return 1
        (( 10#$octet <= 255 )) || return 1
    done
}

is_private_ipv4() {
    local value="$1" a b c d
    is_ipv4 "$value" || return 1
    IFS=. read -r a b c d <<< "$value"
    [ "$a" -eq 10 ] \
        || { [ "$a" -eq 172 ] && [ "$b" -ge 16 ] && [ "$b" -le 31 ]; } \
        || { [ "$a" -eq 192 ] && [ "$b" -eq 168 ]; }
}

detect_private_ipv4() {
    local value

    if command -v ip >/dev/null 2>&1; then
        value="$(ip -4 route get 1.1.1.1 2>/dev/null \
            | awk '{ for (i = 1; i <= NF; i++) if ($i == "src") { print $(i + 1); exit } }')"
        if [ -n "$value" ] && is_private_ipv4 "$value"; then
            printf '%s\n' "$value"
            return 0
        fi

        while read -r value; do
            if is_private_ipv4 "$value"; then
                printf '%s\n' "$value"
                return 0
            fi
        done < <(ip -o -4 addr show scope global 2>/dev/null \
            | awk '{ sub(/\/.*/, "", $4); print $4 }')
    fi

    while read -r value; do
        if is_private_ipv4 "$value"; then
            printf '%s\n' "$value"
            return 0
        fi
    done < <(hostname -I 2>/dev/null | tr ' ' '\n')

    return 1
}

detect_public_ipv4() {
    local value
    value="$(curl -4fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
    if is_ipv4 "$value"; then
        printf '%s\n' "$value"
        return 0
    fi
    return 1
}

# ── Distro helpers ───────────────────────────────────────────────────

# Sets DISTRO_ID (e.g. "ubuntu", "debian") and DISTRO_CODENAME.
detect_distro() {
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        DISTRO_ID="${ID:-}"
        DISTRO_CODENAME="${VERSION_CODENAME:-}"
    fi
    if [ -z "${DISTRO_ID:-}" ]; then
        error "Cannot detect Linux distribution. Only Debian/Ubuntu are supported."
    fi
    case "$DISTRO_ID" in
        debian|ubuntu) ;;
        *) error "Unsupported distribution: $DISTRO_ID. Only Debian and Ubuntu are supported." ;;
    esac
}

install_containerd() {
    info "Installing containerd from the Docker apt repository..."
    detect_distro
    apt-get install -y -qq ca-certificates curl
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL "https://download.docker.com/linux/${DISTRO_ID}/gpg" \
        -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    cat > /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/${DISTRO_ID}
Suites: ${DISTRO_CODENAME}
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF
    apt-get update -qq
    apt-get install -y -qq containerd.io
    # Apply a default config so containerd starts cleanly.
    containerd config default > /etc/containerd/config.toml
    systemctl enable --now containerd
    info "containerd installed and started."
}

install_gvisor() {
    info "Installing gVisor (runsc + containerd shim) from the official apt repository..."
    detect_distro
    curl -fsSL https://gvisor.dev/archive.key \
        | gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] \
https://storage.googleapis.com/gvisor/releases release main" \
        > /etc/apt/sources.list.d/gvisor.list
    apt-get update -qq
    apt-get install -y -qq runsc
    # Register the runsc runtime with containerd.
    runsc install
    systemctl restart containerd
    info "gVisor installed and containerd restarted."
}

# ── Preflight checks ────────────────────────────────────────────────

[ "$(uname -s)" = "Linux" ] || error "This script only supports Linux."
[ "$(uname -m)" = "x86_64" ] || error "This script only supports x86_64 (amd64)."
[ "$(id -u)" -eq 0 ] || error "Run this script as root (or with sudo)."

for cmd in curl tar systemctl; do
    command -v "$cmd" >/dev/null 2>&1 || error "Required command not found: $cmd"
done

if [ -x "${INSTALL_DIR}/trellis-node" ]; then
    installed_version="$("${INSTALL_DIR}/trellis-node" --version 2>/dev/null | awk '{print $NF}')" \
        || installed_version="unknown"
    error "Trellis is already installed (${installed_version}). To upgrade, run scripts/upgrade.sh instead."
fi

if ! systemctl is-active --quiet containerd 2>/dev/null; then
    if command -v containerd >/dev/null 2>&1; then
        info "containerd is installed but not running. Starting it..."
        systemctl enable --now containerd
    else
        info "containerd is not installed."
        confirm "Install containerd automatically?" "y" \
            || error "containerd is required. Install it manually and re-run this script."
        install_containerd
    fi
fi

# ── Download binaries from latest release ────────────────────────────

info "Fetching latest release from GitHub..."
release_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")" \
    || error "Failed to find a release. Make sure the repository has at least one tagged release."

release_tag="$(printf '%s' "$release_json" \
    | grep -oP '"tag_name":\s*"\K[^"]+')" \
    || error "Latest release is missing a tag name."
ui_image="ghcr.io/clofour/trellis-ui:${release_tag}"

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
        info "gVisor (containerd-shim-runsc-v1) is required for WireGuard networking."
        confirm "Install gVisor automatically?" "y" \
            || error "gVisor is required for WireGuard jobs. Install it from https://gvisor.dev/docs/user_guide/install/ and re-run."
        install_gvisor
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
prompt advertise_host "Advertise hostname or IP (reachable by other nodes; public/private to auto-detect)" "$default_hostname"
case "$advertise_host" in
    private)
        advertise_host="$(detect_private_ipv4)" \
            || error "Could not detect a private IPv4 address. Enter the address explicitly instead."
        info "Resolved 'private' to ${advertise_host}."
        ;;
    public)
        advertise_host="$(detect_public_ipv4)" \
            || error "Could not detect the public IPv4 address. Enter the address explicitly instead."
        info "Resolved 'public' to ${advertise_host}."
        warn "Public IP discovery reports this node's egress IPv4. Ensure NAT, firewall, and port forwarding allow other nodes to reach it."
        ;;
esac

# ── Join an existing cluster? ────────────────────────────────────────

join_flag=""
join_addr=""
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
ui_namespace=""
if confirm "Install the web dashboard?" "n"; then
    prompt ui_namespace "Dashboard namespace (jobs and allocations shown by the dashboard)" "default"
    [[ "$ui_namespace" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$ ]] \
        || error "Dashboard namespace must be a safe identifier (1-63 letters, numbers, '_', '.', or '-')."

    dashboard_server_addr="${join_addr:-${advertise_host}:8128}"
    prompt dashboard_server_addr "Trellis leader API address (host:port)" "$dashboard_server_addr"

    dashboard_mode="r"
    echo
    echo "  Dashboard access mode:"
    echo "    r  — read-only  (view jobs, nodes, allocations; no changes)"
    echo "    rw — read-write (also create, edit, and stop jobs via the UI)"
    echo
    printf 'Dashboard mode [r/rw] (default: r): '
    read -r _mode_input </dev/tty
    _mode_input="${_mode_input:-r}"
    case "$_mode_input" in
        rw|RW|r/w|R/W) dashboard_mode="rw" ;;
        r|R)            dashboard_mode="r" ;;
        *) warn "Unrecognised value '${_mode_input}'; defaulting to read-only." ; dashboard_mode="r" ;;
    esac

    if ! systemctl is-active --quiet trellis-node 2>/dev/null && [ -z "$join_addr" ]; then
        error "Cannot deploy the dashboard before trellis-node is running. Start trellis-node and re-run setup."
    fi

    allow_writes_env=""
    if [ "$dashboard_mode" = "rw" ]; then
        allow_writes_env="          TRELLIS_ALLOW_WRITES: \"true\"
"
    fi

    dashboard_manifest="${tmp}/trellis-dashboard.yaml"
    cat > "$dashboard_manifest" <<EOF
namespace: ${ui_namespace}
name: trellis-dashboard
task_groups:
  - name: web
    count: 1
    api_access: true
    tasks:
      - name: dashboard
        image: ${ui_image}
        env:
          TRELLIS_NAMESPACE: ${ui_namespace}
${allow_writes_env}        resources:
          cpu: 250
          memory: 536870912
        ports:
          - host_port: 3000
            container_port: 3000
        health_check:
          type: http
          port: 3000
          path: /
EOF

    cluster_token="$(grep -oP '(?<=TRELLIS_TOKEN=).+' "$ENV_FILE")"
    info "Deploying dashboard as Trellis job ${ui_namespace}/trellis-dashboard..."
    dashboard_applied=false
    for attempt in $(seq 1 30); do
        if "${INSTALL_DIR}/trellis" \
            --server-addr "$dashboard_server_addr" \
            --cluster-token "$cluster_token" \
            jobs apply --file "$dashboard_manifest" >/dev/null 2>&1; then
            dashboard_applied=true
            break
        fi
        sleep 2
    done
    [ "$dashboard_applied" = true ] \
        || error "Failed to deploy the dashboard through Trellis at ${dashboard_server_addr}."

    if [ "$dashboard_mode" = "rw" ]; then
        warn "Dashboard is configured in read-write mode. Anyone who can reach port 3000 can modify jobs."
    fi
    info "Dashboard job submitted. Trellis will pull ${ui_image} and keep it running."
    install_ui=true
fi

echo
info "Setup complete!"
info "CLI usage: trellis --server-addr ${advertise_host}:8128 --cluster-token \"\$(. ${ENV_FILE} && echo \$TRELLIS_TOKEN)\" nodes list"
if [ "$install_ui" = true ]; then
    info "Dashboard status: trellis --server-addr ${dashboard_server_addr} --cluster-token \"\$(. ${ENV_FILE} && echo \$TRELLIS_TOKEN)\" --namespace ${ui_namespace} jobs status trellis-dashboard"
    info "Dashboard listens on port 3000 of the node where Trellis places its allocation."
fi
