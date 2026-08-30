# Allocation lifecycle and failure semantics

Trellis keeps allocation execution state in the existing Raft store. It does
not add a second workload resource or storage system: an allocation is still a
placement of one task group on one node, including all colocated tasks.

## Lifecycle and health

Lifecycle describes execution: `placed`, `starting`, `running`, `stopping`,
`stopped`, `failed`, or `lost`. Health is a separate observation:
`unknown`, `healthy`, or `unhealthy`. A running allocation may be unhealthy;
an unhealthy check does not by itself mean the container stopped.

Allocation responses retain the legacy `status` field as a projection for
older clients and additionally expose `phase`, `health`, allocation
`generation`, job revision, transition timestamps, reason, message, attempt,
and next retry time. Persisted `pending`, `healthy`, and `unhealthy` records are
read as `placed/unknown`, `running/healthy`, and `running/unhealthy`
respectively. Missing IDs and generations are recovered from the legacy name
and generation 1.

## Identity, retries, and fencing

An allocation ID belongs to the scheduler placement. Its generation identifies
one incarnation of the runtime resources. Starts and stops carry both values
plus a durable control-plane epoch. Nodes persist the highest epoch accepted
and reject older leaders even after `trellis-node` restarts. A repeated start
of the same ID, generation, and execution hash succeeds; a different execution
at that generation conflicts; and an obsolete generation cannot replace or
stop a newer one.

Control-plane retries keep the same ID and generation after an RPC timeout.
The attempt, diagnostic reason, and bounded exponential-backoff deadline are
stored in Raft, so a newly elected leader resumes the operation instead of
assuming that a lost response means a failed start. Agent task restart policy
is independent and still controls restarts of an already-created container.

## Node and containerd restarts

The agent atomically records each task's allocation identity, ports, mounts,
network attachment, health configuration, and restart policy beneath its data
directory. Containerd containers carry corresponding `trellis.*` labels. On
startup the agent inventories those containers before normal heartbeats,
adopts valid running tasks, restores port claims, and resumes health and
restart monitoring. Records are independent, so a malformed record does not
prevent recovery of unrelated workloads. A label-only workload remains
adopted and visible even when some optional local metadata cannot be rebuilt.

Raft leadership changes preserve jobs, allocations, generations, lifecycle,
and retry state. Agent restarts preserve running containerd workloads and local
execution metadata. A containerd data loss cannot be repaired in place; the
leader eventually records the missing allocation and schedules replacement
according to normal reconciliation.

## Loss and orphan collection

A new leader allows nodes to re-register before declaring allocations lost.
An allocation is marked `lost` only after the leader recovery grace and node
loss timeout; its history is retained. Absence from one heartbeat is not proof
that an allocation stopped.

Nodes never collect Trellis-managed resources merely because the leader is
unreachable. Collection requires an authenticated response from the current
leader, expiry of the leader recovery grace, and two consecutive positive
confirmations that the allocation ID and generation are not desired. Teardown
is retryable and never removes persistent volume contents. Container tasks,
network attachments, port claims, and local execution records are removed;
malformed or unidentifiable resources are left for operator inspection.

## Partition semantics

Trellis fences control-plane mutations, but it cannot guarantee at-most-once
workload effects across node/network failure: an unreachable old execution may
continue while the leader eventually marks it lost and creates a replacement.
The system therefore provides at-least-once workload execution across such a
partition. Applications that perform non-idempotent external side effects need
their own fencing or deduplication. Within one reachable node, allocation
generation and epoch checks prevent obsolete commands from replacing or
stopping newer execution.
