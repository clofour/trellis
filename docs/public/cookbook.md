# Cookbook

The examples are complete manifests or runnable request snippets. Validate a manifest by applying it; the repository test suite also parses every `examples/**/*.yaml`.

## Colocate a sidecar

Put both tasks in one task group. They share allocation placement and lifecycle, while each remains a distinct container. See [`examples/sidecar/trellis.yaml`](../../examples/sidecar/trellis.yaml).

## Deliver secrets safely

Create secrets before applying the job. Prefer file injection for credentials consumed from disk and environment injection only when required. See [`examples/secrets/`](../../examples/secrets/).

## Select persistent nodes

Advertise host-volume names from nodes and reference `host_volume` in a task. The scheduler filters incompatible nodes. See [`examples/volumes/trellis.yaml`](../../examples/volumes/trellis.yaml).

## Run WordPress

The WordPress example colocates the web process and MariaDB for clarity and uses persistent host volumes. This is approachable but couples their scaling and failure domains; split production data services deliberately. See [`examples/wordpress/`](../../examples/wordpress/).

## Release safely

- **Rolling:** built-in `strategy: rolling` with a health check gates removal of old allocations.
- **Blue/green:** keep two independently named jobs and switch the proxy's route label.
- **Canary:** keep stable and canary jobs discoverable under the same route and use `trellis/weight` with `trellis-proxy-sync`.

See [`examples/deployment-strategies/`](../../examples/deployment-strategies/). Blue/green switching and canary weighting are external routing workflows, not scheduler strategy keywords.

## Call the API from a workload

Set `api_access: true` only on a trusted task group. Trellis injects a namespace token and address. The API example uses those variables with Bearer authentication. See [`examples/api-access/`](../../examples/api-access/).

## Run Patroni

The Patroni example demonstrates three replicas, API discovery, health checks, and node-local persistent volumes. Real PostgreSQL HA also requires correct Patroni/DCS integration, backups, fencing, and network design; treat it as a starting point. See [`examples/patroni/`](../../examples/patroni/).
