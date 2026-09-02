# Healthy replicated web service

**Level:** Intermediate · **Prerequisites:** complete [`hello`](../hello/), then add at least two schedulable nodes; use a third node when exercising a rolling replacement

This example adds the first operational features to the minimal workload:

- `count: 2` creates two independently placed allocations of the `web` task group;
- task-level `networking.mode: host` makes nginx reachable through the node network;
- `host_port: 80` reserves the port nginx actually binds on each selected node;
- the HTTP health check keeps new capacity unready until nginx answers `/`;
- `strategy: rolling` and `max_parallel: 1` replace one replica at a time.

Host networking does not perform NAT or port translation. A declared host port must therefore match the port the process binds. Because both replicas reserve port 80, they must run on different nodes. A rolling replacement needs another node with port 80 free while old and new capacity overlap.

## Deploy and inspect

```sh
trellisctl jobs validate --file examples/web-service/trellis.yaml
trellisctl jobs apply --file examples/web-service/trellis.yaml --wait
trellisctl jobs status web-service
```

The allocation view shows each node and its reserved host port. Test nginx at `http://NODE_ADDRESS:80` for either allocation.

## Watch a rolling update

Change the nginx image to another qualified version, preview the semantic change, and watch the new revision converge:

```sh
trellisctl jobs diff --file examples/web-service/trellis.yaml
trellisctl jobs apply --file examples/web-service/trellis.yaml --wait
```

Old and new allocations overlap, so the cluster needs a third node with port 80 available. If the health check never succeeds, the rollout stalls instead of removing healthy old capacity. Use `trellisctl jobs diagnose web-service` and `trellisctl jobs logs web-service` before retrying.

```sh
trellisctl jobs delete web-service --wait
```

Continue through the [example learning path](../README.md) for secrets, volumes, sidecars, WireGuard networking, and advanced release patterns.
