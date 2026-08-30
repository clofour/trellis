# Volumes

This example shows how to give a container persistent storage using Trellis
named volumes.

## How it works

A named volume is a directory managed by Trellis on the node where the
allocation runs. The volume is created the first time an allocation uses it and
persists across container restarts and redeployments. If the same job is
redeployed and its allocation lands on the same node, the data survives.

Named volumes are **node-local**: if an allocation moves to a different node,
the new allocation starts with an empty volume. This makes named volumes
suitable for single-instance stateful workloads (databases, caches, file
stores) that are pinned to one node by design, but not for services that are
expected to migrate freely.

For data that must survive node changes, use a host volume backed by network
storage (NFS, a cloud block device, etc.) and mount it with `host_volume`.

## Manifest

`app.yaml` runs a single PostgreSQL instance with a `pgdata` volume:

```yaml
volumes:
  - name: pgdata
    path: /var/lib/postgresql/data
```

Trellis creates the volume directory on the node at first use and mounts it
into the container at `/var/lib/postgresql/data`. The database files written
there survive container restarts and redeployments.

## Volume fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Volume name, unique within the task group |
| `path` | yes | Absolute mount path inside the container |
| `host_volume` | no | Mount an operator-managed host path instead of a Trellis-managed volume |
| `read_only` | no | Mount the volume read-only (default false) |

## Host volumes

A host volume lets you mount a pre-existing directory on the node — useful for
network-backed storage:

```yaml
volumes:
  - name: uploads
    path: /data/uploads
    host_volume: uploads   # operator registered this with --host-volume uploads=/mnt/nfs/uploads
```

The node operator registers host volumes when starting `trellis-node`:

```sh
trellis-node --host-volume uploads=/mnt/nfs/uploads
```

Trellis only schedules allocations that require a host volume on nodes that
advertise it.

## Deploying

First, create the password secret:

```sh
echo -n "s3cr3t" | trellis --namespace acme secrets set postgres-password
```

Then apply the job:

```sh
trellis --namespace acme jobs apply --file app.yaml
```

The allocation starts, Trellis creates the `pgdata` volume directory on the
node, and Postgres initializes the cluster there.

## Caveats

- With `count: 1`, Trellis places one allocation. If that node goes offline
  the allocation does not restart on another node, because the data cannot
  follow it. This is intentional for databases; use a HA Postgres setup or an
  external managed database for higher availability.
- Do not set `count` above 1 for a stateful job that shares a single volume —
  each allocation gets its own volume directory, not a shared one.
