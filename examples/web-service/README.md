# Healthy replicated web service

**Level:** Intermediate · **Prerequisites:** complete [`hello`](../hello/) and keep enough capacity for overlapping replacements

This example adds the first operational features to the minimal workload:

- `count: 2` creates two independently placed allocations of the `web` task group;
- task-level `networking.mode: host` makes nginx reachable through the node network;
- `host_port: 0` asks Trellis to reserve a free host port for each allocation;
- the HTTP health check keeps new capacity unready until nginx answers `/`;
- `strategy: rolling` and `max_parallel: 1` replace one replica at a time.

## Deploy and inspect

```sh
trellis jobs validate --file examples/web-service/trellis.yaml
trellis jobs apply --file examples/web-service/trellis.yaml --wait
trellis jobs status web-service
```

Use `--output json` when you need the exact allocation port mappings for testing or automation:

```sh
trellis --output json jobs status web-service
```

The dynamic host ports may differ between allocations. Host networking is intentionally explicit: it trades container network isolation for direct reachability on the node.

## Watch a rolling update

Change the nginx image to another qualified version, preview the semantic change, and watch the new revision converge:

```sh
trellis jobs diff --file examples/web-service/trellis.yaml
trellis jobs apply --file examples/web-service/trellis.yaml --wait
```

Old and new allocations overlap, so the cluster needs spare capacity. If the health check never succeeds, the rollout stalls instead of removing healthy old capacity. Use `trellis jobs diagnose web-service` and `trellis jobs logs web-service` before retrying.

```sh
trellis jobs delete web-service --wait
```

Continue through the [example learning path](../README.md) for secrets, volumes, sidecars, WireGuard networking, and advanced release patterns.
