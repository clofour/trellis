# Volume responsibility and semantics

Trellis schedules storage; it does not provide storage. Replication, durability,
backup, attachment, failover, and data availability belong to the operator's
storage system (local disks, ZFS, DRBD, NFS, Ceph, cloud storage, or anything
else that exposes a directory on a node). Trellis deliberately has no CSI or
storage-plugin lifecycle.

## Identity

A **host volume name** is an opaque, cluster-wide identifier. Operators assign
the same name to paths containing the same logical dataset on every applicable
node. Paths are node-local configuration and never appear in job manifests.
Trellis neither compares nor synchronizes their contents.

Configure an advertisement with a repeatable node flag:

```sh
trellis-node --host-volume uploads=/srv/uploads \
  --host-volume database=/mnt/zfs/database ...
```

A configured volume is advertised only while its path exists as a directory.
The node reports the current set at registration and on every heartbeat. A lost
advertisement prevents new placement; it does not stop an allocation that is
already running. This avoids treating a transient mount probe as storage
fencing. The external storage system and operator remain responsible for safe
failure handling.

## Job requirements and mounting

Set `host_volume` on a task volume to require that identity:

```yaml
volumes:
  - name: uploads-mount
    host_volume: uploads
    path: /var/lib/app/uploads
    read_only: false
```

The scheduler places the entire task group only on a healthy node advertising
every host volume required by all its tasks. At allocation start, the agent
bind-mounts the node's configured directory at `path`. `name` identifies this
mount within the task; `host_volume` identifies the external dataset.

Omitting `host_volume` retains Trellis's legacy allocation-local managed
directory. That directory is not replicated and must not be assumed to follow a
rescheduled allocation to another node.

## Concurrent access

Host-volume advertisements are statements of **availability**, not leases or
locks. Trellis permits any number of allocations, including replicas on
different nodes, to mount the same host-volume identity concurrently. It does
not serialize writers, enforce single-writer attachment, fence stale mounts, or
infer consistency from `read_only`.

`read_only: true` makes that individual container mount read-only; it is not a
cluster-wide access claim. Jobs requiring single-writer or other attachment
semantics should use a replica count and placement constraints appropriate to
their storage system, while that system must enforce any required exclusion or
fencing. This intentionally keeps volume identity independent of its backing
implementation.
