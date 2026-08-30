# Getting started

This guide builds a local Trellis node, deploys a multi-job application, and
demonstrates DNS discovery and allocation queries. It assumes containerd is
already installed and running on a Linux host. See the
[operations guide](operations.md) for a persistent systemd deployment.

## 1. Build the binaries

Trellis requires Go 1.26.4 or later.

```sh
git clone https://github.com/clofour/trellis.git
cd trellis/orchestrator
mkdir -p bin
go build -o bin/trellis-node ./cmd/trellis-node
go build -o bin/trellis ./cmd/trellis
```

Confirm that containerd is reachable:

```sh
sudo ctr version
```

## 2. Start a node

Generate a cluster token once. Every node, CLI invocation, and dashboard
server in a cluster must use the same value.

```sh
export TRELLIS_TOKEN="$(head -c 32 /dev/urandom | base64)"
```

Start the daemon:

```sh
sudo ./bin/trellis-node \
  --data-dir ./data \
  --cluster-token "$TRELLIS_TOKEN"
```

Use `--agent-advertise` and `--server-advertise` with addresses that other
cluster nodes can reach; on a single-node setup the defaults are fine. The
first node bootstraps a new cluster. The log message `leadership acquired`
confirms that this node is the active leader.

## 3. Deploy a web server

Save the following as `hello.yaml`:

```yaml
namespace: examples
name: hello
task_groups:
  - name: web
    count: 2
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

`host_port: 0` tells Trellis to allocate a free host port for each replica.
CPU values are in millicores and memory values are in bytes. See the
[manifest reference](job-manifest.md) for all supported fields.

Submit the job and check its status:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file hello.yaml

./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" --namespace examples jobs status hello
```

You should see two allocations transition to `healthy`. Copy an allocation ID
from `jobs status` to stream its logs:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" --namespace examples \
  jobs logs --tail 100 --follow ALLOCATION_ID
```

## 4. Add a backend and use DNS discovery

Trellis includes built-in DNS so that containers can find jobs by name. Any
container can resolve `<job>.<namespace>.trellis` to the addresses of that
job's healthy allocations — no separate service object or registry is
required.

Save the following as `backend.yaml`:

```yaml
namespace: examples
name: backend
task_groups:
  - name: api
    count: 2
    labels:
      trellis.role: backend
      trellis.expose: "true"
    tasks:
      - name: api
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

Deploy it:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file backend.yaml
```

Once the backend is healthy, the `hello` containers can already resolve it at
`backend.examples.trellis`. To verify, exec into a running allocation and
resolve the name:

```sh
# Find a hello allocation ID from jobs status, then:
sudo ctr -n trellis tasks exec --exec-id test ALLOCATION_ID \
  nslookup backend.examples.trellis
```

The DNS response includes the host addresses of all healthy `backend`
replicas. When you scale the backend, the DNS results update automatically.

## 5. Query allocations by job or label

Allocations are the public runtime resource for inspecting where work is
running. The allocations API includes task-group labels, runtime status, node
address, and allocated ports.

Scope the request to the `examples` namespace and filter by job:

```sh
curl -s \
  -H "Authorization: Bearer $TRELLIS_TOKEN" \
  -H "X-Trellis-Namespace: examples" \
  "http://localhost:8128/v1/allocations?job=backend" \
  | python3 -m json.tool
```

Or filter by a label key/value pair:

```sh
curl -s \
  -H "Authorization: Bearer $TRELLIS_TOKEN" \
  -H "X-Trellis-Namespace: examples" \
  "http://localhost:8128/v1/allocations?label=trellis.expose:true" \
  | python3 -m json.tool
```

A key-only filter such as `?label=trellis.role` matches allocations whose task
group carries that label regardless of its value. Job and label filters may be
combined.

This is useful for dynamic configuration and tooling without turning the
internal DNS service catalog into a user-facing resource. See the
[reverse-proxy example](../examples/reverse-proxy/) for one such pattern.

## 6. Scale and update

Edit a manifest's `count` and reapply to scale up or down. Trellis reconciles
the running allocations to match: new replicas are placed on nodes with
available capacity, and surplus replicas are stopped.

```sh
# Scale the backend to 3 replicas
sed -i 's/count: 2/count: 3/' backend.yaml
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" jobs apply --file backend.yaml
```

To remove a job and all its allocations:

```sh
./bin/trellis --server-addr localhost:8128 \
  --cluster-token "$TRELLIS_TOKEN" --namespace examples jobs destroy backend
```

## Vagrant demo

The repository includes provisioning scripts for a multi-node demo. Install
Vagrant and a compatible provider, then run:

```sh
cd orchestrator
vagrant up
```

The demo provisions containerd and `trellis-node` on three VMs: one
bootstraps the cluster and two join it. It is intended for evaluation rather
than production deployment.

## Troubleshooting

- **`--cluster-token is required`:** pass the same non-empty token to the node
  and all clients.
- **Cannot connect to `localhost:8128`:** verify that a node has acquired
  leadership and that its leader API address is reachable.
- **Containerd initialization fails:** check the socket path and pass
  `--containerd-sock` if it is not `/run/containerd/containerd.sock`.
- **A job remains pending:** use `nodes list` to check node readiness and
  capacity; requested CPU and memory must fit on a ready node.
- **DNS resolution not working:** ensure `trellis-node` is running with the
  default `--dns-listen :8053`; containers receive the resolver address
  automatically.
