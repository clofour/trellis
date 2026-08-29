# Trellis

Trellis is an experimental container scheduler built on containerd and Consul.
Every machine runs the same `trellis-node` daemon: Consul elects one node to
serve the control-plane API and reconcile jobs, while every node continues to
run allocations and can participate in the next election.

The current MVP includes:

- YAML job validation and revisioned job submission
- node registration, heartbeats, draining, and balanced placement
- allocation lifecycle management, health checks, and restart handling
- container resource limits, dynamic host ports, and persistent local volumes
- Consul service registration and optional WireGuard namespace networking
- a CLI and a read-only Next.js operations dashboard

> [!NOTE]
> Trellis is experimental. Evaluate its security, failure modes, and upgrade
> behavior before using it for critical workloads.

## Documentation

| Guide | Use it to |
| --- | --- |
| [Getting started](docs/getting-started.md) | Build Trellis and run a first job |
| [Core concepts](docs/concepts.md) | Understand nodes, jobs, allocations, namespaces, and leadership |
| [Job manifest reference](docs/job-manifest.md) | Configure task groups, resources, ports, volumes, and health checks |
| [Operations guide](docs/operations.md) | Deploy services, operate nodes, and enable WireGuard |
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

If you prefer to build from source, install Go 1.26.4 or later and a
running containerd instance, then:

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
