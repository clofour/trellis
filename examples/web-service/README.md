# Healthy web service

**Level:** Intermediate · **Prerequisites:** complete [`hello`](../hello/) on one healthy node

This example adds the first service-specific concerns to the minimal workload without introducing scaling or rollout strategy yet:

- task-level `networking.mode: host` makes the tutorial application reachable through the node network;
- `port: 8080` reserves the port the process actually binds;
- an HTTP health check keeps the allocation unready until `/health` succeeds.

Host networking does not perform NAT or port translation. A declared port must therefore match the port the process binds.

## Deploy and inspect

```sh
trellisctl jobs validate --file examples/web-service/trellis.yaml
trellisctl jobs diff --file examples/web-service/trellis.yaml
trellisctl jobs apply --file examples/web-service/trellis.yaml --wait
trellisctl jobs status web-service
```

The allocation view shows the selected node and reserved host port. Open `http://NODE_ADDRESS:8080` or query `/health` to verify that the service is reachable.

If the health check does not succeed, use:

```sh
trellisctl jobs diagnose web-service
trellisctl jobs logs web-service
```

Remove it when finished:

```sh
trellisctl jobs delete web-service --wait
```

Next, continue to [`replicated-service`](../replicated-service/) to add a second replica and learn the placement consequences of a fixed host port before introducing rolling replacement.
