# Contributing

## Repository layout

```text
.
├── docs/          Operator and user documentation
├── examples/      Example job manifests and deployment patterns
├── orchestrator/  Go node daemon, CLI, scheduler, and Vagrant demo
├── scripts/       Installation and setup scripts
└── ui/            Next.js operations dashboard
```

## Building from source

Install Go 1.26.4 or later and a running containerd instance, then build
the node daemon and CLI:

```sh
cd orchestrator
go build -o bin/trellis-node ./cmd/trellis-node
go build -o bin/trellis ./cmd/trellis
```

## Development checks

Run these before submitting a pull request:

```sh
(cd orchestrator && go test ./...)
(cd orchestrator && go vet ./...)
(cd ui && npm ci && npm run lint && npm run build)
```

## Running locally

Start a node with a temporary token:

```sh
export TRELLIS_TOKEN="$(head -c 32 /dev/urandom | base64)"

sudo ./orchestrator/bin/trellis-node \
  --data-dir ./data \
  --cluster-token "$TRELLIS_TOKEN"
```

The leader API listens on `:8128` and each node's agent API on `:8127`.
Both require the cluster token as a bearer token.

For a multi-node local setup, see the [Vagrant demo](vagrant.md).
