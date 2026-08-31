# Job manifest reference

A manifest is YAML with one job.

```yaml
name: web
namespace: default
network:
  wireguard: true
task_groups:
  - name: frontend
    count: 2
    runtime: runc
    network_mode: ""
    api_access: false
    labels: {route: web}
    constraints: [{attribute: arch, value: amd64}]
    restart: {max_restarts: 3, window: 5m}
    update: {strategy: rolling, max_parallel: 1}
    tasks:
      - name: app
        image: docker.io/library/nginx:1.27
```

## Job fields

| Field | Meaning |
|---|---|
| `name`, `namespace` | Required safe identifiers, each at most 63 characters. |
| `network.wireguard` | Build a namespace overlay for isolated groups. |
| `task_groups` | One or more placement/scaling units. |

## Task-group fields

- `name`: unique group identifier; `count`: at least one.
- `runtime`: empty/default, `runc`, or `runsc`.
- `network_mode`: empty for isolation or `host`.
- `api_access`: injects `TRELLIS_ADDR`, `TRELLIS_TOKEN`, and `TRELLIS_NAMESPACE` for in-cluster API access. Grant it only to trusted images.
- `labels`: discovery/routing metadata. Keys start with a letter; values are at most 256 characters.
- `constraints`: exact matches against `os`, `arch`, or node labels.
- `restart`: `max_restarts` (zero or greater) during a positive duration `window`.
- `update`: `strategy` is `recreate` (the default) or `rolling`; positive `max_parallel` defaults effectively to one during rolling updates.
- `tasks`: one or more containers colocated in every allocation.

## Task fields

- `name` and `image` are required.
- `env` maps literal environment variables.
- `resources.cpu` is millicores; `resources.memory` is bytes.
- `ports` contains `host_port` (0 requests dynamic assignment) and required `container_port`.
- `volumes` contains a unique `name`, absolute container `path`, optional advertised `host_volume`, and `read_only`.
- `secrets` maps a stored name to `target: env` plus `env`, or `target: file` plus a clean path under `/run/trellis-secrets/`. File mode may be `0400`/`0600` (YAML numeric values are accepted); zero selects the default.
- `health_check` supports `http`, `tcp`, or `script`. HTTP/TCP require `port`; script requires `command`. Optional `interval`, `timeout`, and `threshold` override defaults.

Identifiers accept letters, digits, `_`, `.`, and `-`, must begin alphanumerically, and are limited to 63 characters. See the validated manifests under [`examples/`](../../examples/).
