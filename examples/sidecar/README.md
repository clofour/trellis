# Sidecar pattern

This example shows how to colocate a helper container alongside a main
application by placing multiple tasks in one task group.

## What Trellis guarantees

A task group is the scheduling unit. Every task in one replica is placed on the
same node and started/stopped as part of the same allocation lifecycle.

Colocation does **not** implicitly make separately declared managed volumes the
same directory, and tasks do not automatically share one container network
namespace. If two tasks need to share files, configure an explicit storage
mechanism that both can mount. If they need to communicate, use an address/port
that is actually reachable between their containers rather than assuming
`localhost` refers to the other task.

## Manifest

`app.yaml` defines two colocated tasks:

- `app` — the primary application
- `log-shipper` — a Fluent Bit helper

The example intentionally uses one replica and an operator-provided host volume
named `api-logs`. Register that volume on the node before applying the job, for
example:

```sh
trellis-node ... --host-volume api-logs=/srv/trellis/api-logs
```

Both tasks reference the same `host_volume` identity, so Trellis schedules the
allocation only on a node that advertises it and mounts the same backing
directory into both containers:

```yaml
tasks:
  - name: app
    volumes:
      - name: logs
        path: /var/log/api
        host_volume: api-logs

  - name: log-shipper
    volumes:
      - name: logs
        path: /var/log/api
        host_volume: api-logs
        read_only: true
```

This is different from merely giving both tasks a managed volume named `logs`.
Managed local volume paths are scoped by task name, so equal names in two tasks
do not make them shared.

## Replication

A single operator host-volume identity represents one backing path on each node
that advertises it. If you raise this example's `count`, multiple replicas that
land on the same node can therefore see the same host directory. For replicated
applications, design shared or per-replica storage explicitly rather than
assuming Trellis creates one private shared volume per task-group replica.

## Resources and health

Each task gets its own resource request. The example gives the app 500 CPU
millicores and 256 MiB, and the log shipper 50 millicores and 32 MiB.

Health checks are per task. The allocation's combined health reflects the tasks
that Trellis is checking; a task without a health check is treated as healthy
once it is running.

## Deploying

After configuring `api-logs` on an eligible node:

```sh
trellis --namespace acme jobs apply --file app.yaml
trellis --namespace acme jobs status api
```

The allocation contains both tasks and the scheduler keeps them colocated on
the same node.
