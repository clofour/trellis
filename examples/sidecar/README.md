# Sidecar pattern

This example shows how to colocate a helper container alongside your main
application using multiple tasks in a single task group.

## How it works

A task group can contain more than one task. All tasks in a group are placed
together on the same node as part of one allocation. Tasks share the same
host-network namespace, which means they can reach each other via `localhost`.
They can also share data through named volumes declared in the task group.

Common sidecar patterns:

| Sidecar | Purpose |
| --- | --- |
| Log shipper (e.g. Fluent Bit) | Tail and forward application logs |
| Metrics exporter | Scrape `/metrics` and push to a metrics store |
| Secrets agent | Rotate secrets and write them to a shared volume |
| Service mesh proxy | Intercept traffic for mTLS and observability |

## Manifest

`app.yaml` defines a task group with two tasks: `app` (the main service) and
`log-shipper` (Fluent Bit). Both tasks land on the same node for every
allocation.

The `app` task writes logs to `/var/log/api`. The `log-shipper` mounts the same
path via the `logs` volume and tails the files. Trellis creates the volume on
the node and mounts it into both containers.

```yaml
tasks:
  - name: app
    …
    env:
      LOG_DIR: /var/log/api
    volumes:
      - name: logs        # app writes to this volume
        path: /var/log/api

  - name: log-shipper
    …
    volumes:
      - name: logs        # shipper reads from the same volume
        path: /var/log/api
```

## Resources

Each task gets its own resource allocation. The example gives the app 500 CPU
millicores and 256 MiB, and the log shipper 50 millicores and 32 MiB. Set
these based on your observed usage — a log shipper typically needs very few
resources.

## Localhost communication

Because both tasks share the host network, the sidecar can reach the app on
`localhost:8080` without any additional configuration. This is useful for a
metrics exporter that scrapes the app's `/metrics` endpoint:

```yaml
- name: metrics-exporter
  image: your-registry/exporter:latest
  resources:
    cpu: 50
    memory: 16777216
  env:
    SCRAPE_TARGET: http://localhost:8080/metrics
```

## Health checks

Health checks are per task. A task group allocation is considered healthy only
when all tasks with health checks pass. The example only defines a health check
on the `app` task; the log shipper has no health check and is treated as healthy
once it starts.

## Deploying

```sh
trellis --namespace acme jobs apply --file app.yaml
trellis --namespace acme jobs status api
```

All three allocations each start two containers: `app` and `log-shipper`.
