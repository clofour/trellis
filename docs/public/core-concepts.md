# Core concepts

Start with the [Trellis user model](user-model.md) for the vocabulary shared by manifests, the CLI, dashboard, and examples. This page explains how those user-facing concepts behave.

## Cluster and nodes

A **cluster** is one Trellis deployment operated as a unit. Each machine in the cluster is a **node** running `trellis`. Nodes report capacity, labels, advertised host volumes, runtime state, and health. From an operator's perspective a node is healthy, unhealthy, or draining.

Trellis uses Raft internally to replicate desired state and elect a control-plane leader. Leadership is an implementation detail for normal workload workflows; see the developer documentation when operating or debugging the consensus layer itself.

## Namespaces and jobs

A **namespace** is the tenant, authorization, discovery, and workload-isolation boundary for jobs, allocations, secrets, and namespace tokens.

A **job** is named desired state inside a namespace. Humans define a job with a YAML **job manifest**. Applying a manifest creates the job or advances its **revision** when desired state changes.

## Task groups, tasks, and allocations

A **task group** is the unit of placement, scaling, restart policy, and update strategy. `count` is the desired number of group replicas. A task group contains one or more **tasks**, each describing a container and its own network attachment.

Trellis creates runtime **allocations** to satisfy desired task-group capacity. Users normally inspect the job first and drill into allocations when they need placement, lifecycle, health, retry, port, event, or log details.

## Desired state versus runtime state

Keep these concepts separate when reading any Trellis interface:

- The job manifest and revision are **desired state**.
- Allocation **lifecycle** is execution state: `placed`, `starting`, `running`, `stopping`, `stopped`, `failed`, or `lost`.
- Allocation **health** is readiness/health state: `unknown`, `healthy`, or `unhealthy`.

An allocation can therefore be `running` and `unhealthy`. Older persisted state may still contain legacy status values for compatibility, but lifecycle and health are the canonical model.

## Scheduling

The scheduler considers only healthy, non-draining nodes. It filters on `os`, `arch`, custom label constraints, advertised host volumes, CPU millicores, and memory bytes. It then uses deterministic best-fit placement with replica spreading as a tie-breaker. Resource values of zero on a node mean capacity is not enforced for that dimension.

## Reconciliation and failure handling

Trellis continuously compares desired jobs with runtime allocations and converges the cluster toward desired state. Start and stop operations are idempotent; failed control operations are retried with bounded exponential backoff and jitter. Allocation generations and leadership fencing prevent stale work from overwriting newer decisions.

Those generation/fencing details are useful diagnostics, but they are not separate workload resources users need to model in manifests.

## Networking and discovery

Each task selects its attachment through `networking.mode`. The default is an isolated container network with no external routes; `host` joins the node network directly, and `wireguard` joins the namespace WireGuard mesh. Host-port reservations belong under that task's `networking` block and are valid only in host mode. Healthy allocation endpoints enter the service catalog. Discovery supports namespace and label filtering, and Trellis DNS resolves names shaped like `group.job.namespace.trellis`.

## Persistence and secrets

Unnamed volumes are allocation-local directories beneath the node data directory. A `host_volume` requires a node to advertise that volume name and constrains placement to compatible nodes.

**Secrets** are namespace-scoped named values referenced by job manifests without embedding their plaintext in YAML. Trellis encrypts stored secret records and injects values into allocations as environment variables or files below `/run/trellis-secrets/`. Updating a secret does not mutate already-running allocations.

## Updates

`recreate` stops outdated allocations before replacements. `rolling` starts bounded replacements and removes draining old allocations after replacements are healthy.

Blue/green and canary releases are deployment patterns composed from ordinary jobs, task groups, labels, health checks, and routing. They are not additional Trellis resource types. See the [Cookbook](cookbook.md) and [examples](../../examples/README.md).

[Documentation index](../README.md) · [Previous: User model](user-model.md) · [Next: Job manifest reference](job-specification.md)
