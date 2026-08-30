# Getting started

This guide installs Trellis on a single Linux machine and walks through two
deployments: a simple web server to see the basics in action, then a
WordPress blog backed by MySQL to see DNS-based service discovery and
persistent storage working together.

## Install

The setup script downloads the latest release, installs the binaries,
configures a systemd service, and generates a cluster token. It runs on
Linux x64 (Debian or Ubuntu) and requires root access.

```sh
curl -fsSL https://raw.githubusercontent.com/clofour/trellis-experimental/main/scripts/setup.sh | sudo bash
```

The script will prompt you through the options. For this guide, accept the
defaults — single node, no WireGuard, and install the web dashboard when
asked.

To build from source instead, see the [operations guide](operations.md#build-and-install).

## Check the dashboard

Once `trellis-node` is running, open the web dashboard at
<http://localhost:3000>. You should see:

- **Nodes** — your single node listed as ready
- **Jobs** — empty for now
- **Allocations** — empty for now

The dashboard refreshes every five seconds. Keep it open — you will watch
jobs and allocations appear as you work through this guide.

## Deploy a web server

Save the following as `hello.yaml`. This deploys two replicas of
[traefik/whoami](https://github.com/traefik/whoami) — a lightweight server
that responds with request details — with dynamically allocated host ports
and an HTTP health check:

```yaml
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
```

`host_port: 0` tells Trellis to pick a free port on the host for each
replica. CPU is in millicores and memory is in bytes.

Submit it and watch the status:

```sh
export TRELLIS_TOKEN="$(sudo grep -oP '(?<=TRELLIS_TOKEN=).+' /etc/trellis/trellis.env)"

trellis --server-addr localhost:8128 --cluster-token "$TRELLIS_TOKEN" \
  jobs apply --file hello.yaml

trellis --server-addr localhost:8128 --cluster-token "$TRELLIS_TOKEN" \
  --namespace examples jobs status hello
```

Two allocations will appear and transition from `pending` → `running` →
`healthy` as the container pulls and the health check passes. The same
transition is visible in the dashboard under **Allocations**.

The `jobs status` output includes the allocated host ports. Open
`http://localhost:<port>` in a browser — you will see request headers,
the server hostname, and other details from the whoami container.

To stream logs from one of the allocations:

```sh
trellis --server-addr localhost:8128 --cluster-token "$TRELLIS_TOKEN" \
  --namespace examples jobs logs --tail 50 --follow ALLOCATION_ID
```

## Deploy a multi-service stack

This example deploys a WordPress blog backed by a MySQL database. It shows
two Trellis features working together:

- **DNS discovery**: WordPress finds MySQL at `mysql.blog.trellis` — no
  manual IP configuration or service objects required.
- **Persistent volumes**: MySQL data survives allocation replacement on
  the same node.

### 1. Deploy MySQL

Save the following as `mysql.yaml`:

```yaml
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
```

Submit it and wait for the allocation to become healthy before moving on:

```sh
trellis --server-addr localhost:8128 --cluster-token "$TRELLIS_TOKEN" \
  jobs apply --file mysql.yaml

trellis --server-addr localhost:8128 --cluster-token "$TRELLIS_TOKEN" \
  --namespace blog jobs status mysql
```

### 2. Deploy WordPress

Save the following as `wordpress.yaml`. Note how `WORDPRESS_DB_HOST` uses
the Trellis DNS name `mysql.blog.trellis` — Trellis resolves this to the
address of the healthy MySQL allocation automatically:

```yaml
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
```

Submit it:

```sh
trellis --server-addr localhost:8128 --cluster-token "$TRELLIS_TOKEN" \
  jobs apply --file wordpress.yaml
```

Once the WordPress allocation is healthy, find its host port with `jobs
status` and open `http://localhost:<port>` in your browser. WordPress will
show the familiar installation screen — already connected to the database
through Trellis DNS. Complete the setup to have a fully running blog.

### 3. Add a reverse proxy

The allocations above use dynamic host ports. For a real deployment you
would want a reverse proxy that listens on port 80 and routes incoming
requests to the healthy allocations automatically.

The [reverse-proxy example](../examples/reverse-proxy/) shows exactly this
pattern using nginx and the allocations API. It uses the `trellis.expose`
and `trellis/domain` labels already set on the WordPress task group above —
deploy the proxy job from that example and it will pick up WordPress
without any additional configuration.

## What to explore next

- **Scale and update**: edit `count` in a manifest and reapply. Trellis
  reconciles running allocations to match — new replicas are placed where
  capacity is available and surplus replicas are stopped. Set an
  `update.strategy: rolling` to replace replicas incrementally rather
  than all at once.
- **Allocation queries**: `GET /v1/allocations?label=trellis.expose:true`
  returns runtime info (node address, ports, health) for any labeled group.
  The reverse proxy example uses this to configure nginx dynamically.
- **Multi-node cluster**: run the setup script on additional machines and
  join them to the first. Jobs spread across nodes; draining a node
  migrates its allocations.
- **Vagrant demo**: try a pre-configured three-node cluster locally with
  the [Vagrant demo](development/vagrant.md).

## Troubleshooting

- **`--cluster-token is required`:** read the token with `sudo grep -oP '(?<=TRELLIS_TOKEN=).+' /etc/trellis/trellis.env`.
- **Cannot connect to `localhost:8128`:** verify the node has acquired leadership — look for `leadership acquired` in `sudo journalctl -u trellis-node`.
- **A job stays pending:** run `nodes list` to confirm the node is ready and has enough capacity for the requested CPU and memory.
- **DNS not resolving:** ensure `trellis-node` is running with the default `--dns-listen :8053`; containers receive the resolver address automatically.
- **WordPress can't reach MySQL:** confirm the MySQL allocation is healthy before the WordPress allocation starts; WordPress reads `WORDPRESS_DB_HOST` only at startup.
