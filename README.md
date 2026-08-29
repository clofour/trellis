# Trellis

Trellis is a small container scheduler built around containerd and Consul. The
MVP provides job manifest validation, node registration and heartbeats,
balanced task placement, allocation lifecycle management, health checks,
restart handling, port allocation, persistent volumes, and Consul service
registration. Every machine runs the same `trellis-node` daemon. Consul elects
one node as leader; only that node exposes the control-plane API and reconciles
jobs. The remaining nodes continue running allocations and automatically take
part in the next election.

## Quick start

The demo scripts provision Consul and containerd, then start a Trellis node on
each machine:

```sh
vagrant up
```

When starting manually, run Consul and containerd first, provide the same
cluster token to every node, and advertise addresses reachable by the other
nodes:

```sh
go run ./cmd/trellis-node --data-dir ./data \
  --agent-advertise node-1.example:8127 \
  --server-advertise node-1.example:8128 \
  --cluster-token "$TRELLIS_TOKEN"
```

The first node initializes cluster metadata from the token. Later nodes verify
the same token before joining. Leader ownership is a renewable Consul session;
loss of that session cancels the leader's API and reconciliation loop, after
which another node acquires the lock. Agents watch the lock and re-register
with the elected leader.

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

The elected leader API listens on port `8128` by default; every node's agent API
listens on `8127`.
