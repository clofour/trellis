# Examples

Runnable Trellis job manifests that demonstrate common patterns.

| Example | What it shows |
| --- | --- |
| [`reverse-proxy`](./reverse-proxy/) | nginx reverse proxy that dynamically tracks healthy allocations via the allocations API |
| [`rolling-update`](./rolling-update/) | Deploy a new image version one allocation at a time with zero downtime |
| [`blue-green`](./blue-green/) | Two independent job slots; instant cutover by toggling a label |
| [`canary`](./canary/) | Serve a small fraction of traffic from a new version before promoting it |
| [`api-access`](./api-access/) | Use `api_access: true` to query live allocation state from inside a container |
| [`secrets`](./secrets/) | Store encrypted secrets and inject them as environment variables or files |
| [`sidecar`](./sidecar/) | Colocate a helper container (log shipper, exporter) with the main application |
| [`volumes`](./volumes/) | Attach persistent local storage to a single-instance stateful workload |
| [`patroni`](./patroni/) | Three-replica PostgreSQL HA cluster with Patroni leader election and automatic failover |
| [`wordpress`](./wordpress/) | Multi-job application with a separate database job |

## Prerequisites

All examples target the `acme` namespace. Create it if it does not exist:

```sh
trellis namespaces create acme
```

Replace `your-registry/…` image references with your own registry and tags
before applying any manifest.
