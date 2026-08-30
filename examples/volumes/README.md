# Volumes

This example shows how Trellis-managed node-local storage behaves and when to
use an operator-provided host volume instead.

## Managed local volumes

A volume without `host_volume` is a directory managed by Trellis on the node
where the task runs. Its backing path is scoped by namespace, job, task, and
volume name, so the data survives container replacement and job revisions when
the replacement lands on the same node with the same identity.

Managed volumes are **node-local** and are not a scheduler-affinity mechanism.
If the allocation is later placed on another node, Trellis creates or reuses the
corresponding directory on that node; it does not copy the data from the old
node. A replacement can therefore start with an empty data directory after a
node failure or placement change.

Also note that the local path is not scoped by allocation ID. Multiple replicas
of the same job/task that land on one node and declare the same managed volume
name will mount the same directory on that node. Do not use this as an implicit
replicated-database storage design.

## Manifest

`app.yaml` runs a single PostgreSQL instance with a `pgdata` volume:

```yaml
volumes:
  - name: pgdata
    path: /var/lib/postgresql/data
```

Trellis creates the node-local directory at first use and mounts it at
`/var/lib/postgresql/data`.

## Volume fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Volume name, unique within the task |
| `path` | yes | Absolute mount path inside the container |
| `host_volume` | no | Mount an operator-managed host-volume identity instead of a Trellis-managed local directory |
| `read_only` | no | Mount the volume read-only (default false) |

## Host volumes

A host volume maps an operator-defined identity to a path on each node that
advertises it. This is the mechanism to use when the backing storage is managed
outside Trellis, including a network-backed mount:

```yaml
volumes:
  - name: uploads
    path: /data/uploads
    host_volume: uploads
```

Register the identity when starting an eligible node:

```sh
trellis-node ... --host-volume uploads=/mnt/nfs/uploads
```

The scheduler only places a task that requires `host_volume: uploads` on nodes
that currently advertise the `uploads` identity. Trellis does not mount NFS,
attach cloud disks, replicate data, or coordinate writers itself; the operator
is responsible for making the backing path safe and available on those nodes.

## Deploying

First, create the password secret:

```sh
printf %s "s3cr3t" | trellis --namespace acme secrets set postgres-password --stdin
```

Then apply the job:

```sh
trellis --namespace acme jobs apply --file app.yaml
```

The allocation starts, Trellis creates the local `pgdata` directory on its node,
and Postgres initializes the cluster there.

## Caveats

- `count: 1` does not pin the database to its current node. If that node is lost,
  Trellis may place a replacement elsewhere, where the local `pgdata` path can
  be empty. Use explicit node constraints, an operator-managed host volume, or a
  database replication/failover design when placement changes must preserve
  data access.
- Do not raise `count` for a database just to obtain HA. Replicas can land on the
  same node, where matching managed volume names share one directory, or on
  different nodes, where they see different directories. Database replication
  must be handled by the database system itself.
- Trellis does not snapshot or back up volume contents. Back up the underlying
  storage separately.
