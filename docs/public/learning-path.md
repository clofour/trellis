# Learning path

Complete [Getting Started](getting-started.md) first. It establishes the only golden path: install → connect → deploy → inspect → update → view logs → remove. This page then introduces one layer of Trellis at a time instead of beginning with an application architecture.

## The sequence

| Stage | Learn | Run |
|---|---|---|
| 1. Minimal workload | Job → task group → task → allocation; revisions and logs | [`examples/hello`](../../examples/hello/) |
| 2. Healthy service | Host networking, one fixed port reservation, and HTTP health | [`examples/web-service`](../../examples/web-service/) |
| 3. Replicas and placement | Multiple replicas and the scheduling consequences of fixed host ports | [`examples/replicated-service`](../../examples/replicated-service/) |
| 4. Rolling updates | Healthy overlap, `max_parallel`, and temporary capacity requirements | [`examples/rolling-update`](../../examples/rolling-update/) |
| 5. Runtime configuration | Namespace-scoped environment/file secrets and rotation | [`examples/secrets`](../../examples/secrets/) |
| 6. Persistence | Allocation-local storage, advertised host volumes, constraints, backup responsibility | [`examples/volumes`](../../examples/volumes/) |
| 7. Colocated tasks | Sidecars and the consequences of shared placement/scaling/lifecycle | [`examples/sidecar`](../../examples/sidecar/) |
| 8. Namespace networking | Isolated, host, and WireGuard-backed namespace networking; service discovery | [Networking below](#8-namespace-networking-and-discovery) |
| 9. In-cluster automation | Namespace/cluster scope and read/write API access | [`examples/api-access`](../../examples/api-access/) |
| 10. Release architecture | Rolling, blue/green, and weighted canary composition | [`examples/deployment-strategies`](../../examples/deployment-strategies/) |
| 11. Stateful compositions | Coupled development stacks, local-volume caveats, application-native HA | [`examples/wordpress`](../../examples/wordpress/), then [`examples/patroni`](../../examples/patroni/) |

Do not skip directly to Patroni to learn basic Trellis. Patroni assumes you already understand every earlier layer and still requires a real DCS, replication, fencing, routing, and independent data backups.

## 1. Minimal workload

The `hello` example intentionally omits network exposure and application health settings. Learn the core loop first:

```sh
trellisctl jobs validate --file examples/hello/trellis.yaml
trellisctl jobs diff --file examples/hello/trellis.yaml
trellisctl jobs apply --file examples/hello/trellis.yaml --wait
trellisctl jobs status hello
trellisctl jobs logs hello
trellisctl jobs delete hello --wait
```

At this stage, understand that the manifest is desired state and the allocation is runtime state. Drill into allocation details only when status or logs require it.

## 2. Health and service networking

The `web-service` example keeps `count: 1` and adds only the pieces needed to make the tutorial application a reachable, application-aware service:

- `networking.mode: host` opts the task into the node network;
- `networking.ports` reserves the exact `port` the process listens on;
- the HTTP health check decides when the running task is ready.

Host networking has no Trellis NAT or port translation. The reservation prevents another Trellis task from claiming the same node port, and the process must bind that port itself.

Apply the example, reach the service at the selected node's port 8080, and use `jobs diagnose` if its health check blocks readiness. Do not add replicas yet; first make the one-allocation service model concrete.

## 3. Replicas and placement

The `replicated-service` example changes the healthy service from one desired allocation to two. Both replicas reserve port 8080, so they cannot share a node and require at least two compatible nodes.

This stage is about scheduling rather than rollout policy. Inspect both allocations with:

```sh
trellisctl jobs status replicated-service
trellisctl nodes list
trellisctl nodes status NODE
```

If only one compatible node exists, `jobs diagnose replicated-service` should make the placement failure visible. Understand why the second allocation cannot be placed before moving on to overlapping updates.

## 4. Rolling updates

The `rolling-update` example keeps the same two-replica service and adds:

```yaml
update:
  strategy: rolling
  max_parallel: 1
```

Change only the tutorial image from `v1` to `v2`, run `jobs diff`, then apply again. Trellis starts healthy replacement capacity before completing removal of the old revision.

The fixed host port makes the temporary-capacity cost visible: two old replicas already occupy port 8080 on two nodes, so the first replacement needs another compatible node with that port free. `max_parallel: 1` limits how much replacement capacity can be in flight at once. If placement or health blocks progress, use `jobs diagnose` rather than treating the rollout as an opaque failed command.

## 5. Secrets

Create secret values separately, reference only their names in YAML, and decide whether each application needs an environment or file target. Trellis never reads plaintext values back. A rotated value reaches newly started allocations; it does not mutate a running process.

Follow [`examples/secrets`](../../examples/secrets/) before using secrets in a larger stack. Preserve and back up the node secrets-encryption key separately from Trellis desired-state backups.

## 6. Volumes

Start with allocation-managed scratch data, then learn advertised `host_volume` placement. A host-volume name tells the scheduler which nodes can satisfy a mount; it does not replicate, snapshot, or transport bytes.

The [`volumes`](../../examples/volumes/) example deliberately requires operator preparation. Complete its node-label, directory-ownership, backup, and restore notes before adapting it to real data. `trellisctl nodes status NODE` shows the labels and advertised host-volume names that affect placement.

## 7. Sidecars and task groups

A task group is more than YAML nesting: every task in it is placed, scaled, updated, and drained together. The [`sidecar`](../../examples/sidecar/) example uses this coupling intentionally for nginx and its metrics exporter.

If two containers should scale or fail independently, use separate task groups or jobs. If a helper needs to observe many allocations rather than only its colocated application, continue to the API-access/controller stage instead.

## 8. Namespace networking and discovery

Networking is selected per task:

| `networking.mode` | Meaning | When to use it |
|---|---|---|
| omitted / empty | Private container namespace without external routes | Jobs that need no network, or custom runtime setup |
| `host` | Join the node network; may reserve ports used directly by the process | Directly reachable services and simple local communication |
| `wireguard` | Join the Trellis namespace network, currently implemented as a WireGuard mesh | Cross-node communication within the workload namespace when every node has WireGuard/runsc configured |

Host port declarations are valid only with `mode: host`:

```yaml
networking:
  mode: host
  ports:
    - port: 8080
```

There is only one port because host networking has no Trellis NAT or translation layer. The reservation prevents another Trellis task from claiming the same node port; the process must bind it itself.

WireGuard-networked tasks do not declare host ports. Check them from inside the container with a script health check when needed:

```yaml
runtime: runsc
tasks:
  - name: app
    image: registry.example/app:v1
    networking:
      mode: wireguard
    health_check:
      type: script
      command: [wget, -q, -O, /dev/null, http://127.0.0.1:8080/health]
```

Configure WireGuard/runsc on every participating node, open the configured UDP port between nodes, and verify the image contains the health-check command. Healthy allocations enter Trellis discovery; API-aware controllers can filter them by task-group labels. Treat discovery as runtime endpoint information, not application consensus.

## 9. In-cluster API access

API access has two dimensions: **scope** (`namespace` or `cluster`) and **access** (`read` or `write`). Prefer the narrowest pair that works.

A typical observer uses:

```yaml
api_access:
  scope: namespace
  access: read
```

Trellis gives every task in the group a bearer credential restricted to the job's own namespace, plus the API address, job namespace, and cluster CA when configured. The namespace cannot be redirected to another tenant.

Use `namespace/write` only for a namespace-local controller that actually mutates desired state. Use `cluster/read` for a trusted cluster-wide observer. Use `cluster/write` only for an operator workload that needs ordinary cluster mutations.

The bootstrap cluster credential is separate and more privileged. It is used for node registration, Raft membership, backup/restore, and minting scoped credentials, and Trellis never injects it into workloads.

Every task in an API-enabled group can read the injected token, so do not add untrusted sidecars. The [`api-access`](../../examples/api-access/) example intentionally uses `namespace/read` and explains TLS verification, authenticated requests, last-known-good behavior, and token hygiene.

## 10. Release patterns

Trellis directly implements `recreate` and `rolling`. The dedicated [`rolling-update`](../../examples/rolling-update/) lesson covers the built-in rolling primitive before this stage. Blue/green and canary are compositions of separate jobs plus external routing state.

Read [`deployment-strategies`](../../examples/deployment-strategies/) only after completing the rolling lesson. When these patterns use a shared fixed host port, every simultaneously running allocation needs a node where that port is free; Trellis does not hide this capacity requirement behind a port-forwarding layer.

## 11. Stateful and HA patterns

The WordPress example is a development composition, not a production topology. The Patroni example is an architecture skeleton, not a database service. At this stage you should be able to identify which responsibilities Trellis supplies—placement, lifecycle, health observation, secret delivery, network attachment—and which remain application/operator responsibilities.

## Reference and operations

Use the learning path to acquire the model; use these pages afterward:

- [Job manifest reference](job-specification.md) for exact fields and validation.
- [CLI workflows](cli.md) for contexts, planning, diagnosis, logging, and automation.
- [Operations](operations.md) for node maintenance, backups, TLS, and recovery.
- [Cookbook](cookbook.md) for architecture outcomes and tradeoffs.

[Documentation index](../README.md) · [Previous: Getting Started](getting-started.md) · [Next: User model](user-model.md)
