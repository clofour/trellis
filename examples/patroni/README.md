# Patroni architecture skeleton

**Level:** Advanced architecture · **Prerequisites:** complete the earlier learning path, prepare at least three failure-separated nodes, and design an external Patroni DCS and backup system

This directory demonstrates how Trellis can place and monitor three Patroni/PostgreSQL containers. It is intentionally **not a turnkey HA database**. Trellis supplies container scheduling, health observations, namespace networking, secret delivery, and local-volume placement; Patroni and a supported distributed configuration store (DCS) must supply database membership, leader election, replication, and promotion safety.

## What the manifest provides

- Three `postgres` group allocations with a normal scheduler preference to spread replicas.
- A `database=true` node constraint and required `patroni-data` host volume.
- Task-level namespace WireGuard networking with the `runsc` runtime.
- PostgreSQL and Patroni REST listeners inside the WireGuard-attached task, plus a script `/health` probe.
- Namespace-scoped Trellis API access for optional endpoint discovery.
- Environment-delivered superuser and replication credentials.
- Rolling replacement with one in-flight replacement.

Normal replica spreading is a preference, not a hard topology constraint. Verify actual node placement and do not assume three allocations imply three independent failure domains.

## Required work before applying

### 1. Provision storage and nodes

Prepare at least three nodes, ideally in separate failure domains:

```sh
sudo install -d -m 0700 /srv/trellis/patroni
sudo trellis \
  --cluster-token "$TRELLIS_TOKEN" \
  --label database=true \
  --host-volume patroni-data=/srv/trellis/patroni
```

Each path is node-local and contains a different PostgreSQL data directory. Back it up independently. A host-volume name does not replicate bytes between nodes.

### 2. Supply a real DCS and Patroni configuration

The manifest's static `PATRONI_NAME=trellis-member` is a placeholder and is invalid for a real cluster because every member needs a unique identity. Build an entrypoint that derives identity from stable allocation/node context or injects explicitly unique configuration. Configure Patroni for etcd, Consul, or another Patroni-supported DCS with quorum and TLS appropriate to your environment.

`discover-members.sh` queries Trellis allocations labeled `service:patroni`; it can help a controller find endpoints, but the Trellis catalog is eventually reconciled service discovery—not Patroni's consensus DCS. Never use the catalog alone to decide which PostgreSQL member may accept writes.

### 3. Create credentials

```sh
openssl rand -base64 32 | \
  trellisctl --namespace database secrets set postgres-password --stdin
openssl rand -base64 32 | \
  trellisctl --namespace database secrets set replication-password --stdin
```

Confirm the selected Patroni image actually consumes `PGPASSWORD_SUPERUSER` and `PGPASSWORD_STANDBY`, or adapt the environment to that image's documented configuration contract. Pin the image by digest after qualification.

### 4. Design networking and client routing

Open the WireGuard UDP port between nodes and ensure advertised endpoints are routable. PostgreSQL clients should not pick an arbitrary catalog member for writes. Route through a Patroni-aware proxy or controller that checks the leader/read-only REST endpoints and distinguishes primary from replica traffic.

## Apply and inspect

Only after replacing the placeholders and configuring the DCS:

```sh
trellisctl jobs apply --file examples/patroni/trellis.yaml
trellisctl --namespace database jobs status patroni
trellisctl nodes list
```

Check that all three allocations are on intended nodes, Patroni reports one leader, replicas stream successfully, and write/read routing follows the desired roles. Inspect allocation events when a health check fails; Trellis health alone does not prove replication is current or promotion is safe.

## Failure and upgrade tests

Before storing production data, demonstrate all of the following in a disposable environment:

1. loss and return of a replica;
2. loss of the PostgreSQL leader and exactly-one safe promotion;
3. loss of DCS quorum without split-brain writes;
4. node drain and replacement without losing the only current copy;
5. WAL archiving, base backup, point-in-time restore, and credential recovery;
6. `pg_rewind` or reinitialization of the former primary;
7. PostgreSQL/Patroni rolling upgrades with version-skew compatibility;
8. restoration when Trellis desired state and database data are recovered separately.

Trellis backups contain the job and encrypted secret records, not PostgreSQL data or the separate secrets encryption key. Database backup and fencing remain application/operator responsibilities.

[Examples index](../README.md) · [Learning path](../../docs/public/learning-path.md)
