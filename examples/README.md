# Examples

Runnable Trellis job manifests that demonstrate common patterns.

| Example | What it shows |
| --- | --- |
| [`reverse-proxy`](./reverse-proxy/) | nginx reverse proxy that dynamically tracks healthy allocations via the allocations API |
| [`rolling-update`](./rolling-update/) | Roll out a new image incrementally while healthy old allocations remain available |
| [`blue-green`](./blue-green/) | Two independent job slots; cut over by changing routing labels |
| [`canary`](./canary/) | Serve a small fraction of traffic from a new version before promoting it |
| [`api-access`](./api-access/) | Use `api_access: true` to query live allocation state from inside a container |
| [`secrets`](./secrets/) | Store encrypted secrets and inject them as environment variables or files |
| [`sidecar`](./sidecar/) | Colocate a helper container with the main application |
| [`volumes`](./volumes/) | Use node-local or operator-provided host storage |
| [`patroni`](./patroni/) | Sketch a PostgreSQL/Patroni topology and its storage/DCS requirements |
| [`wordpress`](./wordpress/) | Multi-job application with a separate database job |

## Prerequisites

All examples use the `acme` namespace in their manifests. Trellis does not
require a separate namespace-creation step; the namespace is part of the job
specification and scopes jobs, allocation queries, secrets, and networking.

Replace `your-registry/…` image references with your own registry and tags
before applying any manifest.
