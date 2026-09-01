# Volume patterns

**Level:** Intermediate · **Prerequisites:** complete `hello` and prepare a node path, label, and advertised host-volume name

This example mounts both Trellis-managed allocation storage and an operator-provisioned host volume into a long-running nginx task. It is intended for operators deciding what should survive allocation replacement and where a stateful task may run; nginx merely keeps the demonstration allocation alive and does not use the mounted paths as application data.

## Storage in the manifest

| Volume | Kind | Lifecycle |
|---|---|---|
| `scratch` | Allocation-managed | Created below the node data directory and suitable for replaceable cache/work files. |
| `database` | Named host volume `app-data` | Resolves to an absolute host path configured when `trellis` starts. Trellis mounts but does not create, replicate, snapshot, or back up that data. |

The group also requires node label `storage=fast`. Scheduling succeeds only on a healthy node that has both that exact label and an available `app-data` directory.

## Prepare a node

Create and secure the path before starting the node:

```sh
sudo install -d -m 0750 /srv/trellis/app-data
sudo trellis \
  --cluster-token "$TRELLIS_TOKEN" \
  --label storage=fast \
  --host-volume app-data=/srv/trellis/app-data
```

Repeat `--label` and `--host-volume` for additional values. The name in `--host-volume` must match the manifest; the host path must be absolute and already exist. Ensure its ownership matches the UID/GID used by the container image.

## Deploy and verify placement

```sh
trellisctl jobs validate --file examples/volumes/trellis.yaml
trellisctl jobs apply --file examples/volumes/trellis.yaml --wait
trellisctl --namespace default jobs status volumes-demo
trellisctl nodes list --output json
```

The allocation should land on a node reporting `app-data`. If it remains unplaced, check node health, the `storage` label, directory existence, and free CPU/memory.

## Recovery and scaling

A host-volume name is a capability, not a globally shared volume. If two nodes advertise `app-data=/srv/trellis/app-data`, those directories may contain completely different bytes. Constrain a single-writer workload deliberately, or use application-level replication/shared storage. Back up `/srv/trellis/app-data` independently and test restoration before relying on it.

Scaling this manifest above one replica is unsafe unless the application supports multiple writers and every eligible volume path has the required data semantics. Draining the only compatible node cannot make its local bytes appear elsewhere.

[Examples index](../README.md) · [Next: Sidecars](../sidecar/)
