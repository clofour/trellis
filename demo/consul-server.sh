#!/usr/bin/env bash
set -euo pipefail

cat > /etc/consul.d/server.hcl <<EOF
    server = true

    datacenter = "dc1"

    bind_addr = "0.0.0.0"
    client_addr = "127.0.0.1"

    data_dir = "/opt/consul"

    bootstrap_expect = 1

    ui_config = {
        enabled = true
    }
EOF

systemctl enable consul
systemctl start consul
