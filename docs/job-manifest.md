# Job manifest reference

Trellis job manifests are YAML documents. The CLI parses and validates a
manifest before submitting it. Check field names carefully: compatibility
parsing may ignore fields that are not part of the schema.

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
| `labels` | No | Arbitrary key-value metadata attached to the task group. Labels do not create a user-facing service resource. |
| `network_mode` | No | Set to `host` to give the group's containers direct access to the host's network namespace instead of an isolated one. |
| `api_access` | No | When `true`, Trellis injects `TRELLIS_TOKEN` and `TRELLIS_ADDR` environment variables into the group's containers. The token is scoped to the job's namespace and can be used to call documented namespace-scoped control-plane APIs. |

Each replica is one scheduling and colocation unit. Resource requests for all
tasks in a group are therefore multiplied by `count` across the job.

## Task fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | Yes | Unique task name within its group. |
| `image` | Yes | Container image reference. |
| `env` | No | String-to-string environment variable map. |
| `resources.cpu` | No | CPU request and limit in millicores; must be non-negative. |
| `resources.memory` | No | Memory request and limit in bytes; must be non-negative. |
| `ports` | No | Host-to-container port mappings. |
| `volumes` | No | Named persistent local volumes and absolute mount paths. |
| `health_check` | No | HTTP, TCP, or script health check. |

### Ports

| Field | Rules |
| --- | --- |
| `host_port` | `0` requests a dynamically allocated port; otherwise 1–65535. |
| `container_port` | Required; 1–65535. |

### Volumes

Each volume requires a safe identifier in `name` and an absolute container
mount `path`. Volume names must be unique within a task. Storage is local to a
node and scoped by namespace.

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
a non-empty command.

## Applying a manifest

```sh
trellis --server-addr leader.example:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file trellis.yaml
```

The namespace used by `jobs apply` always comes from the manifest. Use the
global `--namespace` option with `jobs status`, `jobs logs`, and `jobs destroy`.
