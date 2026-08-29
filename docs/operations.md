# Operations guide

This guide covers a persistent single-node deployment and the controls that
also apply to multi-node clusters. Commands assume a Debian- or Ubuntu-based
Linux host with root access.

## Prerequisites

- Consul and containerd, managed as system services
- Go 1.26.4 or later when building from source
- Node.js 20 or later and npm for the optional dashboard
- at least 2 CPU cores and 4 GB RAM for an evaluation deployment

For installation details, follow the upstream Consul and containerd
documentation for your distribution. Verify both services before starting
Trellis:

```sh
consul members
sudo systemctl status containerd
```

## Build and install

```sh
cd orchestrator
go build -o /usr/local/bin/trellis-node ./cmd/trellis-node
go build -o /usr/local/bin/trellis ./cmd/trellis
```

CI also publishes the `trellis_linux_x64` artifact on builds of the main
branch.

## Protect the cluster token

Generate a token and store it in a root-readable environment file:

```sh
sudo install -d -m 0750 /etc/trellis
TOKEN="$(head -c 32 /dev/urandom | base64)"
printf 'TRELLIS_TOKEN=%s\n' "$TOKEN" | sudo tee /etc/trellis/trellis.env >/dev/null
sudo chmod 600 /etc/trellis/trellis.env
unset TOKEN
```

Treat this token as a cluster-wide administrative secret. The node APIs accept
it as a bearer token; rotate and distribute it using your normal secrets
management process.

## Run the node with systemd

Create storage and `/etc/systemd/system/trellis-node.service`:

```sh
sudo install -d -m 0750 /var/lib/trellis/data
```

```ini
[Unit]
Description=Trellis node
After=containerd.service consul.service network-online.target
Wants=containerd.service consul.service network-online.target

[Service]
EnvironmentFile=/etc/trellis/trellis.env
ExecStart=/usr/local/bin/trellis-node \
  --data-dir /var/lib/trellis/data \
  --agent-advertise node-1.example:8127 \
  --server-advertise node-1.example:8128 \
  --cluster-token ${TRELLIS_TOKEN}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Replace the advertised hostnames with addresses reachable by every cluster
member. Then enable and inspect the service:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now trellis-node
sudo journalctl -u trellis-node -f
```

## Node flag reference

| Flag | Default | Description |
| --- | --- | --- |
| `--agent-listen` | `:8127` | Agent API listen address. |
| `--agent-advertise` | `<hostname>:8127` | Agent address advertised to the cluster. |
| `--server-listen` | `:8128` | Leader API listen address. |
| `--server-advertise` | `<hostname>:8128` | Leader address advertised to the cluster. |
| `--data-dir` | `/var/lib/trellis/data` | Local state and volume directory. |
| `--cluster` | `default` | Cluster name and Consul state scope. |
| `--cluster-token` | none | Required shared API and cluster token. |
| `--containerd-sock` | `/run/containerd/containerd.sock` | containerd socket. |
| `--consul-addr` | `127.0.0.1:8500` | Consul HTTP address. |
| `--election-ttl` | `15s` | Leadership session TTL. |
| `--wireguard-pool` | `10.64.0.0/10` | Cluster namespace address pool. |
| `--wireguard-endpoint` | automatic | Reachable WireGuard host or host:port. |
| `--wireguard-port` | `51820` | WireGuard UDP port. |

Run `trellis-node --help` against the installed version before deployment; its
output is authoritative if it differs from this page.

## Routine operations

List node health and advertised capacity:

```sh
trellis --server-addr leader.example:8128 \
  --cluster-token "$TRELLIS_TOKEN" nodes list
```

Drain a node to stop new placement and migrate its allocations where capacity
permits:

```sh
trellis --server-addr leader.example:8128 \
  --cluster-token "$TRELLIS_TOKEN" nodes drain NODE_ID
```

Inspect a namespaced job and follow allocation output:

```sh
trellis --server-addr leader.example:8128 --cluster-token "$TRELLIS_TOKEN" \
  --namespace examples jobs status hello

trellis --server-addr leader.example:8128 --cluster-token "$TRELLIS_TOKEN" \
  --namespace examples jobs logs --tail 100 --follow ALLOCATION_ID
```

## Dashboard

The dashboard proxies API calls on the server so that the cluster token is not
sent to the browser. See the [dashboard guide](../ui/README.md) for local and
systemd deployment instructions.

## WireGuard networking (optional)

Each participating node needs:

- kernel WireGuard support and `wireguard-tools`
- `iproute2` and `iptables`
- the `io.containerd.runsc.v1` gVisor containerd shim
- bidirectional UDP reachability on the configured WireGuard port

Install the distribution packages and verify the gVisor shim:

```sh
sudo apt-get install -y wireguard-tools iproute2 iptables
command -v containerd-shim-runsc-v1
```

All nodes must use the same `--wireguard-pool`, and that CIDR must not overlap
host or datacenter routes. Trellis automatically persists node identity below
the data directory. Set an endpoint only when the advertised agent host is not
the correct externally reachable address:

```sh
trellis-node --wireguard-pool 10.64.0.0/10 \
  --wireguard-endpoint node-a.example:51820 \
  --wireguard-port 51820 \
  --cluster-token "$TRELLIS_TOKEN"
```

Enable the mechanism per job with:

```yaml
network:
  wireguard: true
```

WireGuard state, private keys, and IP leases live below the node data
directory. Back up and protect that directory accordingly.
