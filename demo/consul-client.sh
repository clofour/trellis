#!/usr/bin/env bash

cat > /etc/consul.d/server.hcl <<EOF
    server = false

    datacenter = "dc1"

    bind_addr = ""
    retry_join = ""

    data_dir = "/opt/consul"
EOF

systemctl enable consul
systemctl start consul
