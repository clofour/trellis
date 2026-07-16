#!/usr/bin/env bash
set -euo pipefail

SHARE_DIR="/vagrant/bin"
DATA_DIR="/var/lib/trellis/data"

for _ in $(seq 1 30); do
    if [ -s "${SHARE_DIR}/token" ]; then
        break
    fi
    sleep 1
done

cat > /etc/systemd/system/trellis-agent.service <<EOF
[Unit]
Description=Trellis agent
After=containerd.service consul.service network-online.target
Wants=containerd.service consul.service network-online.target

[Service]
ExecStart=/usr/local/bin/trellis-agent --listen :9100 --data-dir ${DATA_DIR} --server-addr http://control.trellis.local:9100 --cluster-token $(cat ${SHARE_DIR}/token)
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable trellis-agent
systemctl start trellis-agent
