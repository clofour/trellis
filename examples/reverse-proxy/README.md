# Reverse proxy with Nginx

This example deploys Nginx as a Trellis-managed reverse proxy that
automatically discovers backend services using the Trellis API.

## How it works

Trellis provides three building blocks that make this possible:

1. **`api_access: true`** — When set on a task group, Trellis injects
   `TRELLIS_TOKEN` and `TRELLIS_ADDR` into the container. These
   credentials are scoped to the job's namespace and allow the container
   to call the Trellis control-plane API.

2. **`GET /v1/services`** — Returns all healthy allocations in the
   namespace, including their labels, host addresses, and port mappings.

3. **Labels** — Task groups carry arbitrary key-value labels that flow
   through to the services API. The proxy uses labels to decide which
   services to expose and how to route to them.

The proxy job runs with `network_mode: host` so it can bind ports 80 and
443 directly. A sidecar script polls the services API every few seconds,
filters for task groups labeled `trellis.expose: "true"`, reads each
service's `trellis/domain` and `trellis/path-prefix` labels, and
regenerates the Nginx configuration.

## Label conventions

These labels are just conventions used by this example — Trellis itself
is not opinionated about them.

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
| `sync-upstreams.sh` | Script that polls the services API and regenerates Nginx config |
| `nginx.conf.template` | Base Nginx configuration template |

## Deploying

```sh
# Deploy the backend application first
trellis jobs apply --file app.yaml

# Deploy the reverse proxy
trellis jobs apply --file proxy.yaml
```

## Customization

This example uses a simple shell script for service sync. For production
use, you might build a small purpose-built container image that:

- Watches the services API for changes instead of polling
- Handles TLS certificate management (e.g., via Let's Encrypt)
- Supports more routing rules (header-based, weighted, etc.)
- Validates the generated config before reloading Nginx
