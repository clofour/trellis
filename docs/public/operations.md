# Operations

Operational commands use the vocabulary in the [Trellis user model](user-model.md): apply/delete jobs, inspect allocations, and drain/undrain nodes. Raft and leadership controls are advanced control-plane operations rather than part of the normal workload model. The [CLI workflows guide](cli.md) covers contexts, planning, rollout watching, diagnosis, and log selection in detail.

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

Add `--output json` to list/status/secret commands for automation. JSON is the API/automation representation; human-authored jobs remain YAML manifests.

## Add a node

Run the same setup script on the new Debian/Ubuntu machine. When prompted:

1. choose an advertise address reachable by the existing nodes;
2. answer **yes** to **Join an existing cluster?**;
3. enter an existing node's `host:8128` address;
4. enter the existing cluster token when prompted. The token input is not echoed.

The cluster token is the value stored in `/etc/trellis/trellis.env` on an existing node. Treat it as an administrator credential and transfer it over a secure channel rather than placing it in shell history.

After the new daemon starts, verify membership from an existing CLI context:

```sh
trellisctl nodes list
```

A joining node must use the existing cluster token; creating a second independent token does not create a shared cluster.

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

Backups contain desired jobs and encrypted secret records, not allocations, container images, local volume data, TLS private keys, or the secret encryption key. Restores schedule fresh allocations. Secure and separately back up the 32-byte secrets key configured with `--secrets-key`; encrypted records are unusable without it.

## Secrets

```sh
printf %s 'value' | trellisctl --namespace default secrets set db-password --stdin
trellisctl --namespace default secrets describe db-password
trellisctl --namespace default secrets delete db-password
```

Use `--expected-version N` for compare-and-swap (`0` means create only). Values are capped at 65,536 bytes. Rotation affects newly started allocations, so apply a workload revision or replace the consuming allocations afterward.

## Observability

The control plane exposes Prometheus metrics at `/metrics`. Job status and allocation events explain lifecycle transitions; logs proxy per-task allocation logs. Monitor leader availability, unhealthy/draining nodes, desired-versus-running/healthy counts, reconciliation latency, retries, and disk capacity for Raft, containerd, and volumes.

For normal workload diagnosis, start with `jobs status`. `ready`, `converging`, and `degraded` summarize desired-versus-observed state without collapsing allocation lifecycle and health. `jobs diagnose` then surfaces only the allocations that need attention, including reason/message, retry timing, and attempt count. `jobs logs NAME` reads matching task streams without requiring full internal runtime IDs.

## Networking and TLS

Ports `8127`, `8128`, and `8129` must be reachable between appropriate cluster members; allow WireGuard UDP `51820` and DNS UDP `8053` when enabled. Never expose the unauthenticated transport surface to an untrusted network. Configure a CA and node certificates on every node and pass the CA/client certificate flags to the CLI. Advertised addresses must be routable from peers, not wildcard bind addresses.

## Failure recovery

A missed-heartbeat node becomes unhealthy; allocations may become lost after leader recovery grace and an availability timeout. Reconciliation replaces missing desired capacity. Persistent workloads using host volumes can only land on nodes advertising the required names, so loss of all compatible nodes leaves them pending.

[Documentation index](../README.md) · [Previous: CLI workflows](cli.md) · [Next: Cookbook](cookbook.md)
