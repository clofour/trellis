# Sidecar pattern

This example runs nginx and an nginx Prometheus exporter as one scheduling unit. It demonstrates when two processes should share placement and lifecycle without being built into one image.

## What the manifest demonstrates

- A task group is the scaling and placement unit. With `count: 2`, Trellis creates two allocations and starts both `app` and `metrics-sidecar` in each allocation.
- The application owns the public port and readiness check; the exporter has a separate resource reservation.
- `network_mode: host` lets the exporter scrape nginx over `127.0.0.1`. Trellis tasks are otherwise separate containers and should not be assumed to share a loopback interface merely because they belong to one group.
- A rolling update limits replacement concurrency to one allocation.

## Prerequisites

Use at least two schedulable nodes if both replicas must run: host-networked nginx reserves port 80 on each selected node. Each node needs containerd and the two images must be pullable.

The stock nginx image does not enable `/stub_status`. Build an nginx image with a server/location such as:

```nginx
location = /stub_status {
    stub_status;
    allow 127.0.0.1;
    deny all;
}
```

Change the `app` image in `trellis.yaml` to that image before expecting exporter metrics.

## Deploy and inspect

```sh
trellis jobs apply --file examples/sidecar/trellis.yaml
trellis --namespace default jobs status sidecar-demo
trellis --namespace default jobs list
```

Use the allocation IDs from `jobs status` to inspect logs. Because allocation logs cover task output on the selected node, include the task name when correlating messages emitted by the two containers.

## Adapt the pattern

Sidecars work well for local proxies, metrics exporters, and tightly coupled agents. Give each task explicit resources and pin image versions. Do not use a sidecar merely to colocate unrelated services: a group scales, updates, drains, and fails as one unit. If the helper must discover many replicas rather than only its local application, use the API-access/controller pattern instead.
