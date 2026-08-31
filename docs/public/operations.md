# Operations

## Routine commands

```sh
trellisctl nodes list
trellisctl jobs list
trellisctl jobs status NAME
trellisctl jobs logs --tail 200 ALLOCATION
trellisctl jobs logs --follow ALLOCATION
trellisctl secrets list --namespace default
```

Add `--output json` to list/status/secret commands for automation.

## Drain and maintenance

`trellisctl nodes drain ID` prevents new placement and migrates allocations. Wait until workloads have healthy replacements before maintenance. `trellisctl nodes undrain ID` re-enables scheduling. `nodes remove ID` permanently removes a Raft member and is different from draining. Before stopping the leader, use `nodes transfer-leadership`.

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

Use `--expected-version N` for compare-and-swap (`0` means create only). Values are capped at 65,536 bytes. Rotation affects newly started allocations, so apply/restart the consuming workload afterward.

## Observability

The control plane exposes Prometheus metrics at `/metrics`. Job status and allocation events explain lifecycle transitions; logs proxy the node's allocation logs. Monitor leader availability, unhealthy/draining nodes, desired-versus-running/healthy counts, reconciliation latency, retries, and disk capacity for Raft, containerd, and volumes.

## Networking and TLS

Ports `8127`, `8128`, and `8129` must be reachable between appropriate cluster members; allow WireGuard UDP `51820` and DNS UDP `8053` when enabled. Never expose the unauthenticated transport surface to an untrusted network. Configure a CA and node certificates on every node and pass the CA/client certificate flags to the CLI. Advertised addresses must be routable from peers, not wildcard bind addresses.

## Failure recovery

A missed-heartbeat node becomes unhealthy; allocations may become lost after leader recovery grace and an availability timeout. Reconciliation replaces missing desired capacity. Persistent workloads using host volumes can only land on nodes advertising the required names, so loss of all compatible nodes leaves them pending. Preserve quorum: operate an odd number of Raft voters and avoid removing several members together.
