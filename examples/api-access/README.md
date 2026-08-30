# API access

This example shows how a job can query the Trellis control-plane API at runtime
to discover other allocations in the same namespace.

## How it works

Setting `api_access: true` on a task group injects two environment variables
into every container in that group:

| Variable | Value |
| --- | --- |
| `TRELLIS_TOKEN` | Namespace-scoped bearer token |
| `TRELLIS_ADDR` | Base URL of the Trellis API (e.g. `https://trellis.internal`) |

The token is scoped to the namespace — it can read allocations, jobs, and
secrets within the namespace but cannot see other namespaces or internal cluster
resources. This makes it safe to inject into application containers.

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
# All healthy workers
curl -H "Authorization: Bearer $TRELLIS_TOKEN" \
     "$TRELLIS_ADDR/v1/allocations?label=component:worker"
```

The `label` query parameter accepts `key` (any value) or `key:value` (exact
match). Only one filter can be applied per request.

### Response shape

```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "job": "worker",
    "group": "processor",
    "status": "healthy",
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

An application container reads `TRELLIS_TOKEN` and `TRELLIS_ADDR` from the
environment to build a list of peer addresses at startup or on demand:

```go
token := os.Getenv("TRELLIS_TOKEN")
addr  := os.Getenv("TRELLIS_ADDR")

resp, _ := http.NewRequest("GET", addr+"/v1/allocations?label=component:worker", nil)
resp.Header.Set("Authorization", "Bearer "+token)

// Parse the response and connect to each allocation's address:port.
```

This is useful for:

- **Dashboard / control planes** that display the live state of other services.
- **Load balancers or sidecars** that need an up-to-date list of backend
  addresses (see `examples/reverse-proxy` for a complete example).
- **Job coordinators** that assign work to specific worker allocations.
- **Configuration tools** that push settings to every instance of a service.

## DNS alternative

For straightforward service-to-service calls you often do not need the API at
all. Trellis registers a DNS name for every job:

```
<job>.<namespace>.trellis
```

`worker.acme.trellis` resolves to the address of a healthy worker allocation
(round-robin across all healthy allocations). Use this when you need a single
endpoint rather than the full list.

## Deploying

```sh
trellis --namespace acme jobs apply --file worker.yaml
trellis --namespace acme jobs apply --file app.yaml
```

The dashboard containers start with `TRELLIS_TOKEN` and `TRELLIS_ADDR` set and
can immediately query the worker fleet.
