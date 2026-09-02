# Learning path

Complete [Getting Started](getting-started.md) first. It establishes the only golden path: install → connect → deploy → inspect → update → view logs → remove. This page then introduces one layer of Trellis at a time instead of beginning with an application architecture.

## The sequence

| Stage | Learn | Run |
|---|---|---|
| 1. Minimal workload | Job → task group → task → allocation; revisions and logs | [`examples/hello`](../../examples/hello/) |
| 2. Healthy service | Multiple replicas, host networking, fixed port reservations, HTTP health, rolling updates | [`examples/web-service`](../../examples/web-service/) |
| 3. Runtime configuration | Namespace-scoped environment/file secrets and rotation | [`examples/secrets`](../../examples/secrets/) |
| 4. Persistence | Allocation-local storage, advertised host volumes, constraints, backup responsibility | [`examples/volumes`](../../examples/volumes/) |
| 5. Colocated tasks | Sidecars and the consequences of shared placement/scaling/lifecycle | [`examples/sidecar`](../../examples/sidecar/) |
| 6. Namespace networking | Isolated, host, and WireGuard task networking; service discovery | [Networking below](#6-namespace-networking-and-discovery) |
| 7. In-cluster automation | Namespace/cluster scope and read/write API access | [`examples/api-access`](../../examples/api-access/) |
| 8. Release architecture | Rolling, blue/green, and weighted canary composition | [`examples/deployment-strategies`](../../examples/deployment-strategies/) |
| 9. Stateful compositions | Coupled development stacks, local-volume caveats, application-native HA | [`examples/wordpress`](../../examples/wordpress/), then [`examples/patroni`](../../examples/patroni/) |

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

## 2. Health, networking, scaling, and updates

The `web-service` example adds several related operational ideas together:

- `count: 2` asks for two task-group allocations;
- `networking.mode: host` opts the task into the node network;
- `networking.ports` reserves the exact `port` the process listens on; Trellis does not translate ports;
- because both replicas reserve port 8080, they need two different nodes;
- the HTTP health check decides when a running task is ready;
- rolling replacement waits for healthy new capacity before removing old capacity, so a fixed-port service needs another compatible node during overlap.

Apply it with `--wait`, change the image, run `jobs diff`, then apply again. Use `jobs diagnose` when health or placement blocks progress. This is the baseline stateless-service pattern; later examples compose it rather than replacing it with new resource types.

## 3. Secrets

Create secret values separately, reference only their names in YAML, and decide whether each application needs an environment or file target. Trellis never reads plaintext values back. A rotated value reaches newly started allocations; it does not mutate a running process.

Follow [`examples/secrets`](../../examples/secrets/) before using secrets in a larger stack. Preserve and back up the node secrets-encryption key separately from Trellis desired-state backups.

## 4. Volumes

Start with allocation-managed scratch data, then learn advertised `host_volume` placement. A host-volume name tells the scheduler which nodes can satisfy a mount; it does not replicate, snapshot, or transport bytes.

The [`volumes`](../../examples/volumes/) example deliberately requires operator preparation. Complete its node-label, directory-ownership, backup, and restore notes before adapting it to real data.

## 5. Sidecars and task groups

A task group is more than YAML nesting: every task in it is placed, scaled, updated, and drained together. The [`sidecar`](../../examples/sidecar/) example uses this coupling intentionally for nginx and its metrics exporter.

If two containers should scale or fail independently, use separate task groups or jobs. If a helper needs to observe many allocations rather than only its colocated application, continue to the API-access/controller stage instead.

## 6. Namespace networking and discovery

Networking is selected per task:

| `networking.mode` | Meaning | When to use it |
|---|---|---|
| omitted / empty | Private container namespace without external routes | Jobs that need no network, or custom runtime setup |
| `host` | Join the node network; may reserve ports used directly by the process | Directly reachable services and simple local communication |
| `wireguard` | Join the configured namespace WireGuard mesh | Cross-node namespace communication when every node has WireGuard/runsc configured |

Host port declarations are valid only with `mode: host`:

```yaml
networking:
  mode: host
  ports:
    - port: 8080
```

There is only one port because host networking has no Trellis NAT or translation layer. The reservation prevents another Trellis task from claiming the same node port; the process must bind it itself.

WireGuard tasks do not declare host ports. Check them from inside the container with a script health check when needed:

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

## 7. In-cluster API access

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

## 8. Release patterns

Trellis directly implements `recreate` and `rolling`. Blue/green and canary are compositions of separate jobs plus external routing state. Read [`deployment-strategies`](../../examples/deployment-strategies/) only after running the simpler rolling update in `web-service`. When these patterns use a shared fixed host port, every simultaneously running allocation needs a node where that port is free; Trellis does not hide this capacity requirement behind a port-forwarding layer.

## 9. Stateful and HA patterns

The WordPress example is a development composition, not a production topology. The Patroni example is an architecture skeleton, not a database service. At this stage you should be able to identify which responsibilities Trellis supplies—placement, lifecycle, health observation, secret delivery, network attachment—and which remain application/operator responsibilities.

## Reference and operations

Use the learning path to acquire the model; use these pages afterward:

- [Job manifest reference](job-specification.md) for exact fields and validation.
- [CLI workflows](cli.md) for contexts, planning, diagnosis, logging, and automation.
- [Operations](operations.md) for node maintenance, backups, TLS, and recovery.
- [Cookbook](cookbook.md) for architecture outcomes and tradeoffs.

[Documentation index](../README.md) · [Previous: Getting Started](getting-started.md) · [Next: User model](user-model.md)
