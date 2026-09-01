# Operations

Operational commands use the vocabulary in the [Trellis user model](user-model.md): apply/delete jobs, inspect allocations, and drain/undrain nodes. Raft and leadership controls are advanced control-plane operations rather than part of the normal workload model. The [CLI workflows guide](cli.md) covers contexts, planning, rollout watching, diagnosis, and log selection in detail.

## Routine workflow

```sh
trellis context current
trellis jobs validate --file trellis.yaml
trellis jobs diff --file trellis.yaml
trellis jobs apply --file trellis.yaml --wait
trellis jobs status NAME
trellis jobs diagnose NAME
trellis jobs logs NAME --tail 200
trellis jobs delete NAME
```

Add `--output json` to list/status/secret commands for automation. JSON is the API/automation representation; human-authored jobs remain YAML manifests.

## Drain and maintenance

`trellis nodes drain NODE` prevents new placement and migrates allocations. `NODE` may be the host/address displayed by `nodes list`, a unique UUID prefix, or a complete UUID. Wait until workloads have healthy replacements before maintenance. `trellis nodes undrain NODE` re-enables scheduling. `nodes remove NODE` permanently removes a node from the cluster and is different from draining.

## Advanced control-plane maintenance

Trellis uses Raft internally. If an operator deliberately needs to move control-plane leadership before maintenance, the compatibility/advanced command `trellis nodes transfer-leadership` requests a transfer to another voter. It is intentionally hidden from normal CLI help because workload operations should not require understanding Raft leadership.

Preserve quorum: operate an odd number of Raft voters and avoid removing several members together.

## Backups

```sh
trellis backup create --file trellis-backup.json
trellis backup restore trellis-backup.json
```

Backups contain desired jobs and encrypted secret records, not allocations, container images, local volume data, TLS private keys, or the secret encryption key. Restores schedule fresh allocations. Secure and separately back up the 32-byte secrets key configured with `--secrets-key`; encrypted records are unusable without it.

## Secrets

```sh
printf %s 'value' | trellis --namespace default secrets set db-password --stdin
trellis --namespace default secrets describe db-password
trellis --namespace default secrets delete db-password
```

Use `--expected-version N` for compare-and-swap (`0` means create only). Values are capped at 65,536 bytes. Rotation affects newly started allocations, so apply a workload revision or replace the consuming allocations afterward.

## Observability

The control plane exposes Prometheus metrics at `/metrics`. Job status and allocation events explain lifecycle transitions; logs proxy the node's allocation logs. Monitor leader availability, unhealthy/draining nodes, desired-versus-running/healthy counts, reconciliation latency, retries, and disk capacity for Raft, containerd, and volumes.

For normal workload diagnosis, start with `jobs status`. `ready`, `converging`, and `degraded` summarize desired-versus-observed state without collapsing allocation lifecycle and health. `jobs diagnose` then surfaces only the allocations that need attention, including reason/message, retry timing, and attempt count. `jobs logs NAME` reads matching allocation logs without requiring a full allocation ID.

## Networking and TLS

Ports `8127`, `8128`, and `8129` must be reachable between appropriate cluster members; allow WireGuard UDP `51820` and DNS UDP `8053` when enabled. Never expose the unauthenticated transport surface to an untrusted network. Configure a CA and node certificates on every node and pass the CA/client certificate flags to the CLI. Advertised addresses must be routable from peers, not wildcard bind addresses.

## Failure recovery

A missed-heartbeat node becomes unhealthy; allocations may become lost after leader recovery grace and an availability timeout. Reconciliation replaces missing desired capacity. Persistent workloads using host volumes can only land on nodes advertising the required names, so loss of all compatible nodes leaves them pending.

[Documentation index](../README.md) · [Previous: CLI workflows](cli.md) · [Next: Cookbook](cookbook.md)
