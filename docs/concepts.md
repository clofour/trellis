# Core concepts

## Cluster and leadership

Every machine runs the same `trellis-node` process. Each process hosts an agent
API, registers its capacity and heartbeat, and runs local allocations. Raft
consensus elects one node as leader. Only that node serves the control-plane
API and reconciles desired jobs with actual allocations.

Leadership is held through the Raft log. If a leader becomes unreachable, the
remaining nodes elect a new leader. The former leader stops its leader API and
reconciliation loop when it detects it has lost leadership. Agents direct
registrations and updates to the current leader.

## Resource model

Trellis exposes orchestration primitives rather than product concepts such as
tenants, applications, or environments. An external control plane is expected
to model those concepts and perform end-user authentication and authorization.

### Namespace

A namespace is a generic isolation domain. It scopes job and allocation
identity, persistent storage, and network identity. A job is uniquely
identified by the pair `(namespace, name)`.

The `network.wireguard` field selects whether WireGuard implements the
namespace network. It does not create the namespace, disable its conceptual
isolation when false, or select the container runtime.

### Job

A job is the desired state submitted as a YAML manifest. Reapplying a job
updates its revision. A job contains one or more task groups.

### Task group

A task group defines colocated tasks, a replica count, and a runtime. Each
replica becomes one scheduling unit. All tasks in that replica run on the same
node, while different replicas may be placed on different nodes.

The group runtime is `runc` by default and may be set to `runsc`. Runtime choice
is independent of namespace and WireGuard configuration.

### Task and allocation

A task specifies an image, environment, resource limits, ports, volumes, and
an optional health check. Resource values are per task. The scheduler accounts
for those values across the task-group replica count.

An allocation records the placement and lifecycle of runnable work on a node.
Allocation IDs are used for log streaming.

### Node

A node advertises CPU capacity in millicores and memory capacity in bytes. The
scheduler places allocations only on ready nodes with sufficient available
capacity. Draining a node rejects new placements and moves existing
allocations when another healthy node has capacity.

## Ports and storage

A port mapping exposes a container port through the host. Set `host_port` to
zero for dynamic allocation or to a specific port from 1 through 65535.

Named volumes are local persistent directories scoped by namespace. Their
contents survive allocation replacement on the same node, but local storage is
not replicated automatically between nodes.

## Health and restart behavior

Tasks may use HTTP, TCP, or script health checks. The agent evaluates health,
reports it to the leader, and participates in restart handling. Desired,
running, and healthy counts are available through `jobs status` and the
dashboard.

## Service discovery

Every container receives automatic DNS-based service discovery. Each node runs
a built-in DNS resolver that resolves names of the form
`<job>.<namespace>.trellis` to the host addresses of healthy allocations for
that job. Containers' `/etc/resolv.conf` is configured to use this resolver
automatically.

For example, a backend container can reach a database job named `postgres` in
namespace `acme` at `postgres.acme.trellis`. When the database has multiple
healthy replicas, the DNS response includes all of their addresses.

The resolver polls the leader's service catalog and caches results locally on
each node, so DNS lookups are fast and do not depend on the leader being
reachable for every query.

### API access

Task groups with `api_access: true` receive `TRELLIS_TOKEN` and `TRELLIS_ADDR`
environment variables. The token is scoped to the job's namespace and allows
the container to query the control-plane API directly.

The `GET /v1/services` endpoint returns all healthy allocations in the
namespace, including their labels, host addresses, and port mappings. It
supports optional `?job=<name>` and `?label=<key>:<value>` query parameters
for filtering. This enables richer service-discovery patterns beyond DNS, such
as building dynamic reverse-proxy configurations.

Labels on task groups are arbitrary key-value metadata carried through to the
services API. They are the recommended mechanism for building conventions such
as reverse-proxy routing rules. See the [reverse proxy example](../examples/reverse-proxy/)
for a complete walkthrough.

## Network model

With WireGuard enabled, Trellis generates and persists each node's WireGuard
identity, derives non-overlapping node subnets from the cluster pool, and
publishes public keys and endpoints during registration. The leader creates
peer plans; agents create namespace bridges and interfaces, routes, and
forwarding guards.

Network setup fails closed: an isolated allocation is not started if setup
fails, and a deterministic subnet collision does not replace an existing
namespace route. See [WireGuard networking](operations.md#wireguard-networking-optional)
for host requirements and configuration.
