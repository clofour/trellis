# Getting started

This guide builds a local Trellis node and submits a first workload. It assumes
Consul and containerd are already installed and running on a Linux host. See
the [operations guide](operations.md) for a persistent systemd deployment.

## 1. Build the binaries

Trellis currently requires Go 1.26.4 or later.

```sh
git clone https://github.com/clofour/trellis.git
cd trellis/orchestrator
mkdir -p bin
go build -o bin/trellis-node ./cmd/trellis-node
go build -o bin/trellis ./cmd/trellis
```

Confirm that the external services are reachable:

```sh
consul members
sudo ctr version
```

## 2. Start a node

Generate a cluster token once. Every node, CLI invocation, and dashboard
server in a cluster must use this same value.

```sh
export TRELLIS_TOKEN="$(head -c 32 /dev/urandom | base64)"
```

Start the daemon:

```sh
./bin/trellis-node \
  --data-dir ./data \
  --agent-advertise node-1.example:8127 \
  --server-advertise node-1.example:8128 \
  --cluster-token "$TRELLIS_TOKEN"
```

Use addresses that other cluster nodes can resolve and reach. On the first
node, Trellis initializes cluster metadata from the token. Later nodes verify
that token before joining. The log message `leadership acquired` indicates
that this node owns the current Consul leadership session.

## 3. Write a job manifest

Save the following as `trellis.yaml`:

```yaml
namespace: examples
name: hello
task_groups:
  - name: web
    count: 2
    runtime: runc
    tasks:
      - name: server
        image: docker.io/library/nginx:alpine
        resources:
          cpu: 250
          memory: 134217728
        ports:
          - host_port: 0
            container_port: 80
        health_check:
          type: http
          port: 80
          path: /
```

`host_port: 0` asks Trellis to allocate a free host port. CPU values are in
millicores and memory values are in bytes. See the
[manifest reference](job-manifest.md) for all supported fields.

## 4. Submit and inspect the job

Run these commands from `orchestrator/` in a second terminal:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file trellis.yaml

./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" --namespace examples jobs status hello

./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" nodes list
```

Job queries require `--namespace`; `jobs apply` instead reads the namespace
from the manifest. Copy an allocation ID from `jobs status` to stream logs:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" --namespace examples \
  jobs logs --tail 100 --follow ALLOCATION_ID
```

## 5. Update or remove the job

Edit `trellis.yaml` and run `jobs apply` again to submit a new revision. To
remove the job and its allocations:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" --namespace examples jobs destroy hello
```

## Vagrant demo

The repository also contains provisioning scripts for a multi-machine demo.
Install Vagrant and a compatible provider, then run:

```sh
cd orchestrator
vagrant up
```

The demo provisions Consul, containerd, and `trellis-node`. It is intended for
evaluation rather than production deployment.

## Troubleshooting

- **`--cluster-token is required`:** pass the same non-empty token to the node
  and all clients.
- **Cannot connect to `localhost:8128`:** verify that a node currently owns the
  leadership session and that its leader API address is reachable.
- **Containerd initialization fails:** check the socket path and pass
  `--containerd-sock` if it is not `/run/containerd/containerd.sock`.
- **A job remains pending:** use `nodes list` to check node readiness and
  capacity; requested CPU and memory must fit on a ready node.
