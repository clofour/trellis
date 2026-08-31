# Trellis documentation

Trellis is an experimental, containerd-backed workload orchestrator. These pages describe the implementation in this repository; they do not promise production readiness.

## Public documentation

1. [Getting Started](public/getting-started.md)
2. [Core concepts](public/core-concepts.md)
3. [Operations](public/operations.md)
4. [Dashboard](public/dashboard.md)
5. [Cookbook](public/cookbook.md)
6. [Job manifest reference](public/job-specification.md)

## Developer documentation

- [Architecture and major concepts](developer/architecture.md)
- [Control plane, reconciliation, and lifecycle](developer/control-plane.md)
- [Runtime, networking, storage, and secrets](developer/node-internals.md)
- [HTTP API](developer/api.md)
- [Development and testing](developer/development.md)

> Security note: use TLS, protect the cluster token, bind administrative endpoints to a trusted network, and keep the secrets encryption key outside the data directory.
