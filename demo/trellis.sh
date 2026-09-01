#!/usr/bin/env bash
set -euo pipefail

SHARE_DIR="/vagrant/bin"
DATA_DIR="/var/lib/trellis/data"
TOKEN_FILE="${SHARE_DIR}/token"

mkdir -p "${SHARE_DIR}"
if [ ! -s "${TOKEN_FILE}" ]; then
    umask 077
    head -c 32 /dev/urandom | base64 > "${TOKEN_FILE}"
fi

cat > /etc/systemd/system/trellis.service <<EOF
[Unit]
Description=Trellis node
After=containerd.service consul.service network-online.target
Wants=containerd.service consul.service network-online.target

[Service]
ExecStart=/usr/local/bin/trellis --data-dir ${DATA_DIR} --agent-advertise $(hostname):8127 --server-advertise $(hostname):8128 --cluster-token $(cat "${TOKEN_FILE}")
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable trellis
systemctl start trellis
