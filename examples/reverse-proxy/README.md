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
   task-group labels, node addresses, port mappings, and health status. The
   endpoint supports `job` and `label` filters.

3. **Labels** — Task groups carry arbitrary key-value labels. The proxy uses
   labels to decide which allocations to expose and how to route to them.

The proxy job runs with `network_mode: host` so it can bind ports 80 and 443
directly. A sync process polls:

```text
GET /v1/allocations?label=trellis.expose:true
```

It keeps only healthy allocations, reads their `trellis/domain` and
`trellis/path-prefix` labels, and regenerates the Nginx upstream configuration.
Nginx is reloaded only when the config actually changes.

The internal service-discovery catalog remains an implementation detail used by
Trellis DNS. This example depends only on allocations and task-group metadata,
which are public scheduler concepts.

## Label conventions

These labels are conventions used by this example — Trellis itself is not
opinionated about them.

| Label | Purpose | Example |
| --- | --- | --- |
| `trellis.expose` | Mark a group for proxy discovery | `"true"` |
| `trellis/domain` | Virtual-host domain name | `app.example.com` |
| `trellis/path-prefix` | Path-prefix routing (optional) | `/api` |

## Files

| File | Description |
| --- | --- |
| `Dockerfile` | Builds the proxy image bundling Nginx, the sync script, and entrypoint |
| `entrypoint.sh` | Container startup: installs Nginx config, starts sync daemon, runs Nginx |
| `nginx.conf.template` | Base Nginx configuration with a `/health` endpoint and a conf.d include |
| `sync-upstreams.sh` | Shell script that polls the allocations API and regenerates Nginx config |
| `proxy.yaml` | Trellis job manifest for the reverse proxy |
| `app.yaml` | Example backend job with the appropriate labels set |

## Building the proxy image

The proxy container runs both Nginx and the sync script. Build the image from
this directory:

```sh
docker build -t your-registry/trellis-proxy:latest examples/reverse-proxy/
docker push your-registry/trellis-proxy:latest
```

Update the `image` field in `proxy.yaml` to match your registry and tag.

## Deploying

```sh
# Deploy the backend application
trellis jobs apply --file examples/reverse-proxy/app.yaml

# Deploy the reverse proxy
trellis jobs apply --file examples/reverse-proxy/proxy.yaml
```

The proxy will start discovering the backend allocations within the first poll
cycle (five seconds by default). You can watch the Nginx config update by
streaming the proxy container's logs.

The `app.yaml` backend uses [traefik/whoami](https://github.com/traefik/whoami)
for the frontend — a lightweight container that returns request details, making
it easy to verify that the proxy is correctly routing to the right upstream.

## Customization

The `sync-upstreams.sh` script is a complete reference implementation. For
production use you might extend it or replace it with a purpose-built process
that:

- Watches allocation changes via long-polling instead of fixed-interval polling
- Manages TLS certificates (e.g., via Let's Encrypt)
- Supports weighted routing, header-based routing, or circuit breaking
- Validates the generated config before reloading Nginx

Alternatively, `trellis-proxy-sync` (in `orchestrator/cmd/trellis-proxy-sync/`)
is a compiled Go implementation with a template-driven approach and a configurable
label selector. Build it with `go build ./cmd/trellis-proxy-sync` and see its
`--help` for usage.
