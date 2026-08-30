# PostgreSQL HA with Patroni

This example sketches a three-member PostgreSQL cluster managed by
[Patroni](https://github.com/zalando/patroni) using
[Spilo](https://github.com/zalando/spilo).

Patroni, not Trellis, owns PostgreSQL leader election, replication, and database
failover. Trellis is responsible for placing and running the containers. A
Patroni failover is only possible while its distributed configuration store
(DCS) is available.

> This is an integration example, not a production-ready PostgreSQL design.
> Validate the Spilo/Patroni configuration, DCS topology, storage semantics,
> backup plan, and fencing behavior for your environment before relying on it.

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

Each PostgreSQL job has `count: 1` and a node-label constraint (`pg-node=1`,
`pg-node=2`, or `pg-node=3`). The constraint keeps that job on a node with the
matching operator label. Trellis DNS supplies job-level names such as
`pg-1.acme.trellis`; those names can resolve to a replacement allocation on the
same constrained node if Trellis restarts the job.

## Manifests

| File | Purpose |
| --- | --- |
| `etcd.yaml` | Single-node etcd for demonstration; constrained to `pg-node=1` |
| `pg-1.yaml` | Patroni member 1; constrained to `pg-node=1` |
| `pg-2.yaml` | Patroni member 2; constrained to `pg-node=2` |
| `pg-3.yaml` | Patroni member 3; constrained to `pg-node=3` |

The PostgreSQL manifests request `network_mode: host` and advertise Trellis DNS
names to Patroni. Review the networking mode and advertised addresses in your
own environment; Patroni peers must be able to reach each member's PostgreSQL
and REST API addresses.

Representative Patroni variables are:

```yaml
PATRONI_POSTGRESQL_LISTEN: 0.0.0.0:5432
PATRONI_POSTGRESQL_CONNECT_ADDRESS: pg-1.acme.trellis:5432
PATRONI_RESTAPI_LISTEN: 0.0.0.0:8008
PATRONI_RESTAPI_CONNECT_ADDRESS: pg-1.acme.trellis:8008
ETCD_HOSTS: etcd.acme.trellis:2379
```

## Prerequisites

### 1 — Dedicated database nodes

Label three Trellis nodes so the jobs can be constrained to predictable hosts:

```sh
# On db-node-1
trellis-node ... --label pg-node=1

# On db-node-2
trellis-node ... --label pg-node=2

# On db-node-3
trellis-node ... --label pg-node=3
```

The arbitrary constraint attributes in these manifests match node labels.

### 2 — Create secrets

```sh
printf %s "supersecret" | \
  trellis --namespace acme secrets set pg-superuser-password --stdin
printf %s "replsecret" | \
  trellis --namespace acme secrets set pg-replication-password --stdin
```

## Deploying

Start etcd first, then apply the three Patroni jobs:

```sh
trellis --namespace acme jobs apply --file etcd.yaml
trellis --namespace acme jobs status etcd

trellis --namespace acme jobs apply --file pg-1.yaml
trellis --namespace acme jobs apply --file pg-2.yaml
trellis --namespace acme jobs apply --file pg-3.yaml
```

Wait for etcd to be healthy before starting the Patroni members. Patroni then
uses etcd to bootstrap/elect a primary and establish PostgreSQL replication.
Use the Patroni REST API or `patronictl` from an environment that can reach the
members to inspect cluster state.

## Connecting

Do not assume `pg-1` is permanently the primary. Route writes through a
PostgreSQL-aware proxy or load balancer that follows Patroni's primary/leader
health endpoint, or otherwise discover the current Patroni leader before
connecting.

## Failover

If the PostgreSQL primary fails **and etcd remains available**, Patroni can
promote an eligible replica according to its own configuration. Trellis does
not perform that promotion.

Trellis currently has no `jobs exec` command, so a manual Patroni switchover is
not performed through the Trellis CLI. Run `patronictl` from a trusted
management host/container with network access to the Patroni REST APIs, or use
Patroni's supported REST interface directly.

## Upgrading PostgreSQL

For an image update, change one Patroni job at a time and wait for that member
to rejoin before moving to the next member:

```sh
# Edit pg-1.yaml to the desired image tag.
trellis --namespace acme jobs apply --file pg-1.yaml
trellis --namespace acme jobs status pg-1
```

Repeat for the remaining members only after validating replication health.
Major PostgreSQL upgrades require a database-specific upgrade plan; do not treat
a container image change as a major-version migration procedure.

## etcd availability

This example deliberately uses **one etcd member**, which is a single point of
failure. If it fails, Patroni loses the DCS needed for leader election and an
automatic database failover cannot safely proceed. Because the example places
etcd on `pg-node=1`, losing that node also loses the demo DCS.

A production Patroni deployment normally needs a quorum-capable DCS topology on
independent failure domains. If you run etcd yourself, design that cluster
separately rather than assuming Trellis will provide DCS quorum semantics.

## Storage

Patroni replication does not remove the need for durable PostgreSQL storage.
The provided manifests should be adapted to your storage backend. Trellis
managed local volumes are node-local and do not follow an allocation to another
node. Operator `host_volume` mounts can point at externally managed storage, but
Trellis does not attach, replicate, fence, snapshot, or back it up.

## Key environment variables

| Variable | Description |
| --- | --- |
| `SCOPE` | Patroni cluster name; must be consistent across members |
| `PATRONI_NAME` | Unique name for a Patroni member |
| `ETCD_HOSTS` | Etcd client endpoint(s) |
| `PATRONI_POSTGRESQL_LISTEN` | PostgreSQL bind address |
| `PATRONI_POSTGRESQL_CONNECT_ADDRESS` | PostgreSQL address advertised to peers and clients |
| `PATRONI_RESTAPI_LISTEN` | Patroni REST API bind address |
| `PATRONI_RESTAPI_CONNECT_ADDRESS` | Patroni REST API address advertised to peers |
| `PGPASSWORD_SUPERUSER` | Password variable used by the selected Spilo image/configuration |
| `PGPASSWORD_REPLICATION` | Replication password variable used by the selected Spilo image/configuration |

Refer to the upstream Spilo and Patroni documentation for the exact variables
and operational procedures supported by the image version you deploy.
