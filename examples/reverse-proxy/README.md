# Reverse proxy with Nginx

This example deploys Nginx as a Trellis-managed reverse proxy that
automatically discovers backend allocations using the Trellis API.

## How it works

Trellis provides three building blocks that make this possible without making
"services" a public resource:

1. **`api_access: true`** — When set on a task group, Trellis injects
   `TRELLIS_TOKEN` and `TRELLIS_ADDR` into the container. These credentials are
   scoped to the job's namespace and allow the container to call the Trellis
   control-plane API.

2. **`GET /v1/allocations`** — Returns allocation runtime information including
   task-group labels, node addresses, port mappings, and status. The endpoint
   supports `job` and `label` filters.

3. **Labels** — Task groups carry arbitrary key-value labels. The proxy uses
   labels to decide which allocations to expose and how to route to them.

The proxy job runs with `network_mode: host` so it can bind ports 80 and 443
directly. A sidecar script polls:

```text
GET /v1/allocations?label=trellis.expose:true
```

It keeps healthy allocations, reads their `trellis/domain` and
`trellis/path-prefix` labels, and regenerates the Nginx configuration.

The internal service-discovery catalog remains an implementation detail used by
Trellis DNS. This example depends only on allocations and task-group metadata,
which are public scheduler concepts.

## Label conventions

These labels are just conventions used by this example — Trellis itself is not
opinionated about them.

| Label | Purpose | Example |
| --- | --- | --- |
| `trellis.expose` | Mark a group for proxy discovery | `"true"` |
| `trellis/domain` | Virtual-host domain name | `app.example.com` |
| `trellis/path-prefix` | Path-prefix routing (optional) | `/api` |

## Files

| File | Description |
| --- | --- |
| `proxy.yaml` | Trellis job manifest for the Nginx reverse proxy |
| `app.yaml` | Example backend job with the appropriate labels |
| `sync-upstreams.sh` | Script that polls the allocations API and regenerates Nginx config |
| `nginx.conf.template` | Base Nginx configuration template |

## Deploying

```sh
# Deploy the backend application first
trellis jobs apply --file app.yaml

# Deploy the reverse proxy
trellis jobs apply --file proxy.yaml
```

## Customization

This example uses a simple shell script for allocation sync. For production use,
you might build a small purpose-built container image that:

- Watches allocation changes instead of polling
- Handles TLS certificate management (e.g., via Let's Encrypt)
- Supports more routing rules (header-based, weighted, etc.)
- Validates the generated config before reloading Nginx
