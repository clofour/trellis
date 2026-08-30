# Vagrant demo

The repository includes a Vagrant configuration that provisions a
three-node Trellis cluster on your local machine. It is intended for
evaluation and development — trying multi-node scheduling, testing
failover, and exploring cluster behavior without cloud infrastructure.

The demo provisions the same workloads as the [getting-started guide](getting-started.md):
a simple nginx web server and a WordPress blog backed by MySQL. Once
`vagrant up` finishes you have a running cluster with example jobs already
deployed.

## Prerequisites

- [Vagrant](https://developer.hashicorp.com/vagrant/downloads)
- A Vagrant provider: Hyper-V (Windows), VirtualBox, or libvirt (Linux)
- The [vagrant-hostmanager](https://github.com/devopsgroup-io/vagrant-hostmanager) plugin:
  ```sh
  vagrant plugin install vagrant-hostmanager
  ```
- Go 1.26.4 or later (to build the binaries before provisioning)

## Build the binaries

The VMs share the `orchestrator/` directory from your host as `/vagrant`.
Build the binaries into `orchestrator/bin/` before running `vagrant up`:

```sh
cd orchestrator
go build -o bin/trellis-node ./cmd/trellis-node
go build -o bin/trellis ./cmd/trellis
```

## Start the cluster

```sh
cd orchestrator
vagrant up
```

Vagrant provisions three Debian 12 VMs:

| VM | Hostname | Role |
| --- | --- | --- |
| `control` | `control.trellis.local` | First node — bootstraps the cluster and becomes leader |
| `worker-1` | `worker-1.trellis.local` | Joins the cluster on `control:8128` |
| `worker-2` | `worker-2.trellis.local` | Joins the cluster on `control:8128` |

Each VM gets 2 CPU cores and 2 GB RAM. All three run the same
`trellis-node` process; the control node wins the initial Raft election
and serves the control-plane API.

After provisioning completes, the demo script on the control node deploys
the example workloads automatically.

## Explore the cluster

SSH into the control node and use the CLI:

```sh
vagrant ssh control
```

The cluster token is stored at `/vagrant/bin/token` (shared with your
host at `orchestrator/bin/token`). The `TRELLIS_TOKEN` and `TRELLIS_ADDR`
variables are pre-set in the shell:

```sh
# List nodes
trellis nodes list

# Check the nginx job
trellis --namespace examples jobs status hello

# Check the WordPress + MySQL stack
trellis --namespace blog jobs status mysql
trellis --namespace blog jobs status wordpress
```

You can also query the API directly from your host using the shared token:

```sh
TOKEN="$(cat orchestrator/bin/token)"
curl -s -H "Authorization: Bearer $TOKEN" \
  http://control.trellis.local:8128/v1/allocations | python3 -m json.tool
```

## What the demo deploys

### nginx web server

A two-replica nginx deployment in the `examples` namespace. Demonstrates:

- Dynamic port allocation (`host_port: 0`)
- HTTP health checks
- Placement across multiple nodes

Find the allocated ports and open the nginx welcome page:

```sh
trellis --namespace examples jobs status hello
# Open http://worker-1.trellis.local:<port>
```

### WordPress + MySQL

A WordPress blog backed by MySQL in the `blog` namespace. Demonstrates:

- **DNS discovery**: WordPress connects to MySQL at `mysql.blog.trellis`
  without any manual IP configuration
- **Persistent volumes**: MySQL data directory is a named volume that
  survives allocation replacement on the same node
- **Ordered deployment**: the MySQL allocation becomes healthy before
  WordPress starts accepting traffic

```sh
trellis --namespace blog jobs status mysql
trellis --namespace blog jobs status wordpress
```

Find the WordPress host port and open it in your browser to see the
installation wizard — already connected to the database.

## Simulating failures

### Leader failover

Stop the control node and watch the workers elect a new leader:

```sh
vagrant halt control
```

From `worker-1`, watch the Raft election:

```sh
vagrant ssh worker-1
sudo journalctl -u trellis-node -f | grep -E "election|leadership"
```

Once a new leader is elected, the API continues to respond on that
worker's address. Restart the control node and it rejoins as a follower:

```sh
vagrant up control
```

### Node drain

Drain `worker-1` to move its allocations to `worker-2`:

```sh
vagrant ssh control
trellis nodes drain <worker-1-node-id>
trellis --namespace examples jobs status hello
```

Allocations migrate where capacity is available. Un-drain to allow new
placements again:

```sh
trellis nodes undrain <worker-1-node-id>
```

## Teardown

```sh
cd orchestrator
vagrant destroy -f
```

This removes all three VMs. The shared `bin/` directory and its token file
remain on your host.
