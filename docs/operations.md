# Operations guide

This guide covers a persistent single-node deployment and the controls that
also apply to multi-node clusters. Commands assume a Debian- or Ubuntu-based
Linux host with root access.

## Prerequisites

- containerd, managed as a system service
- at least 2 CPU cores and 4 GB RAM for an evaluation deployment

For installation details, follow the upstream containerd documentation for
your distribution. Verify the service before starting Trellis:

```sh
sudo systemctl status containerd
```

## Install

The setup script handles binary download, systemd unit creation, token
generation, and optional dashboard installation in one step:

```sh
curl -fsSL https://raw.githubusercontent.com/clofour/trellis-experimental/main/scripts/setup.sh | sudo bash
```

### Build and install from source

Requires Go 1.26.4 or later:

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
After=containerd.service network-online.target
Wants=containerd.service network-online.target

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
| `--raft-listen` | `:8129` | Raft consensus transport listen address. |
| `--raft-advertise` | `<hostname>:8129` | Raft transport advertised address. |
| `--join` | none | Address of an existing cluster member to join. |
| `--data-dir` | `/var/lib/trellis/data` | Local state and volume directory. |
| `--cluster` | `default` | Cluster name. |
| `--cluster-token` | none | Required shared API and cluster token. |
| `--secrets-key` | none | Root-readable file containing the 32-byte secrets KEK (raw or base64); enables native secrets. |
| `--secrets-key-id` | derived | Non-sensitive identifier recorded with encrypted secret metadata. |
| `--containerd-sock` | `/run/containerd/containerd.sock` | containerd socket. |
| `--dns-listen` | `:8053` | DNS resolver listen address for service discovery. |
| `--host-volume` | none | Advertise a named host volume as `name=/path`. Repeatable; for example `--host-volume uploads=/srv/uploads`. |
| `--wireguard-pool` | `10.64.0.0/10` | Cluster namespace address pool. |
| `--wireguard-endpoint` | automatic | Reachable WireGuard host or host:port. |
| `--wireguard-port` | `51820` | WireGuard UDP port. |

Run `trellis-node --help` against the installed version before deployment; its
output is authoritative if it differs from this page.

## Native secrets

Generate one encryption key, distribute the same root-only file to every node,
and add `--secrets-key /etc/trellis/secrets.key` to the service command:

```sh
umask 077
head -c 32 /dev/urandom | base64 > /etc/trellis/secrets.key
trellis --namespace acme secrets set database-password --stdin < password.txt
trellis --namespace acme secrets list
```

The key is independent of the cluster token and data directory. Back it up
separately: encrypted secrets cannot be recovered without it. Secret values
are never returned by list or describe operations. Updating a secret affects
new or restarted allocations only.

## Routine operations

### Back up and restore desired state

Create an on-demand, live backup through the leader API:

```sh
trellis backup create /secure/backups/trellis-$(date +%F).json
```

The snapshot contains every persisted job definition and encrypted secret
record. It deliberately excludes allocations, node registrations, leadership
epochs, tokens, and all other runtime state. Trellis does not schedule backups,
rotate them, or select a storage destination; use the command from your normal
backup workflow. Pass `-` instead of a file to read or write standard input.

Restore the snapshot after bootstrapping a fresh cluster:

```sh
trellis backup restore /secure/backups/trellis-2026-08-31.json
```

Restore is rejected when the target already has jobs or secrets. The
reconciler creates new allocations from the restored definitions; allocation
and node identities from the old cluster are never reused. Secret records stay
encrypted in the backup and can only be decrypted when the restored nodes are
configured with the original `--secrets-key` KEK file. Back up that file
separately, with access controls appropriate for encryption key material.

### Nodes

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

After a replacement control-plane node has joined, permanently remove the old
Raft voter using its advertised server ID (the value configured with
`--server-advertise`):

```sh
trellis --server-addr leader.example:8128 \
  --cluster-token "$TRELLIS_TOKEN" nodes remove old-node.example:8128
```

This changes Raft membership immediately. Only remove a permanently retired
member, and ensure the remaining voters still form a quorum.

### Rolling upgrades

Trellis supports rolling upgrades from the immediately preceding release. Raft
log entries and FSM records remain readable by both versions during that
window, so **do not create an upgrade backup or replace the data directory**.
Rollback consists of stopping the service, putting the previous binary back,
and restarting it; the on-disk state survives the binary swap.

Before starting, use `trellis nodes list` and verify that every node reports the
expected current version. Then upgrade one non-leader at a time:

1. Drain it with `trellis nodes drain NODE_ID` and wait for its allocations to
   migrate.
2. Stop it with `sudo systemctl stop trellis-node`.
3. Replace `/usr/local/bin/trellis-node` with the new binary. Do not move,
   rewrite, or restore `/var/lib/trellis/data`.
4. Start it with `sudo systemctl start trellis-node`, wait for it to become
   healthy, and confirm its new version in `trellis nodes list`.
5. Allow scheduling again with `trellis nodes undrain NODE_ID`.

After every non-leader is upgraded, ask the current leader to hand leadership
to one of those upgraded voters:

```sh
trellis --server-addr leader.example:8128 --cluster-token "$TRELLIS_TOKEN" \
  nodes transfer-leadership
```

Confirm the API is available through the new leader, then drain, stop, replace,
start, and un-drain the former leader using the same steps. A mixed version in
`nodes list` is expected only while this procedure is in progress.

Trellis never performs a breaking Raft-log or FSM migration automatically. If
a future release needs one, its release notes will name a release-specific,
explicit migration flag and the operation will refuse to run without that
flag. Do not skip releases during a rolling upgrade, and never enable a
migration flag while an older binary might rejoin the cluster.

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
