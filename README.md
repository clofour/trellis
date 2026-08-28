# Trellis

Trellis is a small container scheduler built around containerd and Consul. The
MVP provides job manifest validation, node registration and heartbeats,
balanced task placement, allocation lifecycle management, health checks,
restart handling, port allocation, persistent volumes, and Consul service
registration.

## Quick start

The demo scripts provision Consul and containerd, then start the server and an
agent:

```sh
vagrant up
```

When starting the components manually, run Consul and containerd first, then:

```sh
go run ./cmd/trellis-server --data-dir ./data
go run ./cmd/trellis-agent --cluster-token "$TRELLIS_TOKEN"
```

On first startup the server logs the generated cluster token and persists it in
the server data directory. Pass that token to agents and CLI requests.

Submit a job using a YAML manifest:

```yaml
name: hello
task_groups:
  - name: web
    count: 2
    tasks:
      - name: server
        image: docker.io/library/nginx:alpine
        ports:
          - host_port: 0
            container_port: 80
        health_check:
          type: http
          port: 80
          path: /
```

```sh
go run ./cmd/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file trellis.yaml
go run ./cmd/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" nodes list
```

The server API listens on port `8128` by default; agents listen on `8127`.
