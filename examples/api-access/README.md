# API access

This example shows how a job can query namespace-scoped Trellis control-plane
state at runtime.

## How it works

Setting `api_access: true` on a task group injects two environment variables
into every container in that group:

| Variable | Value |
| --- | --- |
| `TRELLIS_TOKEN` | Namespace-scoped bearer token |
| `TRELLIS_ADDR` | Address of the Trellis control-plane API |

The injected token carries the task's namespace scope. It is intended for
namespace-scoped workload APIs such as job and allocation queries. It is **not**
a secret-management credential: the secret-management endpoints require the
cluster-authorized credential.

Treat `TRELLIS_TOKEN` as a credential and only enable `api_access` for trusted
containers that actually need control-plane access. Do not expose the token in
logs, application responses, or child processes unnecessarily.

## Manifests

| File | Purpose |
| --- | --- |
| `app.yaml` | Dashboard service with `api_access: true` |
| `worker.yaml` | Worker fleet with `component: worker` label |

## Allocations API

### List all allocations in the namespace

```sh
curl -H "Authorization: Bearer $TRELLIS_TOKEN" \
     "$TRELLIS_ADDR/v1/allocations"
```

### Filter by label

```sh
curl -H "Authorization: Bearer $TRELLIS_TOKEN" \
     "$TRELLIS_ADDR/v1/allocations?label=component:worker"
```

The `label` query parameter accepts `key` (any value) or `key:value` (exact
match). One label filter can be supplied per request.

### Response shape

The response contains allocation identity, lifecycle/health state, task-group
labels, node address, and allocated ports. For example:

```json
[
  {
    "id": "acme-worker-processor-ab12cd34",
    "job": "worker",
    "group": "processor",
    "status": "healthy",
    "phase": "running",
    "health": "healthy",
    "address": "10.0.1.5",
    "ports": [
      { "host_port": 32451, "container_port": 9090 }
    ],
    "labels": {
      "component": "worker",
      "queue": "default"
    }
  }
]
```

## Using it from application code

An application container can read `TRELLIS_TOKEN` and `TRELLIS_ADDR` from the
environment and query the allocation list on demand:

```go
token := os.Getenv("TRELLIS_TOKEN")
addr := os.Getenv("TRELLIS_ADDR")

req, _ := http.NewRequest("GET", addr+"/v1/allocations?label=component:worker", nil)
req.Header.Set("Authorization", "Bearer "+token)

// Execute the request, parse the response, and use the returned address/ports.
```

This is useful for:

- dashboards that display live namespace state;
- load balancers that need backend allocation addresses;
- job coordinators that need to enumerate workers; and
- trusted automation that reacts to allocation metadata.

See `examples/reverse-proxy` for a complete allocation-discovery example.

## DNS alternative

For straightforward service-to-service calls you often do not need API access.
Trellis DNS provides job-level discovery inside the namespace; use it when you
need a routable job name rather than allocation metadata.

## Deploying

```sh
trellis --namespace acme jobs apply --file worker.yaml
trellis --namespace acme jobs apply --file app.yaml
```

The dashboard containers start with `TRELLIS_TOKEN` and `TRELLIS_ADDR` set and
can query allocation state for their namespace.
