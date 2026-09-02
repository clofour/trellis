# Job manifest reference

A **job manifest** is the canonical human-authored representation of one Trellis job. Manifests are YAML and use the vocabulary in the [Trellis user model](user-model.md). The HTTP API carries the same schema as JSON; JSON is an API representation, not a separate authoring model.

```yaml
name: web
namespace: default
task_groups:
  - name: frontend
    count: 2
    runtime: runc
    api_access: namespace
    labels:
      route: web
    constraints:
      - attribute: arch
        value: amd64
    restart:
      max_restarts: 3
      window: 5m
    update:
      strategy: rolling
      max_parallel: 1
    tasks:
      - name: nginx
        image: docker.io/library/nginx:1.27-alpine
        env:
          APP_ENV: production
        networking:
          mode: host
          ports:
            - host_port: 0
              container_port: 80
        resources:
          cpu: 100
          memory: 67108864
        health_check:
          type: http
          port: 80
          path: /
          interval: 5s
          timeout: 2s
          threshold: 2
```

Validate locally with `trellisctl jobs validate --file trellis.yaml`, preview with `trellisctl jobs diff --file trellis.yaml`, and apply with `trellisctl jobs apply --file trellis.yaml`. The dashboard's **Apply Manifest** editor accepts the same YAML.

## Job fields

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | Yes | Job identifier, unique within its namespace. |
| `namespace` | Yes | Namespace containing the job and its runtime allocations. |
| `task_groups` | Yes | One or more placement and scaling units. |

There is no job-level networking block. Network attachment belongs to each task because different tasks in one group may require different isolation.

## Task-group fields

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | Yes | Group identifier, unique within the job. |
| `count` | Yes | Desired allocation count; must be at least one. |
| `tasks` | Yes | One or more containers placed in every allocation. |
| `runtime` | No | Default runtime (`""`), `runc`, or `runsc`. |
| `labels` | No | Discovery and routing metadata. Keys begin with a letter; values are at most 256 characters. |
| `api_access` | No | API credential mode: `namespace` or `cluster`. Omit it for no injected API credentials. |
| `constraints` | No | Exact matches against `os`, `arch`, or node labels. Duplicate attributes are invalid. |
| `restart` | No | Retry policy for failed tasks. |
| `update` | No | Replacement strategy when execution-affecting desired state changes. |

### API access

`api_access` controls which control-plane credential is injected into every task in the group:

- `namespace` injects a persistent token restricted to **the namespace of this job**. The manifest cannot name a different namespace for this mode.
- `cluster` injects the cluster administrator token. It can perform cluster-wide and administrative operations, so use it only for fully trusted operator workloads.
- omitted means no API credential is injected.

Both enabled modes inject `TRELLIS_ADDR`, `TRELLIS_TOKEN`, and `TRELLIS_NAMESPACE`; `TRELLIS_NAMESPACE` is always initialized to the job's namespace. When TLS is configured, `TRELLIS_CA_CERT` contains the cluster CA PEM. A namespace token remains restricted to that job namespace even if a client changes the `X-Trellis-Namespace` header. A cluster token is not restricted by that default namespace and can deliberately make cluster-wide or differently scoped requests.

`api_access` is a task-group privilege boundary: every task in the group can read the credential. Do not colocate untrusted sidecars with an API-enabled controller. Prefer `namespace`; choose `cluster` only when the workload genuinely performs node, backup, secret-administration, cross-namespace, Raft, or other administrator operations.

For compatibility with older manifests and API clients, boolean `true` is read as `namespace` and `false` as disabled. New manifests should always use the explicit string modes.

`restart.max_restarts` is zero or greater and `restart.window` is a positive Go-style duration such as `5m`. Once the allowed failures in that window are exhausted, the allocation remains failed for operator diagnosis.

`update.strategy` is `recreate` (the default) or `rolling`. For rolling updates, `max_parallel` limits how many not-yet-healthy replacements may be in flight; zero uses the effective default of one.

Task groups are the unit of placement, scaling, updates, restart behavior, and draining. Every task in a group is coupled to that lifecycle.

## Task fields

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | Yes | Task identifier, unique within the group. |
| `image` | Yes | Pullable OCI image reference. Pin a version or digest for reproducible deployment. |
| `env` | No | Literal environment-variable map. Do not place credentials here. |
| `networking` | No | Network mode and, for host mode, optional host-port mappings. |
| `resources` | No | CPU in millicores and memory in bytes; values cannot be negative. |
| `volumes` | No | Allocation-local or advertised host-volume mounts. |
| `secrets` | No | References to namespace secrets delivered as environment variables or files. |
| `health_check` | No | HTTP, TCP, or script readiness/health observation. |

### Networking and ports

```yaml
networking:
  mode: host
  ports:
    - host_port: 0
      container_port: 8080
```

`networking.mode` is:

- omitted or `""`: isolated container networking with no external routes;
- `host`: join the node network namespace directly;
- `wireguard`: join the namespace WireGuard mesh in a private container network namespace.

Port mappings are valid only with `mode: host`. `container_port` must be 1–65535. `host_port: 0` requests a free port; a nonzero host port reserves that exact node port. WireGuard must be enabled on the nodes before a task requests `wireguard` mode.

### Resources

```yaml
resources:
  cpu: 250
  memory: 268435456
```

CPU is expressed in millicores and memory in bytes. The scheduler multiplies each task request by its group count when considering desired capacity.

### Volumes

```yaml
volumes:
  - name: cache
    path: /var/cache/app
  - name: data
    path: /var/lib/app
    host_volume: app-data
    read_only: false
```

Every container path must be absolute. Without `host_volume`, Trellis creates allocation-local storage below its node data directory. With `host_volume`, the name must be advertised by the selected node; Trellis does not create, replicate, snapshot, or back up that host data.

### Secrets

```yaml
secrets:
  - name: api-token
    target: env
    env: API_TOKEN
  - name: tls-key
    target: file
    path: /run/trellis-secrets/tls.key
    mode: 256 # decimal form of 0400
```

An environment target requires only a valid `env` name and may not collide with `env`. A file target requires a clean path below `/run/trellis-secrets/`; mode may be `0400` or `0600` (or their YAML numeric values), and zero selects the default. Names, environment targets, and file paths must be unique within a task.

### Health checks

HTTP and TCP checks require a port:

```yaml
health_check:
  type: http
  port: 8080
  path: /ready
  interval: 5s
  timeout: 2s
  threshold: 2
```

A script check instead requires a nonempty command:

```yaml
health_check:
  type: script
  command: ["/usr/local/bin/check-ready"]
```

`interval` and `timeout` are positive Go-style durations when set; `threshold` is at least one. A running task without an explicit health check is treated as healthy, which is useful for the first tutorial but weaker than application-aware readiness for a service.

## Identifiers and authoritative examples

Job, namespace, group, task, secret, and volume identifiers accept letters, digits, `_`, `.`, and `-`, must begin with a letter or digit, and are limited to 63 characters. Unknown or inconsistent manifest fields are rejected by local validation.

All checked-in example YAML manifests are parsed and validated in the test suite. Follow them in learning order from the [examples index](../../examples/README.md), rather than copying an advanced architecture as a first workload.

[Documentation index](../README.md) · [Previous: Core concepts](core-concepts.md) · [Next: CLI workflows](cli.md)
