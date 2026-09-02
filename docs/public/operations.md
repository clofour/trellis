# Operations

Operational commands use the vocabulary in the [Trellis user model](user-model.md): apply/delete jobs, inspect allocations, and drain/undrain nodes. Raft and leadership controls are advanced control-plane operations rather than part of the normal workload model. The [CLI workflows guide](cli.md) covers contexts, planning, rollout watching, diagnosis, logging, and structured output in detail.

## Routine workflow

```sh
trellisctl context current
trellisctl jobs validate --file trellis.yaml
trellisctl jobs diff --file trellis.yaml
trellisctl jobs apply --file trellis.yaml --wait
trellisctl jobs status NAME
trellisctl jobs diagnose NAME
trellisctl jobs logs NAME --tail 200
trellisctl jobs delete NAME
```

Commands with a coherent structured result expose a local `--output json` flag; streaming and action commands do not. See [CLI workflows](cli.md#structured-output-and-automation) for the exact contract.

## Node configuration

Installer-managed nodes keep their durable daemon configuration at `/etc/trellis/trellis.yaml`. The file is root-readable and contains the node's bootstrap credential together with operator-managed settings such as advertise addresses, labels, host-volume advertisements, secret-encryption key path, and WireGuard transport settings.

A minimal installed node resembles:

```yaml
cluster: default
bootstrap_token: trls_boot_...
data_dir: /var/lib/trellis/data
agent_advertise: node-a:8127
server_advertise: node-a:8128
raft_advertise: node-a:8129
```

Edit this file when changing persistent node configuration, then restart the service:

```sh
sudo systemctl restart trellis
```

`trellis --config PATH` loads the same strict YAML format. Explicit daemon flags override values from the file and are useful for one-off runs; the installed systemd unit intentionally contains only `trellis --config /etc/trellis/trellis.yaml` so there is one durable configuration source.

## Add a node

Run the same setup script on the new Debian/Ubuntu machine. When prompted:

1. choose an advertise address reachable by the existing nodes;
2. answer **yes** to **Join an existing cluster?**;
3. enter an existing node's `host:8128` address;
4. enter the existing bootstrap token when prompted. The token input is not echoed.

The bootstrap token is the `bootstrap_token` value in `/etc/trellis/trellis.yaml` on an existing node. Treat it as the root cluster credential and transfer it over a secure channel rather than placing it in shell history.

After the new daemon starts, verify membership from an existing CLI context:

```sh
trellisctl nodes list
```

A joining node must use the existing bootstrap token; minting an ordinary operator/workload token does not create or join a cluster.

## Mint operator credentials

The installer creates one normal `cluster/write` credential for the installing user, but operators often need narrower credentials for another human, a read-only dashboard, or automation. `trellisctl credentials create` is the explicit administrative workflow for that.

Credential minting requires the **bootstrap** credential. On an installed Trellis node, running the command as root automatically uses the root-readable local node connection, so the bootstrap value does not need to be copied into shell history:

```sh
# Read-only cluster observer
sudo trellisctl credentials create --scope cluster --access read

# Writer restricted to one namespace
sudo trellisctl credentials create \
  --scope namespace \
  --namespace-scope staging \
  --access write
```

The default output is the newly minted bearer token so it can be handed directly to a password manager or context setup. Use `--output json` when automation needs the response object instead:

```sh
sudo trellisctl credentials create --scope cluster --access read --output json
```

To save a generated credential as an ordinary user context without leaving it in command history:

```sh
TOKEN="$(sudo trellisctl credentials create --scope namespace --namespace-scope staging --access write)"
trellisctl --token "$TOKEN" --namespace staging context save staging --use
unset TOKEN
```

A remote bootstrap administrator may instead supply the bootstrap bearer credential through `TRELLIS_TOKEN` or `--token`, but it should be handled as a root secret. Ordinary `cluster/write` credentials cannot mint more credentials, join nodes, change Raft membership, or perform backup/restore.

## Drain and maintenance

`trellisctl nodes drain NODE` prevents new placement and migrates allocations. `NODE` may be the host/address displayed by `nodes list`, a unique UUID prefix, or a complete UUID. Wait until workloads have healthy replacements before maintenance. `trellisctl nodes undrain NODE` re-enables scheduling. `nodes remove NODE` permanently removes a node from the cluster and is different from draining.

## Advanced control-plane maintenance

Trellis uses Raft internally. If an operator deliberately needs to move control-plane leadership before maintenance, the advanced command `trellisctl nodes transfer-leadership` requests a transfer to another voter. It is intentionally hidden from normal CLI help because workload operations should not require understanding Raft leadership.

Preserve quorum: operate an odd number of Raft voters and avoid removing several members together.

## Backups

```sh
trellisctl backup create --file trellis-backup.json
trellisctl backup restore trellis-backup.json
```

Backups contain desired jobs and encrypted secret records, not allocations, container images, local volume data, TLS private keys, or the secret encryption key. Restores schedule fresh allocations. Secure and separately back up the 32-byte secrets key referenced by `secrets_key` in the node config; encrypted records are unusable without it.

## Secrets

```sh
printf %s 'value' | trellisctl --namespace default secrets set db-password --stdin
trellisctl --namespace default secrets describe db-password
trellisctl --namespace default secrets delete db-password
```

Use `--expected-version N` for compare-and-swap (`0` means create only). Values are capped at 65,536 bytes. Rotation affects newly started allocations, so apply a workload revision or replace the consuming allocations afterward.

## Observability

The control plane exposes Prometheus metrics at `/metrics`. `GET /v1/auth/whoami` reports the kind, scope, and access of the bearer credential making the request. Job status and allocation events explain lifecycle transitions; logs proxy per-task allocation logs. Monitor leader availability, unhealthy/draining nodes, desired-versus-running/healthy counts, reconciliation latency, retries, and disk capacity for Raft, containerd, and volumes.

For normal workload diagnosis, start with `jobs status`. `ready`, `converging`, and `degraded` summarize desired-versus-observed state without collapsing allocation lifecycle and health. `jobs diagnose` then surfaces only the allocations that need attention, including reason/message, retry timing, and attempt count. `jobs logs NAME` reads matching task streams without requiring full internal runtime IDs.

## Networking and TLS

Ports `8127`, `8128`, and `8129` must be reachable between appropriate cluster members; allow WireGuard UDP `51820` and DNS UDP `8053` when namespace networking is enabled. Never expose the unauthenticated transport surface to an untrusted network. Configure a CA and node certificates on every node and pass the CA/client certificate flags to the CLI. Advertised addresses must be routable from peers, not wildcard bind addresses.

## Failure recovery

A missed-heartbeat node becomes unhealthy; allocations may become lost after leader recovery grace and an availability timeout. Reconciliation replaces missing desired capacity. Persistent workloads using host volumes can only land on nodes advertising the required names, so loss of all compatible nodes leaves them pending.

[Documentation index](../README.md) · [Previous: CLI workflows](cli.md) · [Next: Cookbook](cookbook.md)
