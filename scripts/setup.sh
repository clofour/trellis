#!/usr/bin/env bash
set -euo pipefail

REPO="clofour/trellis-experimental"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/trellis/data"
CONFIG_DIR="/etc/trellis"
CONFIG_FILE="${CONFIG_DIR}/trellis.yaml"
SERVICE_FILE="/etc/systemd/system/trellis.service"

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33mwarning:\033[0m %s\n' "$*"; }
error() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

confirm() {
    local prompt="$1" default="${2:-y}"
    if [ "$default" = "y" ]; then prompt="$prompt [Y/n] "; else prompt="$prompt [y/N] "; fi
    printf '%s' "$prompt"
    read -r answer </dev/tty
    answer="${answer:-$default}"
    case "$answer" in [Yy]*) return 0 ;; *) return 1 ;; esac
}

prompt() {
    local var_name="$1" prompt_text="$2" default="$3"
    printf '%s [%s] ' "$prompt_text" "$default"
    read -r value </dev/tty
    value="${value:-$default}"
    printf -v "$var_name" '%s' "$value"
}

prompt_secret() {
    local var_name="$1" prompt_text="$2" value
    printf '%s: ' "$prompt_text"
    read -r -s value </dev/tty
    printf '\n'
    [ -n "$value" ] || error "$prompt_text is required."
    printf -v "$var_name" '%s' "$value"
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
    [ "$a" -eq 10 ] || { [ "$a" -eq 172 ] && [ "$b" -ge 16 ] && [ "$b" -le 31 ]; } || { [ "$a" -eq 192 ] && [ "$b" -eq 168 ]; }
}

detect_private_ipv4() {
    local value
    if command -v ip >/dev/null 2>&1; then
        value="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{ for (i = 1; i <= NF; i++) if ($i == "src") { print $(i + 1); exit } }')"
        if [ -n "$value" ] && is_private_ipv4 "$value"; then printf '%s\n' "$value"; return 0; fi
        while read -r value; do
            if is_private_ipv4 "$value"; then printf '%s\n' "$value"; return 0; fi
        done < <(ip -o -4 addr show scope global 2>/dev/null | awk '{ sub(/\/.*/, "", $4); print $4 }')
    fi
    while read -r value; do
        if is_private_ipv4 "$value"; then printf '%s\n' "$value"; return 0; fi
    done < <(hostname -I 2>/dev/null | tr ' ' '\n')
    return 1
}

detect_public_ipv4() {
    local value
    value="$(curl -4fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
    if is_ipv4 "$value"; then printf '%s\n' "$value"; return 0; fi
    return 1
}

detect_distro() {
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        DISTRO_ID="${ID:-}"
        DISTRO_CODENAME="${VERSION_CODENAME:-}"
    fi
    [ -n "${DISTRO_ID:-}" ] || error "Cannot detect Linux distribution. Only Debian/Ubuntu are supported."
    case "$DISTRO_ID" in debian|ubuntu) ;; *) error "Unsupported distribution: $DISTRO_ID. Only Debian and Ubuntu are supported." ;; esac
}

install_containerd() {
    info "Installing containerd from the Docker apt repository..."
    detect_distro
    apt-get install -y -qq ca-certificates curl
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL "https://download.docker.com/linux/${DISTRO_ID}/gpg" -o /etc/apt/keyrings/docker.asc
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
    containerd config default > /etc/containerd/config.toml
    systemctl enable --now containerd
}

install_gvisor() {
    info "Installing gVisor..."
    detect_distro
    curl -fsSL https://gvisor.dev/archive.key | gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" > /etc/apt/sources.list.d/gvisor.list
    apt-get update -qq
    apt-get install -y -qq runsc
    runsc install
    systemctl restart containerd
}

[ "$(uname -s)" = "Linux" ] || error "This script only supports Linux."
[ "$(uname -m)" = "x86_64" ] || error "This script only supports x86_64 (amd64)."
[ "$(id -u)" -eq 0 ] || error "Run this script as root (or with sudo)."
for cmd in curl tar systemctl; do command -v "$cmd" >/dev/null 2>&1 || error "Required command not found: $cmd"; done

if [ -x "${INSTALL_DIR}/trellis" ]; then
    installed_version="$("${INSTALL_DIR}/trellis" --version 2>/dev/null | awk '{print $NF}')" || installed_version="unknown"
    error "Trellis is already installed (${installed_version}). To upgrade, run scripts/upgrade.sh instead."
fi

if ! systemctl is-active --quiet containerd 2>/dev/null; then
    if command -v containerd >/dev/null 2>&1; then
        systemctl enable --now containerd
    else
        confirm "Install containerd automatically?" "y" || error "containerd is required."
        install_containerd
    fi
fi

info "Fetching latest release from GitHub..."
release_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")" || error "Failed to find a release."
release_tag="$(printf '%s' "$release_json" | grep -oP '"tag_name":\s*"\K[^"]+')" || error "Latest release is missing a tag name."
ui_image="ghcr.io/clofour/trellis-ui:${release_tag}"
bin_url="$(printf '%s' "$release_json" | grep -oP '"browser_download_url":\s*"\K[^"]*trellis_linux_x64\.tar\.gz')" || error "Release is missing the Linux x64 asset."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fSL -o "${tmp}/trellis_linux_x64.tar.gz" "$bin_url"
tar -xzf "${tmp}/trellis_linux_x64.tar.gz" -C "$tmp"
install -m 0755 "${tmp}/trellis" "${INSTALL_DIR}/trellis"
install -m 0755 "${tmp}/trellisctl" "${INSTALL_DIR}/trellisctl"

info "Creating data directory at ${DATA_DIR}..."
install -d -m 0750 "$DATA_DIR"
install -d -m 0750 "$CONFIG_DIR"

default_hostname="$(hostname)"
prompt advertise_host "Advertise hostname or IP (reachable by other nodes; public/private to auto-detect)" "$default_hostname"
case "$advertise_host" in
    private) advertise_host="$(detect_private_ipv4)" || error "Could not detect a private IPv4 address." ;;
    public)
        advertise_host="$(detect_public_ipv4)" || error "Could not detect the public IPv4 address."
        warn "Ensure NAT, firewall, and port forwarding allow other nodes to reach it."
        ;;
esac

join_addr=""
if confirm "Join an existing cluster?" "n"; then
    prompt join_addr "Address of an existing cluster node (host:8128)" ""
    [ -n "$join_addr" ] || error "An existing cluster node address is required."
fi

if [ -n "$join_addr" ]; then
    prompt_secret cluster_token "Bootstrap token for the existing cluster"
else
    cluster_token="trls_boot_$(head -c 32 /dev/urandom | base64 | tr -d '=\n')"
fi

info "Writing node configuration to ${CONFIG_FILE}..."
cat > "$CONFIG_FILE" <<EOF
cluster: default
bootstrap_token: ${cluster_token}
data_dir: ${DATA_DIR}
agent_advertise: ${advertise_host}:8127
server_advertise: ${advertise_host}:8128
raft_advertise: ${advertise_host}:8129
EOF
if [ -n "$join_addr" ]; then
    printf 'join: %s\n' "$join_addr" >> "$CONFIG_FILE"
fi
chmod 600 "$CONFIG_FILE"

info "Writing systemd unit to ${SERVICE_FILE}..."
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Trellis node
After=containerd.service network-online.target
Wants=containerd.service network-online.target

[Service]
ExecStart=${INSTALL_DIR}/trellis --config ${CONFIG_FILE}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now trellis
info "trellis is running."

# The node is now usable. Optional runtime/networking features come afterwards
# so a first install does not front-load concepts that are not needed to deploy.
if confirm "Enable namespace networking on this node?" "n"; then
    info "Installing WireGuard dependencies for Trellis namespace networking..."
    apt-get update -qq
    apt-get install -y -qq wireguard-tools iproute2 iptables >/dev/null
    if ! command -v containerd-shim-runsc-v1 >/dev/null 2>&1; then
        confirm "Install gVisor automatically?" "y" || error "gVisor is required for namespace-networked jobs."
        install_gvisor
    fi
    info "Namespace networking dependencies are installed. Configure wireguard_endpoint, wireguard_port, or wireguard_pool in ${CONFIG_FILE} when non-default values are required, then restart trellis."
fi

operator_token=""
for attempt in $(seq 1 30); do
    if operator_token="$("${INSTALL_DIR}/trellisctl" --output table credentials create --scope cluster --access write 2>/dev/null)" && [ -n "$operator_token" ]; then
        break
    fi
    operator_token=""
    sleep 1
done
[ -n "$operator_token" ] || error "Trellis started, but a normal operator credential could not be created."

operator_user="${SUDO_USER:-root}"
if [ "$operator_user" = "root" ]; then
    operator_home="/root"
    operator_group="root"
else
    operator_home="$(getent passwd "$operator_user" | cut -d: -f6)"
    operator_group="$(id -gn "$operator_user")"
    [ -n "$operator_home" ] || error "Could not determine home directory for ${operator_user}."
fi
operator_config_home="${operator_home}/.config"
if [ ! -d "$operator_config_home" ]; then
    install -d -m 0700 -o "$operator_user" -g "$operator_group" "$operator_config_home"
fi
HOME="$operator_home" XDG_CONFIG_HOME="$operator_config_home" \
    "${INSTALL_DIR}/trellisctl" --token "$operator_token" --namespace default context save local --use >/dev/null
if [ "$operator_user" != "root" ]; then
    chown -R "${operator_user}:${operator_group}" "${operator_config_home}/trellis"
fi
unset operator_token
info "Saved a scoped cluster/write context for ${operator_user}; normal trellisctl commands no longer need sudo."

install_ui=false
ui_namespace=""
if confirm "Install the web dashboard?" "n"; then
    prompt ui_namespace "Dashboard default namespace" "default"
    [[ "$ui_namespace" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$ ]] || error "Dashboard namespace must be a safe identifier."

    dashboard_mode="r"
    echo "  Dashboard authorization:"
    echo "    r  — cluster/read  (observe cluster state; no mutations)"
    echo "    rw — cluster/write (also apply/delete jobs, drain nodes, and manage secrets)"
    printf 'Dashboard mode [r/rw] (default: r): '
    read -r _mode_input </dev/tty
    _mode_input="${_mode_input:-r}"
    case "$_mode_input" in rw|RW|r/w|R/W) dashboard_mode="rw" ;; r|R) dashboard_mode="r" ;; *) warn "Unknown mode; using read-only." ;; esac

    dashboard_access="read"
    allow_writes_env=""
    if [ "$dashboard_mode" = "rw" ]; then
        dashboard_access="write"
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
    api_access:
      scope: cluster
      access: ${dashboard_access}
    tasks:
      - name: dashboard
        image: ${ui_image}
        env:
          TRELLIS_NAMESPACE: ${ui_namespace}
${allow_writes_env}        resources:
          cpu: 250
          memory: 512MiB
        networking:
          mode: host
          ports:
            - port: 3000
        health_check:
          type: http
          port: 3000
          path: /
EOF

    info "Deploying dashboard as ${ui_namespace}/trellis-dashboard..."
    dashboard_applied=false
    for attempt in $(seq 1 30); do
        if "${INSTALL_DIR}/trellisctl" --token "$cluster_token" jobs apply --file "$dashboard_manifest" >/dev/null 2>&1; then
            dashboard_applied=true
            break
        fi
        sleep 2
    done
    [ "$dashboard_applied" = true ] || error "Failed to deploy the dashboard."
    if [ "$dashboard_mode" = "rw" ]; then
        warn "The dashboard holds a cluster/write credential. Protect port 3000 with your own HTTPS and identity-aware proxy."
    fi
    install_ui=true
fi

unset cluster_token
echo
info "Setup complete!"
info "Node configuration: ${CONFIG_FILE}"
info "Verify the cluster: trellisctl nodes list"
info "Start the tutorial: follow docs/public/getting-started.md"
if [ "$install_ui" = true ]; then
    info "Dashboard status: trellisctl --namespace ${ui_namespace} jobs status trellis-dashboard"
    info "Dashboard listens on port 3000 of its allocation node."
fi
