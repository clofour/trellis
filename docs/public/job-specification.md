# Job manifest reference

A **job manifest** is the first-party human-authored representation of one Trellis job. The CLI, dashboard, documentation, and examples use YAML because it is pleasant to edit, but the control-plane API does not process YAML. Consumers convert their representation into the canonical JSON `JobSpec` before calling Trellis.

> **Consumers own representation; Trellis owns meaning.** YAML, HCL, Python, forms, or another frontend may provide their own authoring conveniences. Consumers are responsible for converting those conveniences into canonical JSON. Trellis remains authoritative for validation, defaults, planning, revision semantics, and reconciliation.

The repository publishes two schemas:

- [`schemas/trellis-job.schema.json`](../../schemas/trellis-job.schema.json) describes the first-party YAML authoring representation and enables editor completion/diagnostics.
- [`schemas/trellis-job-api.schema.json`](../../schemas/trellis-job-api.schema.json) describes the canonical JSON API representation.

The schema improves editing but never replaces `trellisctl jobs validate` or server validation.

```yaml
# yaml-language-server: $schema=../../schemas/trellis-job.schema.json
name: web
namespace: default
task_groups:
  - name: frontend
    count: 2
    runtime: runc
    api_access:
      scope: namespace
      access: read
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
      - name: app
        image: ghcr.io/clofour/trellis-tutorial:v2
        networking:
          mode: host
          ports:
            - port: 8080
        resources:
          cpu: 100
          memory: 64MiB
        health_check:
          type: http
          port: 8080
          path: /health
          interval: 5s
          timeout: 2s
          threshold: 2
```

Because the sample uses host networking and reserves port 8080, its two replicas must run on different nodes. A rolling replacement also needs another compatible node with port 8080 available while old and new allocations overlap.

Validate locally with `trellisctl jobs validate --file trellis.yaml`, preview with `trellisctl jobs diff --file trellis.yaml`, and apply with `trellisctl jobs apply --file trellis.yaml`. The dashboard's **Apply Manifest** editor accepts the same YAML, converts its human values to canonical JSON, and asks the control plane for the same semantic plan.

## Representation boundary

The YAML layer accepts human-readable quantities:

```yaml
resources:
  memory: 256MiB
health_check:
  interval: 10s
```

The canonical JSON representation uses machine values instead:

```json
{
  "resources": { "memory": 268435456 },
  "health_check": { "interval": 10000000000 }
}
```

Memory is bytes and durations are nanoseconds in the current API model. Parsing strings such as `256MiB`, `4GB`, or `10s` is therefore a responsibility of the authoring consumer, not the Trellis HTTP API. Omitted values and their effective defaults remain Trellis semantics; a consumer should not duplicate those rules.

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
| `labels` | No | Discovery and routing metadata. |
| `api_access` | No | Least-privilege API credential request. Omit it for no injected API credentials. |
| `constraints` | No | Exact matches against `os`, `arch`, or node labels. |
| `restart` | No | Retry policy for failed tasks. |
| `update` | No | Replacement strategy when execution-affecting desired state changes. |

### API access

`api_access` has two independent dimensions:

```yaml
api_access:
  scope: namespace
  access: read
```

`scope` is `namespace` or `cluster`. `access` is `read` or `write`; write includes read capability.

- `namespace/read` is appropriate for discovery, observers, and namespace-local read-only controllers.
- `namespace/write` is appropriate for trusted controllers that deliberately mutate jobs in their own namespace.
- `cluster/read` can inspect cluster-scoped state and is the credential used by the read-only first-party dashboard.
- `cluster/write` is the normal high-privilege operator/controller credential for cluster-wide mutations.
- omitted means no API credential is injected.

A job may never delegate more authority than the credential submitting it. Namespace-scoped callers cannot request cluster-scoped workload credentials, and read-only callers cannot request write credentials. Planning enforces the same ceiling as apply so a preview cannot advertise a deployment the caller is not authorized to create.

The bootstrap cluster token is intentionally separate. Trellis no longer injects that root credential into workloads. Operations such as Raft membership changes, backup/restore, and node registration remain bootstrap/root operations rather than abilities granted by ordinary `cluster/write` credentials.

Enabled API access injects `TRELLIS_ADDR`, `TRELLIS_TOKEN`, and `TRELLIS_NAMESPACE`; when TLS is configured, `TRELLIS_CA_CERT` contains the cluster CA PEM. `TRELLIS_NAMESPACE` is initialized to the job namespace even for cluster-scoped credentials.

`api_access` is a task-group privilege boundary: every task in the group can read the credential. Do not colocate untrusted sidecars with an API-enabled controller. Request the narrowest scope and access level the workload needs.

`restart.max_restarts` is zero or greater and `restart.window` is a positive Go-style duration such as `5m`. Once the allowed failures in that window are exhausted, the allocation remains failed for operator diagnosis.

`update.strategy` is `recreate` (the default) or `rolling`. For rolling updates, `max_parallel` limits how many not-yet-healthy replacements may be in flight; zero uses the effective default of one.

Task groups are the unit of placement, scaling, updates, restart behavior, and draining. Every task in a group is coupled to that lifecycle.

## Task fields

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | Yes | Task identifier, unique within the group. |
| `image` | Yes | Pullable OCI image reference. Pin a version or digest for reproducible deployment. |
| `env` | No | Literal environment-variable map. Do not place credentials here. |
| `networking` | No | Network mode and, for host mode, optional port reservations. |
| `resources` | No | CPU in millicores and memory as a byte count or readable size. |
| `volumes` | No | Allocation-local or advertised host-volume mounts. |
| `secrets` | No | References to namespace secrets delivered as environment variables or files. |
| `health_check` | No | HTTP, TCP, or script readiness/health observation. |

### Networking and ports

```yaml
networking:
  mode: host
  ports:
    - port: 8080
```

`networking.mode` is:

- omitted or `""`: isolated container networking with no external routes;
- `host`: join the node network namespace directly;
- `wireguard`: join the namespace WireGuard mesh in a private container network namespace.

Port declarations are valid only with `mode: host`. Host networking has no Trellis NAT or port-forwarding layer, so there is no separate host/container port distinction in desired state. `port` is both the node port Trellis reserves and the port the process must listen on. It must be 1–65535. A fixed port can be used only once per node, so replicas reserving the same port need distinct nodes. WireGuard must be enabled on the nodes before a task requests `wireguard` mode.

### Resources

```yaml
resources:
  cpu: 250
  memory: 256MiB
```

CPU is expressed in millicores. The first-party YAML representation accepts a raw byte count or readable binary/decimal size such as `256MiB`, `1GiB`, or `500MB`; canonical JSON represents memory as integer bytes. The scheduler multiplies each task request by its group count when considering desired capacity.

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
  path: /health
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

`interval` and `timeout` are positive Go-style durations in the YAML representation when set; `threshold` is at least one. A running task without an explicit health check is treated as healthy, which is useful for the first tutorial but weaker than application-aware readiness for a service.

## Validation and editor tooling

Job, namespace, group, task, secret, and volume identifiers accept letters, digits, `_`, `.`, and `-`, must begin with a letter or digit, and are limited to 63 characters. Unknown YAML fields are rejected by the first-party parser.

The YAML schema is intended for VS Code, Neovim, Zed, and other editors that support YAML language-server schemas. Checked-in examples include a `yaml-language-server` schema directive so completion and basic diagnostics work immediately when the repository is opened. Schema diagnostics are structural assistance only; `trellisctl jobs validate`, `/v1/jobs/plan`, and apply use Trellis's authoritative validator, which reports all independently actionable validation issues with paths and error codes.

Follow checked-in examples in learning order from the [examples index](../../examples/README.md), rather than copying an advanced architecture as a first workload.

[Documentation index](../README.md) · [Previous: Core concepts](core-concepts.md) · [Next: CLI workflows](cli.md)
