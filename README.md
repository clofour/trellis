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

## Single-node production setup

This section walks through deploying Trellis with the dashboard UI and
multi-tenant isolation on a single server. Unlike the Vagrant demo, this
targets a real Linux machine (bare metal or VM) and produces a systemd-managed
stack you can operate in production.

### Prerequisites

- A Linux server (Debian 12+ or Ubuntu 22.04+ recommended) with root access
- 2+ CPU cores and 4 GB+ RAM (more for heavier workloads)
- Go 1.26+ (for building from source) or pre-built binaries from CI
- Node.js 20+ and npm (for the dashboard UI)

For tenant isolation (optional):
- Linux kernel with WireGuard support (5.6+ has it built in)
- `wireguard-tools`, `iproute2`, and `iptables` packages
- gVisor `runsc` containerd shim (`io.containerd.runsc.v1`)

### Step 1: Install Consul

Install Consul from the HashiCorp APT repository:

```sh
sudo apt-get update
sudo apt-get install -y ca-certificates curl gpg

wget -O - https://apt.releases.hashicorp.com/gpg | \
  sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg

echo "deb [arch=$(dpkg --print-architecture) \
  signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] \
  https://apt.releases.hashicorp.com $(lsb_release -cs) main" | \
  sudo tee /etc/apt/sources.list.d/hashicorp.list

sudo apt-get update
sudo apt-get install -y consul
```

Configure Consul as a single-node server. Write `/etc/consul.d/server.hcl`:

```hcl
server           = true
datacenter       = "dc1"
bind_addr        = "0.0.0.0"
client_addr      = "127.0.0.1"
data_dir         = "/opt/consul"
bootstrap_expect = 1

ui_config {
  enabled = true
}
```

Start and enable Consul:

```sh
sudo systemctl enable consul
sudo systemctl start consul
```

Verify it is running:

```sh
consul members
```

### Step 2: Install containerd

Install containerd from the Docker APT repository:

```sh
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/debian/gpg \
  -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

sudo tee /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/debian
Suites: $(. /etc/os-release && echo "$VERSION_CODENAME")
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF

sudo apt-get update
sudo apt-get install -y containerd.io
```

Verify containerd is running:

```sh
sudo systemctl status containerd
```

### Step 3: Build Trellis

Clone the repository and build both binaries:

```sh
cd orchestrator
go build -o /usr/local/bin/trellis-node ./cmd/trellis-node
go build -o /usr/local/bin/trellis       ./cmd/trellis
```

Alternatively, download the `trellis_linux_x64` artifact from a CI build.

### Step 4: Generate a cluster token

Create a random token that authenticates all API requests:

```sh
TOKEN=$(head -c 32 /dev/urandom | base64)
echo "$TOKEN" | sudo tee /etc/trellis/cluster-token > /dev/null
sudo chmod 600 /etc/trellis/cluster-token
```

Save this token — the UI and CLI both need it.

### Step 5: Create the Trellis systemd service

Create the data directory:

```sh
sudo mkdir -p /var/lib/trellis/data
```

Write `/etc/systemd/system/trellis-node.service`:

```ini
[Unit]
Description=Trellis node
After=containerd.service consul.service network-online.target
Wants=containerd.service consul.service network-online.target

[Service]
ExecStart=/usr/local/bin/trellis-node \
  --data-dir /var/lib/trellis/data \
  --agent-advertise %H:8127 \
  --server-advertise %H:8128 \
  --cluster-token ${TOKEN}
Restart=on-failure
EnvironmentFile=/etc/trellis/cluster-token.env

[Install]
WantedBy=multi-user.target
```

To keep the token out of the unit file, write
`/etc/trellis/cluster-token.env` instead:

```sh
# /etc/trellis/cluster-token.env
TOKEN=<paste the base64 token from step 4>
```

Then reference it with `${TOKEN}` in the `ExecStart` line (as shown above) and
protect the file:

```sh
sudo chmod 600 /etc/trellis/cluster-token.env
```

Start the node:

```sh
sudo systemctl daemon-reload
sudo systemctl enable trellis-node
sudo systemctl start trellis-node
```

Verify it registered with Consul and elected itself leader:

```sh
sudo journalctl -u trellis-node -f
# Look for: "leadership acquired"
```

### Step 6: Deploy the dashboard UI

The UI is a Next.js application that proxies API calls to the Trellis leader,
injecting the cluster token server-side so the browser never sees it.

```sh
cd ui
npm install
```

Create the environment file at `ui/.env.local`:

```sh
TRELLIS_API_URL=http://localhost:8128
TRELLIS_API_TOKEN=<paste the base64 token from step 4>
```

Build and start the production server:

```sh
npm run build
npm run start
```

The dashboard is now available at `http://<your-server>:3000`. It shows
cluster health, nodes, jobs, and allocation details, refreshing every 5
seconds.

For a persistent deployment, create a systemd service for the UI:

```ini
# /etc/systemd/system/trellis-ui.service
[Unit]
Description=Trellis dashboard
After=trellis-node.service network-online.target
Wants=trellis-node.service

[Service]
WorkingDirectory=/opt/trellis/ui
ExecStart=/usr/bin/npm run start
Restart=on-failure
EnvironmentFile=/opt/trellis/ui/.env.local

[Install]
WantedBy=multi-user.target
```

### Step 7: Enable multi-tenant isolation (optional)

Tenant isolation uses gVisor for container sandboxing and WireGuard for network
segmentation. Install the required tools:

```sh
sudo apt-get install -y wireguard-tools iptables
```

Install the gVisor `runsc` containerd shim following the
[gVisor containerd quick start](https://gvisor.dev/docs/user_guide/containerd/quick_start/).
Verify the shim is available:

```sh
sudo ls /usr/local/bin/containerd-shim-runsc-v1
```

No additional `trellis-node` flags are needed — WireGuard identity is generated
automatically on first start and persisted under the data directory. On a
single-node setup the default `--wireguard-pool 10.64.0.0/10` works without
changes.

Submit a tenant-scoped job:

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
        ports:
          - host_port: 0
            container_port: 80
        health_check:
          type: http
          port: 80
          path: /
```

```sh
trellis --server-addr localhost:8128 \
  --cluster-token "$TOKEN" \
  --tenant acme \
  jobs apply --file storefront.yaml
```

Isolated jobs require explicit `resources` on every task. Trellis enforces the
tenant's aggregate quota, runs each container under the `runsc` shim, stores
volumes under a tenant-specific path, and places each allocation in its own
WireGuard-connected network namespace.

### Step 8: Verify the deployment

Check that everything is working:

```sh
# List registered nodes (should show one node as "ready")
trellis --server-addr localhost:8128 --cluster-token "$TOKEN" nodes list

# Submit the hello-world job from the quick start section
trellis --server-addr localhost:8128 --cluster-token "$TOKEN" \
  jobs apply --file trellis.yaml

# List jobs and allocations
trellis --server-addr localhost:8128 --cluster-token "$TOKEN" jobs list

# Stream logs from an allocation
trellis --server-addr localhost:8128 --cluster-token "$TOKEN" \
  jobs logs --tail 50 --follow <ALLOCATION_ID>
```

Open `http://<your-server>:3000` in a browser to see the dashboard with your
node, jobs, and allocation health.

### CLI flag reference

| Flag | Default | Description |
|------|---------|-------------|
| `--agent-listen` | `:8127` | Agent API listen address |
| `--agent-advertise` | `<hostname>:8127` | Agent address advertised to the cluster |
| `--server-listen` | `:8128` | Leader API listen address |
| `--server-advertise` | `<hostname>:8128` | Leader API address advertised to the cluster |
| `--data-dir` | `/var/lib/trellis/data` | Directory for local state and volumes |
| `--cluster` | `default` | Cluster name |
| `--cluster-token` | (required) | Shared secret for API authentication |
| `--containerd-sock` | `/run/containerd/containerd.sock` | containerd socket path |
| `--consul-addr` | `127.0.0.1:8500` | Consul HTTP address |
| `--election-ttl` | `15s` | Leader election session TTL |
| `--wireguard-pool` | `10.64.0.0/10` | Address pool for tenant networking |
| `--wireguard-endpoint` | (auto) | Externally reachable WireGuard host:port |
| `--wireguard-port` | `51820` | WireGuard UDP listen port |

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
go run ./cmd/trellis jobs logs --tail 100 --follow ALLOCATION_ID
```

Drain a node to reject new placements and migrate its existing allocations to
healthy nodes with enough spare capacity:

```sh
go run ./cmd/trellis nodes drain NODE_ID
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
trellis-node --wireguard-pool 10.64.0.0/10 \
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
