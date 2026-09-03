# Namespace networking and discovery

**Level:** intermediate  
**Prerequisites:** complete the sidecar stage; enable namespace networking on every node that may run this job; use at least two schedulable nodes if you want to observe real cross-node traffic.

This example introduces the private network attached to a Trellis namespace without introducing a proxy, ingress abstraction, or application platform. Both task groups request `networking.mode: namespace`, and therefore use the `runsc` runtime required by the current namespace-networking implementation.

The `web` task group runs two tutorial allocations. Once they are healthy, Trellis publishes them through DNS as:

```text
web.namespace-networking.default.trellis
```

The `observer` group runs the same small tutorial image with an opt-in peer probe. Every ten seconds it requests the `web` group's `/health` endpoint through that DNS name. This makes namespace networking and discovery visible in ordinary task logs instead of requiring a special debugging image.

## Prepare the nodes

When installing Trellis, answer yes to **Enable namespace networking on this node?** on every participating node. The installer adds the WireGuard and gVisor/runsc dependencies. Ensure the configured WireGuard UDP port can pass between the nodes (`51820` by default).

If these nodes were installed before namespace networking was enabled, use the node configuration and setup guidance in the [learning path](../../docs/public/learning-path.md#8-namespace-networking-and-discovery) before applying this manifest.

## Run the example

From the repository root:

```sh
trellisctl jobs validate --file examples/namespace-networking/trellis.yaml
trellisctl jobs diff --file examples/namespace-networking/trellis.yaml
trellisctl jobs apply --file examples/namespace-networking/trellis.yaml --wait
trellisctl jobs status namespace-networking
```

Check which nodes received the allocations. The scheduler prefers to spread replicas when capacity permits, but namespace networking does not require one allocation per node.

Then inspect the observer output:

```sh
trellisctl jobs logs namespace-networking --group observer --tail 100
```

A working namespace network and discovery path produces lines like:

```text
peer reachable: http://web.namespace-networking.default.trellis:8080/health (200 OK)
```

A temporary lookup or network failure is printed as `peer check failed`. If an allocation itself did not start, inspect the control-plane lifecycle separately:

```sh
trellisctl jobs diagnose namespace-networking
trellisctl jobs events namespace-networking
```

`jobs events` is allocation lifecycle history; `jobs logs` is stdout/stderr from the tutorial process. Use `--allocation` with the short ID from `jobs status` to narrow either view when necessary.

## What this demonstrates

- `namespace` is the manifest-level networking semantic; WireGuard and runsc are current node implementation details.
- Healthy task-group allocations are discoverable at `group.job.namespace.trellis`.
- Discovery returns runtime endpoints; it is not leader election, locking, or application consensus.
- No host port is declared or exposed. Communication stays on the namespace network.
- The namespace remains the isolation and discovery boundary. Jobs in another namespace do not join this network merely because they know the DNS name.

Remove the example when finished:

```sh
trellisctl jobs delete namespace-networking --wait
```

[Examples index](../README.md) · [Learning path](../../docs/public/learning-path.md#8-namespace-networking-and-discovery) · [Next: API access](../api-access/)
