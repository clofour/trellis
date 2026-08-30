# Trellis

Trellis is a container scheduler built on containerd. It sits in the space between rolling your own deployment scripts and adopting Kubernetes — a real orchestrator for container workloads, without the operational complexity.

Every project that ships software ends up rebuilding the same infrastructure: workload placement, health checks, rolling updates, port allocation. Tools like Coolify improve the developer experience, but at their core they are not orchestrators. Kubernetes is a full orchestrator, but it brings significant complexity that many workloads simply do not need. Trellis is closer to Nomad in spirit: a lightweight, focused scheduler you can understand and operate yourself.

Every machine runs the same `trellis-node` daemon. Raft consensus elects one node to serve the control-plane API and reconcile jobs, while every node continues to run allocations and participate in the next election.

## Design principles

**Modular and extensible, but focused.** Trellis exposes clean primitives you can build on. The core repository stays lean — only the essentials live here. If you need something specific to your environment, you can extend Trellis yourself rather than waiting on a plugin ecosystem. No Kubernetes complexity, and no surface area you did not ask for.

**Non-opinionated and flexible.** Trellis provides the necessary building blocks without prescribing how you use them. Reverse proxies, for instance, are ordinary jobs rather than a special first-class service or ingress resource. Trellis does not enforce any organizational scheme; namespaces, teams, and projects are yours to arrange however makes sense. Operators can run Trellis as-is, or build their own frontends and abstractions on top for their specific use case.

**Declarative, with open-ended delivery.** Jobs are defined as YAML manifests and submitted through the API or CLI. You can apply them directly from your terminal, drive them from a CI/CD pipeline, build a custom UI on top of the API, or integrate with any tooling that can run a command or make an HTTP request. Trellis accepts manifests; the workflow that generates and submits them is entirely yours.

**Easy to use.** The tension between "flexible building blocks" and "easy to use" is addressed through thorough documentation and first-party examples. Trellis favors clear documentation over opinionated defaults that hide what is actually happening.

**Open-source.** Trellis is fully open-source. Read it, modify it, and run it wherever you like.

## Capabilities

- YAML job validation and revisioned job submission
- Node registration, heartbeats, draining, and balanced placement
- Allocation lifecycle management, health checks, restart handling, and filterable runtime queries
- Rolling and recreate update strategies
- Container resource limits, dynamic host ports, and persistent local volumes
- Built-in DNS discovery for healthy job allocations and optional WireGuard namespace networking
- Namespace-scoped, write-only secrets with encrypted persistence and memory-backed delivery
- A CLI and a read-only Next.js operations dashboard

## Documentation

### User documentation

| Guide | Use it to |
| --- | --- |
| [Getting started](docs/getting-started.md) | Install Trellis and deploy your first workloads |
| [Core concepts](docs/concepts.md) | Understand nodes, jobs, allocations, namespaces, and leadership |
| [Job manifest reference](docs/job-manifest.md) | Configure task groups, resources, ports, volumes, health checks, and update strategies |
| [Operations guide](docs/operations.md) | Deploy persistently, operate nodes, and enable WireGuard |
| [Allocation lifecycle](docs/allocation-lifecycle.md) | Understand durable execution, retries, fencing, recovery, and loss semantics |
| [Volume semantics](docs/volumes.md) | Configure named host-volume availability and understand the storage responsibility boundary |
| [Secrets design](docs/secrets.md) | Review the write-only secret model, encryption, delivery, and rotation semantics |
| [Dashboard guide](ui/README.md) | Configure, develop, and deploy the web UI |

### Developer documentation

| Guide | Use it to |
| --- | --- |
| [Vagrant demo](docs/development/vagrant.md) | Try a multi-node cluster locally with Vagrant |
| [Contributing](docs/development/contributing.md) | Build from source and run development checks |

## Quick start

The setup script downloads the latest release binaries, configures a systemd
service, and generates a cluster token. It supports Linux x64 and requires
root access and a running containerd instance.

```sh
curl -fsSL https://raw.githubusercontent.com/clofour/trellis-experimental/main/scripts/setup.sh | sudo bash
```

Or clone the repository and run the script directly:

```sh
git clone https://github.com/clofour/trellis-experimental.git
sudo ./trellis-experimental/scripts/setup.sh
```

The script interactively asks whether to enable WireGuard networking, whether to install the web dashboard, and whether this node should join an existing cluster.

See the [getting-started guide](docs/getting-started.md) for a walkthrough of deploying your first workloads.
