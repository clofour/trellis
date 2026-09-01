# Trellis documentation

Trellis is an experimental, containerd-backed workload orchestrator. These pages describe the implementation in this repository; they do not promise production readiness.

The public documentation follows one user model across the CLI, dashboard, manifests, and examples. Start with the user model if terminology is unfamiliar; developer documentation intentionally uses lower-level implementation terms where needed.

## Public documentation

1. [User model](public/user-model.md)
2. [Getting started](public/getting-started.md)
3. [CLI workflows](public/cli.md)
4. [Core concepts](public/core-concepts.md)
5. [Job manifest reference](public/job-specification.md)
6. [Operations](public/operations.md)
7. [Cookbook](public/cookbook.md)
8. [Dashboard](public/dashboard.md)
9. [Examples](../examples/README.md)

## Developer documentation

- [Architecture and major concepts](developer/architecture.md)
- [Control plane, reconciliation, and lifecycle](developer/control-plane.md)
- [Runtime, networking, storage, and secrets](developer/node-internals.md)
- [HTTP API](developer/api.md)
- [Development and testing](developer/development.md)

> Security note: use TLS, protect the cluster token, bind administrative endpoints to a trusted network, and keep the secrets encryption key outside the data directory.
