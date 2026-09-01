# Trellis examples

Every example in this directory uses the same user model as the CLI, dashboard, and public documentation. The `trellis.yaml` files are **YAML job manifests**: human-authored desired state that you apply with `trellis jobs apply --file ...`.

Use the terms from the [Trellis user model](../docs/public/user-model.md): jobs contain task groups and tasks; Trellis creates allocations at runtime to satisfy desired task-group capacity.

## Examples by concept

| Example | Demonstrates | Start here when... |
| --- | --- | --- |
| [`sidecar/`](sidecar/) | Multiple tasks in one task group | Two containers must share placement and lifecycle |
| [`secrets/`](secrets/) | Namespace-scoped secret references | A workload needs credentials outside the manifest |
| [`volumes/`](volumes/) | Advertised host volumes | A workload needs operator-provisioned local persistence |
| [`api-access/`](api-access/) | In-cluster Trellis API access | A trusted controller needs namespace-scoped runtime discovery |
| [`deployment-strategies/`](deployment-strategies/) | Rolling, blue/green, and canary patterns | You are designing release workflows |
| [`wordpress/`](wordpress/) | Small multi-container application composition | You want a compact application-stack example |
| [`patroni/`](patroni/) | External HA software on Trellis primitives | You are evaluating a stateful replicated system |

These examples are patterns, not additional resource types. Blue/green, canary, sidecars, and Patroni do not add special Trellis objects; they compose jobs, task groups, labels, health checks, discovery, and volumes.

## Applying an example

Examples assume the CLI is already connected to a cluster and namespace as described in [Getting started](../docs/public/getting-started.md).

```sh
trellis jobs apply --file examples/sidecar/trellis.yaml
trellis jobs status sidecar-demo
```

The first command applies desired state. The second inspects the job and its allocations. Use allocation-level events and logs only when you need runtime diagnostics.

Read each example's README before deploying it. Some examples intentionally require multiple nodes, host networking, pre-provisioned volumes, custom images, or external components.
