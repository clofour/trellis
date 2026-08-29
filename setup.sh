#!/usr/bin/env bash

# Install a single-node, systemd-managed Trellis deployment on Debian or Ubuntu.
set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_UI=1
INSTALL_WIREGUARD=0

usage() {
  cat <<'EOF'
Usage: sudo ./setup.sh [options]

Options:
  --skip-ui         Install the node and CLI without the dashboard
  --with-wireguard  Install WireGuard tools (the runsc shim is still required)
  -h, --help        Show this help

Environment overrides:
  TRELLIS_TOKEN       Reuse a cluster token instead of generating one
  TRELLIS_DATA_DIR    Node data directory (default: /var/lib/trellis/data)
  TRELLIS_UI_DIR      Dashboard install directory (default: /opt/trellis/ui)
  TRELLIS_API_URL     Dashboard API URL (default: http://127.0.0.1:8128)
  TRELLIS_ADVERTISE   Hostname or address advertised to other nodes
EOF
}

while (($#)); do
  case "$1" in
    --skip-ui) INSTALL_UI=0 ;;
    --with-wireguard) INSTALL_WIREGUARD=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

if [[ ${EUID} -ne 0 ]]; then
  echo "Run this installer as root (for example: sudo ./setup.sh)." >&2
  exit 1
fi

if [[ ! -r /etc/os-release ]]; then
  echo "This installer supports Debian and Ubuntu systems with systemd." >&2
  exit 1
fi
. /etc/os-release
case "${ID:-}" in
  debian|ubuntu) ;;
  *) echo "Unsupported distribution: ${ID:-unknown} (expected Debian or Ubuntu)." >&2; exit 1 ;;
esac
command -v systemctl >/dev/null || { echo "systemd is required." >&2; exit 1; }

command -v go >/dev/null || {
  echo "go is required to build Trellis; install Go 1.26+ and retry." >&2
  exit 1
}
go_version="$(go env GOVERSION | sed 's/^go//')"
go_major="${go_version%%.*}"
go_minor="${go_version#*.}"
go_minor="${go_minor%%.*}"
if ((go_major < 1 || (go_major == 1 && go_minor < 26))); then
  echo "Go 1.26+ is required (found go ${go_version})." >&2
  exit 1
fi
if ((INSTALL_UI)); then
  for command in node npm; do
    command -v "$command" >/dev/null || {
      echo "$command is required for the dashboard; install Node.js 20+ or use --skip-ui." >&2
      exit 1
    }
  done
  node_major="$(node --version | sed -E 's/^v([0-9]+).*/\1/')"
  if ((node_major < 20)); then
    echo "Node.js 20+ is required for the dashboard (found $(node --version))." >&2
    exit 1
  fi
fi

data_dir="${TRELLIS_DATA_DIR:-/var/lib/trellis/data}"
ui_dir="${TRELLIS_UI_DIR:-/opt/trellis/ui}"
api_url="${TRELLIS_API_URL:-http://127.0.0.1:8128}"
advertise="${TRELLIS_ADVERTISE:-$(hostname -f 2>/dev/null || hostname)}"

echo "==> Installing Consul and containerd"
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl gpg

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://apt.releases.hashicorp.com/gpg |
  gpg --dearmor --yes -o /etc/apt/keyrings/hashicorp-archive-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com ${VERSION_CODENAME} main" \
  > /etc/apt/sources.list.d/hashicorp.list

curl -fsSL "https://download.docker.com/linux/${ID}/gpg" -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
cat > /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/${ID}
Suites: ${VERSION_CODENAME}
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF

apt-get update
apt-get install -y consul containerd.io
if ((INSTALL_WIREGUARD)); then
  apt-get install -y wireguard-tools iproute2 iptables
fi

echo "==> Configuring Consul"
install -m 0750 -d /etc/consul.d /opt/consul
cat > /etc/consul.d/server.hcl <<'EOF'
server           = true
datacenter       = "dc1"
bind_addr        = "0.0.0.0"
client_addr      = "127.0.0.1"
data_dir         = "/opt/consul"
bootstrap_expect = 1

ui_config {
  enabled = true
}
EOF
chmod 0640 /etc/consul.d/server.hcl
chown root:consul /etc/consul.d/server.hcl
chown consul:consul /opt/consul

echo "==> Building Trellis"
(
  cd "$SCRIPT_DIR/orchestrator"
  go build -o /usr/local/bin/trellis-node ./cmd/trellis-node
  go build -o /usr/local/bin/trellis ./cmd/trellis
)

install -m 0700 -d /etc/trellis
install -m 0750 -d "$data_dir"
token="${TRELLIS_TOKEN:-$(head -c 32 /dev/urandom | base64 | tr -d '\n')}"
if [[ ! "$token" =~ ^[A-Za-z0-9._~+/=-]+$ ]]; then
  echo "TRELLIS_TOKEN contains characters that cannot be stored safely in a systemd environment file." >&2
  exit 1
fi
printf 'TOKEN=%s\n' "$token" > /etc/trellis/cluster-token.env
chmod 0600 /etc/trellis/cluster-token.env

cat > /etc/systemd/system/trellis-node.service <<EOF
[Unit]
Description=Trellis node
After=containerd.service consul.service network-online.target
Wants=containerd.service consul.service network-online.target

[Service]
EnvironmentFile=/etc/trellis/cluster-token.env
ExecStart=/usr/local/bin/trellis-node --data-dir ${data_dir} --agent-advertise ${advertise}:8127 --server-advertise ${advertise}:8128 --cluster-token \${TOKEN}
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl enable --now containerd consul
systemctl daemon-reload
systemctl enable --now trellis-node

if ((INSTALL_UI)); then
  echo "==> Building and configuring the dashboard"
  install -d "$ui_dir"
  cp -a "$SCRIPT_DIR/ui/." "$ui_dir/"
  rm -rf "$ui_dir/node_modules" "$ui_dir/.next"
  cat > "$ui_dir/.env.local" <<EOF
TRELLIS_API_URL=${api_url}
TRELLIS_API_TOKEN=${token}
EOF
  chmod 0600 "$ui_dir/.env.local"
  (cd "$ui_dir" && npm ci && npm run build)

  cat > /etc/systemd/system/trellis-ui.service <<EOF
[Unit]
Description=Trellis dashboard
After=trellis-node.service network-online.target
Wants=trellis-node.service

[Service]
WorkingDirectory=${ui_dir}
ExecStart=$(command -v npm) run start
Restart=on-failure
EnvironmentFile=${ui_dir}/.env.local

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now trellis-ui
fi

echo
echo "Trellis installation complete."
echo "Cluster token: ${token}"
echo "The token is also stored in /etc/trellis/cluster-token.env (mode 0600)."
echo "Check the node with: systemctl status trellis-node"
if ((INSTALL_UI)); then
  echo "Dashboard: http://${advertise}:3000"
fi
if ((INSTALL_WIREGUARD)); then
  echo "WireGuard tools are installed; install the runsc containerd shim before using isolated jobs."
fi
