# Trellis

Trellis is a small container scheduler built around containerd and Consul. The
MVP provides job manifest validation, node registration and heartbeats,
balanced task placement, allocation lifecycle management, health checks,
restart handling, port allocation, persistent volumes, and Consul service
registration. Every machine runs the same `trellis` daemon. Consul elects
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
go run ./cmd/trellis --data-dir ./data \
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
go run ./cmd/trellisctl --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file trellis.yaml
go run ./cmd/trellisctl --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" nodes list
```

The elected leader API listens on port `8128` by default; every node's agent API
listens on `8127`.

## Resources, logs, and maintenance

CPU requests are expressed in millicores and memory requests in bytes. Nodes
advertise their detected capacity, and the scheduler only places allocations
where both requests fit. The same limits are applied to the container runtime:

```yaml
resources:
  cpu: 500
  memory: 268435456
```

Stream the most recent output from an allocation (and keep following it with
`--follow`):

```sh
go run ./cmd/trellisctl jobs logs --tail 100 --follow ALLOCATION_ID
```

Drain a node to reject new placements and migrate its existing allocations to
healthy nodes with enough spare capacity:

```sh
go run ./cmd/trellisctl nodes drain NODE_ID
```

## Optional tenant isolation

Trusted jobs continue to work without any tenant configuration. A frontend for
untrusted users can instead scope every request with `X-Trellis-Tenant` (the CLI
equivalent is `--tenant`) and submit an isolated job:

```yaml
name: storefront
tenant: acme
isolation:
  runtime: runsc
  network:
    enabled: true
  quota:
    cpu: 2000
    memory: 2147483648
task_groups:
  - name: web
    count: 2
    tasks:
      - name: server
        image: docker.io/library/nginx:alpine
        resources:
          cpu: 500
          memory: 268435456
```

Isolated jobs require explicit per-task limits. Trellis enforces the tenant's
aggregate CPU and memory quota before accepting a job, selects the configured
gVisor `runsc` containerd shim, stores volumes below a tenant-specific path, and
creates a network namespace connected to an administrator-defined WireGuard
overlay. The API never resolves jobs, allocations, logs, or volumes outside the
request's tenant scope.

Each node needs `ip`, `wg`, `iptables`, kernel WireGuard support, and an
`io.containerd.runsc.v1` shim. Trellis generates and persists the node's
WireGuard identity, publishes its public key and endpoint during registration,
and derives non-overlapping node subnets from the cluster pool. Configure only
the cluster-wide pool and, when automatic discovery is unsuitable, the node's
reachable endpoint:

```sh
trellis --wireguard-pool 10.64.0.0/10 \
  --wireguard-endpoint node-a.example:51820 \
  --wireguard-port 51820
```

All nodes in a cluster must use the same pool. The pool must not overlap host or
datacenter routes, and UDP traffic on the configured port must be permitted
between nodes. Private keys and IP leases live below the node data directory.
The leader builds peer and `AllowedIPs` plans from registered node identities;
the agent creates tenant bridges and WireGuard interfaces, installs routes and
forwarding guards, and places each allocation in its own network namespace.
Starting an isolated allocation fails closed if any setup step does not
succeed. A detected deterministic subnet collision also fails closed rather
than replacing another tenant route.
