# Trellis

Trellis is a container scheduler built on containerd. It sits in the space between rolling your own deployment scripts and adopting Kubernetes — a real orchestrator for container workloads, without the operational complexity.

Every project that ships software ends up rebuilding the same infrastructure: workload placement, health checks, rolling updates, port allocation. Tools like Coolify improve the developer experience, but at their core they are not orchestrators. Kubernetes is a full orchestrator, but it brings significant complexity that many workloads simply do not need. Trellis is closer to Nomad in spirit: a lightweight, focused scheduler you can understand and operate yourself.

Every machine runs the same `trellis-node` daemon. Raft consensus elects one node to serve the control-plane API and reconcile jobs, while every node continues to run allocations and participate in the next election.

> [!NOTE]
> Trellis is experimental. Evaluate its security, failure modes, and upgrade
> behavior before using it for critical workloads.

## Design principles

**Modular and extensible, but focused.** Trellis exposes clean primitives you can build on. The core repository stays lean — only the essentials live here. If you need something specific to your environment, you can extend Trellis yourself rather than waiting on a plugin ecosystem. No Kubernetes complexity, and no surface area you did not ask for.

**Non-opinionated and flexible.** Trellis provides the necessary building blocks without prescribing how you use them. Reverse proxies, for instance, are ordinary jobs rather than a special first-class service or ingress resource. Trellis does not enforce any organizational scheme; namespaces, teams, and projects are yours to arrange however makes sense. Operators can run Trellis as-is, or build their own frontends and abstractions on top for their specific use case.

**Declarative, and optionally GitOps.** Jobs are defined as YAML manifests submitted through the API or CLI. Trellis supports a GitOps workflow, but does not require one — if your team prefers to apply manifests directly, that works just as well. The choice is yours.

**Easy to use.** The tension between "flexible building blocks" and "easy to use" is addressed through thorough documentation and first-party examples. Trellis favors clear documentation over opinionated defaults that hide what is actually happening.

**Open-source.** Trellis is fully open-source. Read it, modify it, and run it wherever you like.

## Capabilities

- YAML job validation and revisioned job submission
- Node registration, heartbeats, draining, and balanced placement
- Allocation lifecycle management, health checks, restart handling, and filterable runtime queries
- Container resource limits, dynamic host ports, and persistent local volumes
- Built-in DNS discovery for healthy job allocations and optional WireGuard namespace networking
- A CLI and a read-only Next.js operations dashboard

## Documentation

| Guide | Use it to |
| --- | --- |
| [Getting started](docs/getting-started.md) | Build Trellis and run a first job |
| [Core concepts](docs/concepts.md) | Understand nodes, jobs, allocations, namespaces, and leadership |
| [Job manifest reference](docs/job-manifest.md) | Configure task groups, resources, ports, volumes, and health checks |
| [Operations guide](docs/operations.md) | Deploy workloads, operate nodes, and enable WireGuard |
| [Dashboard guide](ui/README.md) | Configure, develop, build, and deploy the web UI |

## Quick start

### Install from release (recommended)

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

The script will interactively ask whether to enable WireGuard networking
and whether this node should join an existing cluster.

### Build from source

Install Go 1.26.4 or later and a running containerd instance, then:

```sh
cd orchestrator
go build -o bin/trellis-node ./cmd/trellis-node
go build -o bin/trellis ./cmd/trellis
```

Start a node (advertised addresses must be reachable by other nodes):

```sh
export TRELLIS_TOKEN="$(head -c 32 /dev/urandom | base64)"

./bin/trellis-node \
  --data-dir ./data \
  --agent-advertise node-1.example:8127 \
  --server-advertise node-1.example:8128 \
  --cluster-token "$TRELLIS_TOKEN"
```

In another terminal, submit a manifest and inspect the job:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file trellis.yaml

./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" --namespace examples jobs status hello
```

A complete example manifest and an explanation of each command are in the
[getting-started guide](docs/getting-started.md).

## Repository layout

```text
.
├── docs/          Operator and user documentation
├── orchestrator/  Go node daemon, CLI, scheduler, and Vagrant demo
└── ui/            Next.js operations dashboard
```

## Development checks

```sh
(cd orchestrator && go test ./...)
(cd orchestrator && go vet ./...)
(cd ui && npm ci && npm run lint && npm run build)
```

The elected leader API listens on `8128` by default. Each node's agent API
listens on `8127`. Both APIs require the cluster token as a bearer token.
