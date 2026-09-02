# Replicated web service

**Level:** Intermediate · **Prerequisites:** complete [`web-service`](../web-service/) and have at least two schedulable nodes

This example changes one idea from the single-service example: `count` becomes `2`.

Each replica reserves host port 8080. Because one node cannot satisfy the same fixed host-port reservation twice, the two allocations must land on different nodes. This makes the placement consequence of scaling explicit before adding rolling-update overlap.

## Deploy and inspect placement

```sh
trellisctl jobs validate --file examples/replicated-service/trellis.yaml
trellisctl jobs diff --file examples/replicated-service/trellis.yaml
trellisctl jobs apply --file examples/replicated-service/trellis.yaml --wait
trellisctl jobs status replicated-service
```

The status output should show two healthy allocations on different nodes. Query either node on port 8080 to reach a replica.

If only one compatible node is available, one replica cannot be placed. Use:

```sh
trellisctl jobs diagnose replicated-service
trellisctl nodes list
trellisctl nodes status NODE
```

to inspect the placement failure and the relevant node capacity/configuration.

Remove it when finished:

```sh
trellisctl jobs delete replicated-service --wait
```

Next, continue to [`rolling-update`](../rolling-update/) to keep these two replicas and introduce overlapping replacement as a separate concept.
