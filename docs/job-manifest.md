# Job manifest reference

Trellis job manifests are YAML documents. The CLI parses and validates a
manifest before submitting it. Check field names carefully: compatibility
parsing may ignore fields that are not part of the schema.

See the [getting-started guide](getting-started.md) for a walkthrough of
deploying manifests, and the [operations guide](operations.md) for
day-to-day job management commands.

## Complete example

```yaml
namespace: acme
name: storefront
network:
  wireguard: true
task_groups:
  - name: web
    count: 2
    runtime: runsc
    constraints:
      - attribute: os
        value: linux
      - attribute: arch
        value: amd64
    restart:
      max_restarts: 5
      window: 10m
    tasks:
      - name: server
        image: docker.io/library/nginx:alpine
        env:
          APP_ENV: production
        resources:
          cpu: 500
          memory: 268435456
        ports:
          - host_port: 0
            container_port: 80
        volumes:
          - name: content
            path: /usr/share/nginx/html
        health_check:
          type: http
          port: 80
          path: /
          interval: 10s
          timeout: 5s
          threshold: 3
```

## Top-level fields

| Field | Required | Description |
| --- | --- | --- |
| `namespace` | Yes | Isolation scope and part of the job's identity. |
| `name` | Yes | Job name, unique within the namespace. |
| `network.wireguard` | No | Enables WireGuard as the namespace network mechanism. Defaults to `false`. |
| `task_groups` | Yes | One or more task groups. |

Names for jobs, namespaces, groups, tasks, and volumes must be 1–63 characters.
They must begin with an ASCII letter or digit; remaining characters may also
include `_`, `.`, and `-`.

## Task-group fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | Yes | Unique group name within the job. |
| `count` | Yes | Replica count; must be at least 1. |
| `runtime` | No | `runc` or `runsc`; an omitted value uses `runc`. |
| `tasks` | Yes | One or more tasks colocated in each replica. |
| `labels` | No | Arbitrary key-value metadata attached to the task group. Allocation queries can filter by label key or key/value; labels do not create a separate service resource. |
| `network_mode` | No | Set to `host` to request host networking instead of the namespace WireGuard network. |
| `api_access` | No | When `true`, Trellis injects `TRELLIS_TOKEN` and `TRELLIS_ADDR` environment variables into the group's containers. The token is namespace-scoped and is intended for namespace-scoped control-plane APIs such as job and allocation queries. |
| `restart` | No | Restart budget for stopped tasks. Defaults to 3 restarts in a 10-minute window. |
| `constraints` | No | Node attributes that every placement must match. |
| `update` | No | Update strategy applied when the job revision changes. Defaults to `recreate`. |

Constraints are a list of attribute/value pairs. `os` and `arch` match the
node's reported operating system and architecture. Any other attribute is
matched against the node labels supplied with `trellis-node --label key=value`.
Constraint attributes must be unique within a task group and each value must be
non-empty.

Replica anti-affinity is soft: the scheduler uses replica count as a tiebreaker
between otherwise equal best-fit nodes. A group can still place multiple
replicas on one node when that is the only node with capacity.

When `restart` is present, `max_restarts` must be non-negative and `window`
must be a positive duration. Setting `max_restarts: 0` disables automatic
restarts. Durations use values such as `30s`, `5m`, or `1h`.

Each replica is one scheduling and colocation unit. Resource requests for all
tasks in a group are therefore multiplied by `count` across the job.

### Update strategy

By default Trellis stops old allocations before placing replacements when a
job revision changes. Set `update` to control this:

```yaml
update:
  strategy: rolling
  max_parallel: 1
```

| Field | Default | Description |
| --- | --- | --- |
| `strategy` | `recreate` | `recreate` stops old allocations before replacing them. `rolling` keeps old allocations while healthy replacements are brought up incrementally. |
| `max_parallel` | `1` | Maximum number of not-yet-healthy replacement allocations Trellis allows in flight during a rolling update. Only meaningful when `strategy` is `rolling`. |

With `rolling`, Trellis first places replacements while the old revision stays
running. As replacements become healthy, an equivalent number of old draining
allocations can be stopped. The same healthy replacement is not counted again
on later reconciliation passes.

Because the old allocation remains present until its replacement is healthy,
rolling updates need spare placement capacity for the in-flight replacements.
If the scheduler cannot place a replacement because of CPU, memory, ports,
volumes, constraints, or node availability, the rollout waits instead of
stopping a healthy old allocation to make room.

A task group without health checks is considered healthy after it reaches the
running phase, so define a meaningful health check when readiness must gate an
update.

## Task fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | Yes | Unique task name within its group. |
| `image` | Yes | Container image reference. |
| `env` | No | String-to-string environment variable map. |
| `resources.cpu` | No | CPU request and limit in millicores; must be non-negative. |
| `resources.memory` | No | Memory request and limit in bytes; must be non-negative. |
| `ports` | No | Host-to-container port mappings. |
| `volumes` | No | Managed node-local volumes or named operator-managed host volumes and absolute mount paths. |
| `health_check` | No | HTTP, TCP, or script health check. |
| `secrets` | No | Namespace-scoped secret references delivered as environment variables or memory-backed files. |

### Secrets

Jobs reference secret names; values never appear in manifests. File delivery
is preferred over environment delivery:

```yaml
secrets:
  - name: database-password
    target: env
    env: DATABASE_PASSWORD
  - name: tls-key
    target: file
    path: /run/trellis-secrets/tls.key
    mode: 0400
```

Secret names must be unique within a task. File paths must be clean absolute
paths below `/run/trellis-secrets`; modes may be `0400` (the default) or
`0600`. Referenced secrets are resolved when an allocation starts. Updating a
secret does not modify an already-running allocation.

### Ports

| Field | Rules |
| --- | --- |
| `host_port` | `0` requests a dynamically allocated port; otherwise 1–65535. |
| `container_port` | Required; 1–65535. |

### Volumes

Each volume requires a safe identifier in `name` and an absolute container
mount `path`. Volume names must be unique within a task.

Without `host_volume`, Trellis creates a managed node-local directory scoped by
namespace, job, task, and volume name. It is persistent on that node, but it is
not replicated and it does not create scheduler affinity to the node that
already contains the data. If an allocation is later placed elsewhere, the new
node gets its own local directory.

Set `host_volume` to mount an operator-provided volume identity instead. The
node must advertise that identity with `trellis-node --host-volume
name=/absolute/path`; the scheduler only places the allocation on a node where
the requested host volume is available. Trellis treats the backing path as
operator-managed storage and does not move or replicate it.

### Health checks

HTTP check:

```yaml
health_check:
  type: http
  port: 8080
  path: /health
```

TCP check:

```yaml
health_check:
  type: tcp
  port: 5432
```

Script check:

```yaml
health_check:
  type: script
  command: ["/bin/sh", "-c", "test -f /tmp/ready"]
```

HTTP and TCP checks require a port from 1 through 65535. Script checks require
a non-empty command. All check types accept these optional timing fields:

| Field | Default | Rules |
| --- | --- | --- |
| `interval` | `10s` | Time between checks; must be a positive duration when set. |
| `timeout` | `5s` | Maximum time for one check; must be a positive duration when set. |
| `threshold` | `3` | Consecutive passes or failures required to change health state; must be at least 1 when set. |
