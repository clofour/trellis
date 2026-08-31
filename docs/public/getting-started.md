# Getting Started

This walkthrough creates a single-node development cluster. Trellis needs Linux, root privileges, containerd, and the `ctr` client. Go 1.26.4 is required to build the orchestrator; the dashboard additionally needs Node.js and npm.

## Build

```sh
cd orchestrator
go build -o bin/trellis ./cmd/trellis
go build -o bin/trellisctl ./cmd/trellisctl
```

## Start a node

Choose a long random cluster token. The injected runtime is test-only; a useful cluster uses containerd.

```sh
sudo ./bin/trellis \
  --data-dir /var/lib/trellis/data \
  --cluster demo \
  --cluster-token "$TRELLIS_TOKEN" \
  --containerd-sock /run/containerd/containerd.sock
```

The defaults expose the agent API on `8127`, control-plane API on `8128`, Raft on `8129`, Trellis DNS on UDP `8053`, and WireGuard on UDP `51820`. For anything beyond local experimentation, set advertised addresses explicitly and configure `--ca-cert`, `--ca-key`, `--cert`, and `--key`.

## Configure the CLI

Flags override environment variables, which override `~/.config/trellisctl/config.yaml` (or `$TRELLIS_CONFIG`).

```sh
export TRELLIS_ADDR=127.0.0.1:8128
export TRELLIS_TOKEN='replace-me'
export TRELLIS_NAMESPACE=default
./bin/trellisctl nodes list
```

Equivalent config:

```yaml
server_addr: 127.0.0.1:8128
cluster_token: replace-me
namespace: default
```

## Run a workload

```sh
./bin/trellisctl jobs apply --file ../examples/sidecar/trellis.yaml
./bin/trellisctl jobs list
./bin/trellisctl jobs status sidecar-demo
```

An apply is declarative: it creates the job or advances its revision. Inspect an allocation ID with `jobs status`, then stream output:

```sh
./bin/trellisctl jobs logs --follow ALLOCATION_ID
```

Remove the workload with `./bin/trellisctl jobs destroy sidecar-demo`.

## Add nodes

Start the first node normally, then start later nodes with `--join HOST:8128`. Give every node the same cluster name/token and routable agent, server, and Raft advertised addresses. Confirm membership with `trellisctl nodes list`.

## Next steps

Read [Core concepts](core-concepts.md), the complete [manifest reference](job-specification.md), and the [Cookbook](cookbook.md).
