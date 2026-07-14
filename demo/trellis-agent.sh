#!/usr/bin/env bash

SHARE_DIR="/share"
DATA_DIR="/var/lib/trellis/data"

for _ in $(seq 1 30); do
    if [ -s "${SHARE_DIR}/token" ]; then
        break
    end
done

cat > /etc/systemd/system/trellis-agent.service <<EOF
[Unit]
Description=Trellis agent
After=containerd.service consul.service network-online.target
Wants=containerd.service consul.service network-online.target

[Service]
ExecStart=/usr/local/bin/trellis-server --cluster-name cluster --listen :9100 --data-dir ${DATA_DIR}
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable trellis-agent
systemctl start trellis-agent
