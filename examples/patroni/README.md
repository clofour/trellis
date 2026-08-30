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
                    │  10.0.1.10:2379            │
                    └────────────┬───────────────┘
                                 │ leader election
              ┌──────────────────┼──────────────────┐
              ▼                  ▼                  ▼
        ┌─────────┐        ┌─────────┐        ┌─────────┐
        │  pg-1   │◄──WAL──│  pg-2   │        │  pg-3   │
        │ primary │        │ replica │        │ replica │
        │10.0.1.10│        │10.0.1.11│        │10.0.1.12│
        └─────────┘        └─────────┘        └─────────┘
```

## Manifests

| File | Purpose |
| --- | --- |
| `etcd.yaml` | Single-node etcd for Patroni leader election |
| `pg-1.yaml` | Patroni replica 1 — runs on the node labelled `pg-node=1` |
| `pg-2.yaml` | Patroni replica 2 — runs on the node labelled `pg-node=2` |
| `pg-3.yaml` | Patroni replica 3 — runs on the node labelled `pg-node=3` |

Each pg job runs with `count: 1` and `network_mode: host`. Using host networking
lets Patroni bind directly to the node's IP and port 5432, which is necessary
for PostgreSQL's replication connections.

## Prerequisites

### 1 — Dedicated database nodes

Designate three nodes as PostgreSQL hosts and label them so Trellis can
schedule the allocations on the right nodes. Labels are set when starting
`trellis-node`:

```sh
# On db-node-1 (IP 10.0.1.10)
trellis-node --label pg-node=1 …

# On db-node-2 (IP 10.0.1.11)
trellis-node --label pg-node=2 …

# On db-node-3 (IP 10.0.1.12)
trellis-node --label pg-node=3 …
```

### 2 — Edit the manifests

Replace the placeholder IPs in each file with your actual node IPs:

| File | Variable | Replace with |
| --- | --- | --- |
| `etcd.yaml` | `ETCD_ADVERTISE_CLIENT_URLS` | IP of pg-node=1 |
| `pg-1.yaml` | `POD_IP` | IP of pg-node=1 |
| `pg-2.yaml` | `POD_IP` | IP of pg-node=2 |
| `pg-3.yaml` | `POD_IP` | IP of pg-node=3 |

All three pg files share the same `ETCD_HOSTS` value — the IP of the node
running etcd (pg-node=1 in this example).

### 3 — Create secrets

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
etcd leader key initialises PostgreSQL and becomes primary; the others start
as replicas and begin streaming WAL.

Watch the cluster form:

```sh
# Patroni REST API on any node
curl http://10.0.1.10:8008/cluster | jq .
```

The cluster is ready when the primary shows `"role": "master"` and the two
replicas show `"role": "replica"` and `"state": "streaming"`.

## Connecting

Each node listens on port 5432. Connect to whichever address Patroni designates
as primary, or use [HAProxy](https://www.haproxy.org/) or
[PgBouncer](https://www.pgbouncer.org/) with the Patroni REST API to
auto-route to the current primary.

The Patroni REST API can be queried to find the current primary:

```sh
# Returns HTTP 200 on the primary, 503 on replicas — ideal as a health-check
# backend for a load balancer.
curl -s http://10.0.1.10:8008/primary   # 200 if pg-1 is primary
curl -s http://10.0.1.11:8008/primary   # 200 if pg-2 is primary
```

## Failover

If the primary node fails, Patroni detects the missing leader lock in etcd and
one of the replicas promotes itself automatically. The process takes a few
seconds.

To trigger a manual switchover (e.g. for maintenance):

```sh
# Using the patronictl CLI inside any running container
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

For a major version upgrade (e.g. 16 → 17), refer to the Spilo documentation
— a logical backup and restore is typically required.

## etcd availability

This example runs a single etcd node co-located on pg-node=1 for simplicity.
If that node fails, Patroni can no longer do leader election, which prevents
failover. For production, run a 3-node etcd cluster on separate nodes using
the same pattern of one job per replica with node constraints.

## Key environment variables

Spilo reads PostgreSQL and Patroni configuration from environment variables.
The following are set by these manifests; refer to the
[Spilo documentation](https://github.com/zalando/spilo/blob/master/ENVIRONMENT.rst)
for the full list.

| Variable | Description |
| --- | --- |
| `SCOPE` | Patroni cluster name — must be the same on all nodes |
| `PATRONI_NAME` | Unique name for this Patroni node |
| `POD_IP` | IP address Patroni advertises (PostgreSQL connect and REST API) |
| `ETCD_HOSTS` | Comma-separated list of etcd client endpoints |
| `PATRONI_RESTAPI_LISTEN` | Listen address for the Patroni HTTP API |
| `PGPASSWORD_SUPERUSER` | Password for the `postgres` superuser |
| `PGPASSWORD_REPLICATION` | Password for the replication user |
