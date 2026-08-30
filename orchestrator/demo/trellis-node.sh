#!/usr/bin/env bash
set -euo pipefail

SHARE_DIR="/vagrant/bin"
DATA_DIR="/var/lib/trellis/data"
TOKEN_FILE="${SHARE_DIR}/token"

# Generate a shared cluster token on the first node to write it.
mkdir -p "${SHARE_DIR}"
if [ ! -s "${TOKEN_FILE}" ]; then
    umask 077
    head -c 32 /dev/urandom | base64 > "${TOKEN_FILE}"
fi

# Install binaries from the shared folder.
# Build them first on the host: cd orchestrator && go build -o bin/trellis-node ./cmd/trellis-node && go build -o bin/trellis ./cmd/trellis
install -m 0755 "${SHARE_DIR}/trellis-node" /usr/local/bin/trellis-node
install -m 0755 "${SHARE_DIR}/trellis"      /usr/local/bin/trellis

mkdir -p "${DATA_DIR}"

HOSTNAME=$(hostname)
JOIN_FLAG=""
if [ "${HOSTNAME}" != "control.trellis.local" ]; then
    JOIN_FLAG="--join control.trellis.local:8128"
fi

cat > /etc/systemd/system/trellis-node.service <<EOF
[Unit]
Description=Trellis node
After=containerd.service network-online.target
Wants=containerd.service network-online.target

[Service]
ExecStart=/usr/local/bin/trellis-node \
  --data-dir ${DATA_DIR} \
  --agent-advertise ${HOSTNAME}:8127 \
  --server-advertise ${HOSTNAME}:8128 \
  --raft-advertise ${HOSTNAME}:8129 \
  --cluster-token $(cat "${TOKEN_FILE}") ${JOIN_FLAG}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable trellis-node
systemctl start trellis-node
