# PostgreSQL HA with Patroni

This example runs a three-replica PostgreSQL cluster managed by
[Patroni](https://github.com/zalando/patroni) using
[Spilo](https://github.com/zalando/spilo) (Zalando's production-grade
Patroni+PostgreSQL Docker image).

Patroni coordinates leader election and streaming replication. One node is the
primary (accepts writes); the other two are synchronous or asynchronous replicas.
If the primary fails, Patroni promotes a replica automatically via etcd.

## Architecture

```
                    ┌────────────────────────────┐
                    │  etcd (DCS)                │
                    │  etcd.acme.trellis:2379    │
                    └────────────┬───────────────┘
                                 │ leader election
              ┌──────────────────┼──────────────────┐
              ▼                  ▼                  ▼
        ┌─────────┐        ┌─────────┐        ┌─────────┐
        │  pg-1   │◄──WAL──│  pg-2   │        │  pg-3   │
        │ primary │        │ replica │        │ replica │
        └─────────┘        └─────────┘        └─────────┘
```

Trellis DNS (`<job>.<namespace>.trellis`) provides stable hostnames for every
job. Because each pg job has `count: 1` and is pinned to a dedicated node via
a constraint, `pg-1.acme.trellis` always resolves to the same allocation. No
hardcoded IPs are needed anywhere in the manifests.

## Manifests

| File | Purpose |
| --- | --- |
| `etcd.yaml` | Single-node etcd for Patroni leader election — runs on node `pg-node=1` |
| `pg-1.yaml` | Patroni replica 1 — runs on node labelled `pg-node=1` |
| `pg-2.yaml` | Patroni replica 2 — runs on node labelled `pg-node=2` |
| `pg-3.yaml` | Patroni replica 3 — runs on node labelled `pg-node=3` |

Each pg job runs with `count: 1` and `network_mode: host`. Host networking lets
Patroni bind directly to the node's network interfaces — necessary for
PostgreSQL's replication connections.

The key Patroni variables in each pg job:

```yaml
PATRONI_POSTGRESQL_LISTEN: 0.0.0.0:5432         # bind on all interfaces
PATRONI_POSTGRESQL_CONNECT_ADDRESS: pg-1.acme.trellis:5432   # advertised address
PATRONI_RESTAPI_LISTEN: 0.0.0.0:8008
PATRONI_RESTAPI_CONNECT_ADDRESS: pg-1.acme.trellis:8008
ETCD_HOSTS: etcd.acme.trellis:2379
```

Separating listen and connect addresses is standard Patroni practice. The DNS
names resolve to the node's actual IP at connection time, so they work the same
way as a static IP would.

## Prerequisites

### 1 — Dedicated database nodes

Designate three nodes as PostgreSQL hosts and label them so Trellis can pin the
allocations to the right nodes. Labels are set when starting `trellis-node`:

```sh
# On db-node-1
trellis-node --label pg-node=1 …

# On db-node-2
trellis-node --label pg-node=2 …

# On db-node-3
trellis-node --label pg-node=3 …
```

### 2 — Create secrets

```sh
echo -n "supersecret" | trellis --namespace acme secrets set pg-superuser-password
echo -n "replsecret"  | trellis --namespace acme secrets set pg-replication-password
```

## Deploying

Start etcd first, then apply the three Patroni nodes in any order:

```sh
trellis --namespace acme jobs apply --file etcd.yaml
trellis --namespace acme jobs status etcd   # wait for healthy

trellis --namespace acme jobs apply --file pg-1.yaml
trellis --namespace acme jobs apply --file pg-2.yaml
trellis --namespace acme jobs apply --file pg-3.yaml
```

Patroni bootstraps the cluster on first start. The first node to acquire the
etcd leader key initialises PostgreSQL and becomes primary; the others start as
replicas and begin streaming WAL.

Watch the cluster form:

```sh
curl http://pg-1.acme.trellis:8008/cluster | jq .
```

The cluster is ready when the primary shows `"role": "master"` and the two
replicas show `"role": "replica"` and `"state": "streaming"`.

## Connecting

Each node listens on port 5432. Connect to whichever address Patroni designates
as primary. The Patroni REST API returns HTTP 200 on the primary and 503 on
replicas — this is the standard backend health check for a load balancer:

```sh
curl -s http://pg-1.acme.trellis:8008/primary   # 200 if pg-1 is primary
curl -s http://pg-2.acme.trellis:8008/primary   # 200 if pg-2 is primary
curl -s http://pg-3.acme.trellis:8008/primary   # 200 if pg-3 is primary
```

Point a HAProxy or PgBouncer frontend at these health endpoints to automatically
route writes to whichever node is currently primary.

## Failover

If the primary node fails, Patroni detects the missing leader lock in etcd and
promotes a replica automatically. The process takes a few seconds.

To trigger a manual switchover (e.g. for maintenance):

```sh
trellis --namespace acme jobs exec pg-1 -- \
    patronictl -c /home/postgres/postgres.yml switchover pg-cluster
```

## Upgrading PostgreSQL

Spilo image tags encode the PostgreSQL major version (`spilo-16` for Postgres
16). To upgrade to a new minor release, update the image tag in all three pg
files and reapply them one at a time:

```sh
# Edit pg-1.yaml: image: ghcr.io/zalando/spilo-16:3.2-p2
trellis --namespace acme jobs apply --file pg-1.yaml
# Wait for pg-1 to be healthy, then repeat for pg-2 and pg-3.
```

For a major version upgrade (e.g. 16 → 17), refer to the Spilo documentation —
a logical backup and restore is typically required.

## etcd availability

This example runs a single etcd node for simplicity. If that node fails,
Patroni can no longer do leader election, which prevents automatic failover.
For production, run a 3-node etcd cluster on separate nodes using the same
pattern of one job per replica with node constraints.

## Key environment variables

| Variable | Description |
| --- | --- |
| `SCOPE` | Patroni cluster name — must be identical on all nodes |
| `PATRONI_NAME` | Unique name for this Patroni node |
| `ETCD_HOSTS` | Etcd client endpoint(s) — uses Trellis DNS |
| `PATRONI_POSTGRESQL_LISTEN` | PostgreSQL bind address |
| `PATRONI_POSTGRESQL_CONNECT_ADDRESS` | PostgreSQL address advertised to peers and clients |
| `PATRONI_RESTAPI_LISTEN` | Patroni REST API bind address |
| `PATRONI_RESTAPI_CONNECT_ADDRESS` | Patroni REST API address advertised to peers |
| `PGPASSWORD_SUPERUSER` | Password for the `postgres` superuser |
| `PGPASSWORD_REPLICATION` | Password for the replication user |

Refer to the [Spilo environment reference](https://github.com/zalando/spilo/blob/master/ENVIRONMENT.rst)
for the full list.
