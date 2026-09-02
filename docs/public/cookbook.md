# Cookbook

This cookbook starts with an operational outcome and explains the reusable Trellis pattern that achieves it. It is not an example index and does not define new resource types. Each recipe composes the primitives in the [job manifest reference](job-specification.md), explains why the composition works, and calls out the tradeoffs that should survive when you adapt it.

Use [Getting Started](getting-started.md) and the [learning path](learning-path.md) to learn Trellis in order. Use this page after you understand jobs, task groups, tasks, allocations, health, and namespaces.

## Put stable ingress in front of changing allocations

**Outcome:** expose one stable service endpoint while replicas scale, move between nodes, and roll to new revisions.

Give the serving task group a routing label, expose the application through a dynamic host port, and define a meaningful health check:

```yaml
labels:
  route: web

tasks:
  - name: app
    image: registry.example.com/app:v1
    networking:
      mode: host
      ports:
        - host_port: 0
          container_port: 8080
    health_check:
      type: http
      port: 8080
      path: /ready
```

Run a trusted controller with `api_access: namespace`. It should query allocations by label, include only healthy endpoints, render or update the upstream set, and preserve its last known-good routing state through temporary control-plane failures. Namespace mode grants access only to the controller job's own namespace, which is sufficient for a namespace-local ingress controller. The bundled `trellis-proxy-sync` implements this polling pattern. When an allocation exposes more than one port, select the intended application port explicitly with `-container-port` rather than depending on mapping order.

Keep the public listener itself stable: place it deliberately or put an external load balancer in front of it. The routing controller is ordinary workload code, so make its retries, timeouts, reload behavior, and credential handling explicit. Trellis discovers endpoints; it does not make the proxy highly available for you.

## Connect services privately inside a namespace

**Outcome:** let services communicate across nodes without exposing their application ports on the node network.

Use `networking.mode: wireguard` for tasks that should join the namespace's private mesh:

```yaml
runtime: runsc
tasks:
  - name: api
    image: registry.example.com/api:v1
    networking:
      mode: wireguard
```

Enable and configure WireGuard consistently on every node that may run the workload. Healthy allocation endpoints enter Trellis discovery, and DNS names follow the shape:

```text
group.job.namespace.trellis
```

Use DNS for locating healthy service instances, not for leader election, distributed locking, or application consensus. Namespace discovery is an availability mechanism; applications that require a single writer or elected primary still need their own coordination protocol.

Use `host` networking instead when a task deliberately needs the node network or host-port exposure. Leave networking omitted for work that needs no external routes. Networking is selected per task, so colocated tasks do not need to share the same exposure model.

## Choose task-group boundaries deliberately

**Outcome:** couple only the containers that must share placement, scaling, update, restart, and drain behavior.

Put tightly related processes in one task group when each replica should contain all of them—for example an application plus a local metrics exporter or proxy:

```yaml
task_groups:
  - name: web
    count: 3
    tasks:
      - name: app
        image: registry.example.com/app:v1
      - name: helper
        image: registry.example.com/helper:v1
```

`count: 3` creates three allocations and therefore three copies of both tasks. Give every task its own resources and health check when relevant.

Do not use a task group merely as a manifest-organizing device. If two components must scale independently, be updated independently, survive each other's failures, or run on different classes of nodes, separate them into different task groups or jobs. Task-group boundaries are lifecycle boundaries.

## Gate service readiness on application health

**Outcome:** keep a process out of routing and rollout decisions until it can actually serve useful work.

Use a health check that represents readiness rather than mere process existence:

```yaml
health_check:
  type: http
  port: 8080
  path: /ready
  interval: 5s
  timeout: 2s
  threshold: 2
```

HTTP and TCP checks are appropriate when readiness is externally observable. Script checks are useful when readiness depends on an in-container condition or when the task is not reachable through a host port.

A check should be strong enough to reject an unusable instance but not so broad that an unrelated downstream outage marks every replica unhealthy. This matters especially for rolling updates and health-filtered discovery. A task without an explicit check is treated as healthy once running, which is convenient for simple workloads but weaker than application-aware readiness.

## Restart transiently failing tasks without hiding persistent failure

**Outcome:** recover automatically from occasional process failures while eventually surfacing a broken workload for diagnosis.

Configure the restart policy on the task group:

```yaml
restart:
  max_restarts: 3
  window: 5m
```

The group-level policy applies to the tasks in each allocation. Use a small bounded retry budget for failures that are plausibly transient. Once the allowed failures in the window are exhausted, the allocation remains failed so an operator can inspect its reason, message, attempts, events, and logs instead of entering an unlimited crash loop.

Do not treat restart policy as a substitute for readiness checks or correct dependencies. Repeated startup failures usually indicate a bad revision, missing secret, unavailable volume, invalid configuration, or application defect; use `trellisctl jobs diagnose NAME` after the retry budget is exhausted.

## Choose between recreate and rolling replacement

**Outcome:** make update behavior match the workload's overlap and availability requirements.

Use the default `recreate` strategy when old and new instances must not overlap, temporary reduced capacity is acceptable, or the workload cannot safely run two revisions simultaneously:

```yaml
update:
  strategy: recreate
```

Use `rolling` when service availability matters and old and new revisions may coexist:

```yaml
count: 3
update:
  strategy: rolling
  max_parallel: 1
```

Rolling replacement marks old-revision allocations as draining, starts bounded replacement capacity, and removes old allocations as healthy replacements become available. `max_parallel` limits not-yet-healthy replacements in flight; it is not a percentage.

Rolling updates require spare schedulable capacity and a useful health check. Recreate updates avoid overlap but can reduce or eliminate service capacity during replacement. In either case, inspect the diff before applying and treat rollback as another desired-state revision: restore the earlier image/configuration and apply it again.

## Switch complete releases with blue/green routing

**Outcome:** validate a full new release before moving production traffic, while keeping a fast route-only rollback.

Trellis has no blue/green resource type. Run the two releases as independently named jobs or otherwise independent routing targets. Give each release a distinct discovery label, validate the inactive release, then change the external routing controller from the old label to the new one.

Keep the old release alive for an observation window so rollback is a routing change rather than another deployment. Because both releases coexist, budget roughly double workload capacity during the overlap. Shared databases, queues, and message formats must remain compatible with every revision that can run at the same time.

Store and review the routing switch like application code. Trellis maintains the workloads; the proxy or external load balancer owns which release receives production traffic.

## Expose a canary to a bounded share of traffic

**Outcome:** send limited real traffic to a new release while the stable release continues serving most requests.

Run stable and canary as separate jobs or independently routable groups. Give them the same route label and distinct release metadata. With the bundled proxy synchronizer, `trellis/weight` can be passed through to a proxy template:

```yaml
labels:
  route: web
  track: canary
  trellis/weight: "5"
```

Start with little canary capacity and low effective routing weight. Compare errors, latency, saturation, and application-specific success metrics by release track before increasing exposure.

Weights are attached to discovered allocations. Replica count therefore changes aggregate weight: four stable allocations at weight 100 and one canary at weight 5 produce an aggregate 400:5 pool, not 100:5. Sticky sessions and long-lived connections can further change observed traffic share. Calculate the effective pool rather than treating one label value as a percentage.

## Isolate independent tenants or environments with namespaces

**Outcome:** keep workloads, discovery, secrets, and scoped API credentials separated even when they share one cluster.

Use different namespaces when two sets of workloads should not share the same authorization and discovery boundary. A namespace is not just a prefix for job names: it is Trellis's tenant, workload-isolation, discovery, and namespace-token boundary.

Keep related services that need private discovery in the same namespace. Put unrelated tenants, trust domains, or environments in different namespaces. Create CLI contexts that select the intended namespace so routine commands do not depend on remembering `--namespace` every time.

Do not emulate namespaces with job-name prefixes or labels. Labels are useful for selection and routing; they are not an authorization boundary.

## Place workloads only on compatible nodes

**Outcome:** schedule a task group only where its runtime, architecture, hardware, locality, or operator-managed dependency is available.

Use task-group constraints for exact matches against `os`, `arch`, or node labels:

```yaml
constraints:
  - attribute: arch
    value: amd64
  - attribute: storage
    value: fast
```

Treat node labels as declared capabilities rather than arbitrary decoration. Apply a label only when every node carrying it really satisfies the implied contract.

Constraints are hard filters: if no healthy non-draining node satisfies them and has enough capacity, the workload remains pending. Do not over-constrain ordinary replicas when soft scheduler spreading is sufficient. Use constraints for requirements; let the scheduler choose among equivalent nodes.

## Rotate a credential without storing plaintext in the manifest

**Outcome:** deliver credentials to containers while keeping plaintext out of Git and job YAML.

Create or update the namespace secret separately from the workload:

```sh
printf %s "$NEW_VALUE" | trellisctl --namespace payments secrets set service-credential --stdin
```

Reference only the secret name from the task and choose environment or file delivery according to the application's interface. Prefer a file when the application supports it and a credential does not need to appear in the process environment.

For concurrent automation, read secret metadata and update with `--expected-version N`. A secret update affects allocations started afterward; Trellis does not mutate the environment or filesystem of an already-running container. Rotate consumers by replacing allocations in an order compatible with both the old and new credential.

Back up the secrets-encryption key separately from desired-state backups. Encrypted secret records are not recoverable without that key.

## Keep scratch data separate from persistent local data

**Outcome:** make it explicit which data may disappear with an allocation and which data must survive allocation replacement on a node.

Use an ordinary volume without `host_volume` for allocation-local scratch data. Use an advertised `host_volume` when the data must live at an operator-managed node path:

```yaml
volumes:
  - name: data
    path: /var/lib/app
    host_volume: app-data
```

A host-volume name is a scheduling capability. Trellis places the allocation only on nodes advertising that name, but it does not create, replicate, snapshot, move, or restore the underlying bytes. Two nodes advertising the same volume name may still contain unrelated data.

Pair persistent local storage with deliberate node preparation, ownership, backups, restore drills, and—where availability matters—application replication or an external storage system. Do not assume rescheduling to another compatible node implies the same data is present there.

## Let a trusted workload automate its namespace

**Outcome:** allow an in-cluster controller to inspect and reconcile resources in the namespace that contains the controller job.

Set `api_access: namespace` on the controller's task group. Namespace mode cannot name or select some other namespace: Trellis creates a persistent bearer token restricted to the **job's own namespace**. It injects:

- `TRELLIS_ADDR` — control-plane address;
- `TRELLIS_TOKEN` — bearer token restricted to the job namespace;
- `TRELLIS_NAMESPACE` — that same job namespace, for request scoping;
- `TRELLIS_CA_CERT` — cluster CA PEM when TLS is configured.

Use a namespace-aware client and verify TLS with the injected CA. Treat an address without an explicit scheme as HTTPS, matching first-party Trellis client behavior. Raw HTTP clients should send both Bearer authentication and `X-Trellis-Namespace`.

Every task in the group receives the injected environment, so use only reviewed images in an API-enabled group. Controllers should set request deadlines, retry transient failures with backoff, tolerate resources changing between reads, avoid leaking credentials into logs or metrics, and preserve useful last-known-good state through temporary API outages.

Prefer this mode for proxies, discovery controllers, and automation that does not need cluster administration. Changing `TRELLIS_NAMESPACE` or the request header cannot broaden the token beyond the job namespace.

## Give a trusted operator workload cluster API access

**Outcome:** let a workload perform administrative or cross-namespace operations that a namespace controller cannot perform.

Set `api_access: cluster` only on a fully trusted task group. Trellis injects the cluster administrator token in `TRELLIS_TOKEN`. It also sets `TRELLIS_NAMESPACE` to the job's namespace as a conservative default for clients that automatically send a namespace header, but that value is **not** an authorization boundary for a cluster token.

Cluster mode is appropriate for an operator surface that genuinely needs node maintenance, backups, secret administration, cross-namespace operations, Raft controls, or equivalent administrator APIs. It is not a shortcut for giving an ordinary application access to another namespace.

Treat compromise of any task in the group as compromise of the cluster credential. Pin and review images, avoid unrelated sidecars, keep the token out of logs/metrics/browser code, and prefer `namespace` whenever it can express the controller's job.

## Run application-managed replicated state without confusing scheduling with consensus

**Outcome:** let Trellis place and operate replicated stateful members while the application remains responsible for data correctness and leadership.

Use Trellis for the container layer: replica count, placement constraints, network attachment, secret delivery, host-volume requirements, restart policy, health observation, and discovery. Use the stateful system's native mechanisms for replication, leader election or consensus, fencing, membership changes, backups, and recovery.

Do not infer a primary from Trellis scheduling order or health status. Scheduler replica spreading improves failure distribution but is not a consensus algorithm. Trellis discovery tells members where healthy allocations are; it does not decide which member may accept writes.

Before treating such a deployment as highly available, test node loss, leader loss, stale members, replacement onto a node with different local data, restore from backup, and network partitions. If the storage layer is network-backed, verify that the application's own failover model safely controls which member mounts or writes the data.

## Choose the right pattern

| Desired outcome | Pattern | Primary tradeoff |
|---|---|---|
| Stable public endpoint for changing replicas | Label-driven ingress controller | Controller and listener need their own availability design |
| Private cross-node service communication | Namespace WireGuard plus discovery | Requires consistent node networking configuration |
| Per-replica helper process | Multiple tasks in one task group | Coupled placement, scaling, update, and failure behavior |
| Application-aware readiness | Health-filtered discovery and rollout | A poor check can admit bad instances or stall deployment |
| Recovery from transient process failure | Bounded restart policy | Persistent failure eventually needs operator diagnosis |
| Non-overlapping update | Recreate strategy | Temporary capacity loss |
| Availability-preserving in-place update | Rolling strategy | Spare capacity and reliable readiness required |
| Full release validation and fast route rollback | Blue/green | Both releases consume capacity during overlap |
| Limited real-traffic exposure | Weighted canary | Effective share depends on replicas and proxy semantics |
| Tenant/environment isolation | Separate namespaces | Cross-namespace communication must be designed explicitly |
| Specialized-node placement | Hard constraints | Unsatisfied requirements leave work pending |
| Credential delivery and rotation | Versioned namespace secret | Running allocations need replacement for new values |
| Persistent node-local data | Advertised host volume | No built-in replication, movement, or backup |
| Namespace-local automation | `api_access: namespace` | Every task in the group becomes a namespace credential holder |
| Administrative/cross-namespace automation | `api_access: cluster` | Every task in the group becomes a cluster administrator |
| Replicated stateful service | Trellis lifecycle plus application-native HA | Application remains responsible for consensus and data safety |

Concrete manifests live in the [examples index](../../examples/README.md); use them to see syntax after choosing the pattern here.

[Documentation index](../README.md) · [Previous: Operations](operations.md) · [Next: Dashboard](dashboard.md)
