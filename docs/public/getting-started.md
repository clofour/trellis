# Getting Started

This walkthrough creates a single-node development cluster. If Trellis terminology is new, read the [user model](user-model.md) first: you apply YAML job manifests as desired state, then inspect the allocations Trellis creates at runtime.

Trellis needs Linux, root privileges, containerd, and the `ctr` client. Go 1.26.4 is required to build the orchestrator; the dashboard additionally needs Node.js and npm.

## Build

```sh
cd orchestrator
go build -o bin/trellis ./cmd/trellis
go build -o bin/trellis-node ./cmd/trellis-node
```

## Start a node

Choose a long random cluster token. The injected runtime is test-only; a useful cluster uses containerd.

```sh
sudo ./bin/trellis-node \
  --data-dir /var/lib/trellis/data \
  --cluster demo \
  --cluster-token "$TRELLIS_TOKEN" \
  --containerd-sock /run/containerd/containerd.sock
```

The defaults expose the agent API on `8127`, control-plane API on `8128`, Raft on `8129`, Trellis DNS on UDP `8053`, and WireGuard on UDP `51820`. For anything beyond local experimentation, set advertised addresses explicitly and configure `--ca-cert`, `--ca-key`, `--cert`, and `--key`.

## Configure the CLI

For interactive use, save the cluster connection as a named context:

```sh
export TRELLIS_TOKEN='replace-me'
./bin/trellis \
  --server-addr 127.0.0.1:8128 \
  --namespace default \
  context save local --use

./bin/trellis context current
./bin/trellis nodes list
```

Contexts store the address, namespace, token, and TLS settings in the user config, which Trellis protects with mode `0600`. Explicit flags and environment variables remain available for scripts and override context values:

```sh
export TRELLIS_ADDR=127.0.0.1:8128
export TRELLIS_TOKEN='replace-me'
export TRELLIS_NAMESPACE=default
./bin/trellis nodes list
```

Existing flat `~/.config/trellis/config.yaml` files also remain supported. See [CLI workflows](cli.md) for context precedence and management commands.

## Run a workload

Validate and preview the manifest before applying it:

```sh
./bin/trellis jobs validate --file ../examples/sidecar/trellis.yaml
./bin/trellis jobs diff --file ../examples/sidecar/trellis.yaml
```

Apply desired state and wait for healthy capacity:

```sh
./bin/trellis jobs apply --file ../examples/sidecar/trellis.yaml --wait
./bin/trellis jobs list
./bin/trellis jobs status sidecar-demo
```

An apply is declarative: it creates the job or advances its revision. Read the current logs without first copying an allocation UUID:

```sh
./bin/trellis jobs logs sidecar-demo --tail 100
```

For a live stream from one replica, use the short allocation prefix shown by `jobs status`:

```sh
./bin/trellis jobs logs sidecar-demo --allocation ALLOC_PREFIX --follow
```

If the job does not become healthy, run `./bin/trellis jobs diagnose sidecar-demo` to surface lifecycle, health, failure reason, retry timing, and the next useful log command.

Delete the job and its desired state with `./bin/trellis jobs delete sidecar-demo`. Trellis stops allocations that are no longer desired. The older `jobs destroy` spelling remains as a compatibility alias.

## Add nodes

Start the first node normally, then start later nodes with `--join HOST:8128`. Give every node the same cluster name/token and routable agent, server, and Raft advertised addresses. Confirm membership with `trellis nodes list`.

Node maintenance commands accept the host/address shown in `nodes list` as well as UUIDs, so `trellis nodes drain worker-2` is sufficient when the hostname is unique.

## Next steps

Read [CLI workflows](cli.md), [Core concepts](core-concepts.md), the complete [job manifest reference](job-specification.md), and the [Cookbook](cookbook.md).
