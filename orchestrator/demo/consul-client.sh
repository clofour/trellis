#!/usr/bin/env bash
set -euo pipefail

cat > /etc/consul.d/client.hcl <<-EOF
    server = false

    datacenter = "dc1"

    bind_addr = "0.0.0.0"
    client_addr = "0.0.0.0"
    retry_join = ["control.trellis.local"]

    data_dir = "/opt/consul"
EOF

systemctl enable consul
systemctl start consul
