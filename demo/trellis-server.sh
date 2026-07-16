#!/usr/bin/env bash
set -euo pipefail

SHARE_DIR="/vagrant/bin"
DATA_DIR="/var/lib/trellis/data"

cat > /etc/systemd/system/trellis-server.service <<EOF
[Unit]
Description=Trellis control plane
After=consul.service network-online.target
Wants=consul.service network-online.target

[Service]
ExecStart=/usr/local/bin/trellis-server --listen :9100 --data-dir ${DATA_DIR}
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable trellis-server
systemctl start trellis-server

for _ in $(seq 1 30); do
    if [ -s "${DATA_DIR}/token" ]; then
        install -m 0644 "${DATA_DIR}/token" "${SHARE_DIR}/share"
        exit 0
    fi
    sleep 1
done