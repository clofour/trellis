# Core concepts

## Cluster and nodes

Every `trellis-node` runs an agent and a control-plane server. Servers replicate desired state with Raft and elect one leader; agents reconcile local containerd state. Nodes report CPU, memory, labels, host-volume names, allocations, and heartbeats. A node may be healthy, unhealthy, or draining.

## Namespaces and jobs

A namespace is the tenant boundary used for jobs, allocations, discovery, secrets, and namespace tokens. A job is a named desired-state document within a namespace. Applying changed content increments its revision.

## Task groups, tasks, and allocations

A task group is the unit of placement, scaling, networking, restart policy, and update strategy. `count` is the desired number of group replicas. All tasks in one group are colocated in each allocation, which makes a second task a natural sidecar. An allocation records the chosen node, generation, job revision, lifecycle phase, health, retry information, ports, and diagnostics.

## Scheduling

The scheduler considers only healthy, non-draining nodes. It filters on `os`, `arch`, custom label constraints, advertised host volumes, CPU millicores, and memory bytes. It then uses deterministic best-fit placement with replica spreading as a tie-breaker. Resource values of zero on a node mean capacity is not enforced for that dimension.

## Reconciliation and lifecycle

The leader continuously compares jobs with allocations and issues idempotent start/stop operations to agents. Allocations move through `pending`, `placed`, `starting`, `running`, `stopping`, and terminal `stopped`, `failed`, or `lost` phases. Health is separate: `unknown`, `healthy`, or `unhealthy`. Operations carry a leadership epoch and allocation generation so stale leaders or stale changes cannot overwrite newer work.

## Networking and discovery

The default is isolated container networking; `network_mode: host` opts a group into the host network. A job can request automatic namespace WireGuard networking. Healthy allocation endpoints enter the service catalog. Discovery supports namespace and label filtering, and Trellis DNS resolves names shaped like `group.job.namespace.trellis`.

## Persistence and secrets

Unnamed volumes are allocation-local directories beneath the node data directory. A `host_volume` requires a node to advertise that volume name and ties the allocation to compatible local storage. Secrets are namespace-scoped, encrypted before entering replicated state, versioned, and injected as either environment variables or files below `/run/trellis-secrets/`. Updating a secret does not mutate already-running allocations.

## Updates and failure handling

`recreate` stops outdated allocations before replacements. `rolling` starts bounded replacements and removes draining old allocations after replacements are healthy. Task restart policies limit restarts in a time window; failed control operations are retried with bounded exponential backoff and jitter. Blue/green and canary releases are composed from separate jobs or groups plus label-driven routing; they are not distinct built-in strategy values.
