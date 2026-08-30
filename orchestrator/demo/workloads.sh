#!/usr/bin/env bash
#
# Deploys example workloads on the Trellis demo cluster.
# Runs on the control node after all nodes have joined.
#
set -euo pipefail

SHARE_DIR="/vagrant/bin"
TOKEN_FILE="${SHARE_DIR}/token"
TRELLIS_TOKEN="$(cat "${TOKEN_FILE}")"
TRELLIS_ADDR="localhost:8128"
CLI="trellis --server-addr ${TRELLIS_ADDR} --cluster-token ${TRELLIS_TOKEN}"

wait_healthy() {
    local namespace="$1" job="$2" timeout="${3:-120}"
    local elapsed=0
    echo "Waiting for ${namespace}/${job} to become healthy..."
    while [ "${elapsed}" -lt "${timeout}" ]; do
        if ${CLI} --namespace "${namespace}" jobs status "${job}" 2>/dev/null \
                | grep -q "healthy"; then
            echo "${namespace}/${job} is healthy."
            return 0
        fi
        sleep 5
        elapsed=$((elapsed + 5))
    done
    echo "Timed out waiting for ${namespace}/${job}" >&2
    return 1
}

# ── Wait for the cluster to elect a leader ───────────────────────────

echo "Waiting for cluster leader..."
for i in $(seq 1 30); do
    if ${CLI} nodes list >/dev/null 2>&1; then
        echo "Cluster is ready."
        break
    fi
    sleep 3
done

# ── Hello: two-replica web server ────────────────────────────────────
# Uses traefik/whoami: a lightweight server that returns request details,
# demonstrating dynamic port allocation and placement across nodes.

cat <<'EOF' | ${CLI} jobs apply --file /dev/stdin
namespace: examples
name: hello
task_groups:
  - name: web
    count: 2
    tasks:
      - name: server
        image: docker.io/traefik/whoami
        resources:
          cpu: 100
          memory: 16777216
        ports:
          - host_port: 0
            container_port: 80
        health_check:
          type: http
          port: 80
          path: /health
EOF

# ── Blog: WordPress + MySQL ──────────────────────────────────────────

cat <<'EOF' | ${CLI} jobs apply --file /dev/stdin
namespace: blog
name: mysql
task_groups:
  - name: db
    count: 1
    tasks:
      - name: mysql
        image: docker.io/library/mysql:8
        env:
          MYSQL_ROOT_PASSWORD: trellis_demo
          MYSQL_DATABASE: wordpress
          MYSQL_USER: wordpress
          MYSQL_PASSWORD: wordpress
        resources:
          cpu: 500
          memory: 536870912
        ports:
          - host_port: 0
            container_port: 3306
        volumes:
          - name: data
            path: /var/lib/mysql
        health_check:
          type: tcp
          port: 3306
EOF

wait_healthy blog mysql 180

cat <<'EOF' | ${CLI} jobs apply --file /dev/stdin
namespace: blog
name: wordpress
task_groups:
  - name: app
    count: 1
    labels:
      trellis.expose: "true"
      trellis/domain: localhost
    tasks:
      - name: wordpress
        image: docker.io/library/wordpress:latest
        env:
          WORDPRESS_DB_HOST: mysql.blog.trellis
          WORDPRESS_DB_NAME: wordpress
          WORDPRESS_DB_USER: wordpress
          WORDPRESS_DB_PASSWORD: wordpress
        resources:
          cpu: 500
          memory: 536870912
        ports:
          - host_port: 0
            container_port: 80
        health_check:
          type: http
          port: 80
          path: /wp-login.php
EOF

echo ""
echo "Demo workloads deployed. Check status with:"
echo "  trellis --namespace examples jobs status hello"
echo "  trellis --namespace blog jobs status wordpress"
