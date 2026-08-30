# Reverse proxy with Nginx

This example deploys Nginx as a Trellis-managed reverse proxy that discovers
backend allocations through the Trellis allocations API.

## How it works

Trellis provides three building blocks that make this possible without adding a
public service resource:

1. **`api_access: true`** — Trellis injects `TRELLIS_TOKEN` and `TRELLIS_ADDR`
   into the task-group containers. The token carries the job namespace scope and
   is intended for namespace-scoped workload APIs such as allocation queries.

2. **`GET /v1/allocations`** — Returns allocation runtime information including
   task-group labels, node addresses, port mappings, lifecycle, and health. The
   endpoint supports `job` and `label` filters.

3. **Labels** — Task groups carry arbitrary metadata. The proxy uses a small set
   of label conventions to choose routes.

The proxy job requests `network_mode: host` so it can expose the node's port 80.
A sync process polls:

```text
GET /v1/allocations?label=trellis.expose:true
```

It keeps healthy allocations with usable host ports, validates the routing label
syntax, groups them by domain/path, and regenerates the Nginx upstream
configuration. Generated configuration is checked with `nginx -t`; if validation
or a live reload fails, the previous configuration is restored and the same
change is retried on the next poll.

The internal service-discovery catalog remains an implementation detail used by
Trellis DNS. This example depends only on allocations and task-group metadata.

## Label conventions

These labels are conventions used by this example; Trellis itself does not
interpret them.

| Label | Purpose | Example |
| --- | --- | --- |
| `trellis.expose` | Mark a group for proxy discovery | `"true"` |
| `trellis/domain` | Virtual-host domain name | `app.example.com` |
| `trellis/path-prefix` | Path-prefix routing (optional; defaults to `/`) | `/api` |
| `trellis/weight` | Positive per-allocation Nginx weight (optional; defaults to `1`) | `"3"` |

For safety, the reference script accepts simple DNS-style domains and path
prefixes made from letters, digits, `.`, `_`, `/`, and `-`. Extend the
validation deliberately if your routing scheme needs additional syntax.

## Files

| File | Description |
| --- | --- |
| `Dockerfile` | Builds the proxy image with Nginx, curl, jq, the sync script, and entrypoint |
| `entrypoint.sh` | Installs the base config, starts the sync daemon, and runs Nginx in the foreground |
| `nginx.conf.template` | Base Nginx configuration with a `/health` endpoint and a `conf.d` include |
| `sync-upstreams.sh` | Polls the allocations API, renders routes, validates them, and reloads Nginx |
| `proxy.yaml` | Trellis job manifest for the reverse proxy |
| `app.yaml` | Example backend job with proxy-discovery labels |

## Building the proxy image

```sh
docker build -t your-registry/trellis-proxy:latest examples/reverse-proxy/
docker push your-registry/trellis-proxy:latest
```

Update the `image` field in `proxy.yaml` to match your registry and tag.

## Deploying

```sh
trellis --namespace acme jobs apply --file examples/reverse-proxy/app.yaml
trellis --namespace acme jobs apply --file examples/reverse-proxy/proxy.yaml
```

The sync process polls every five seconds by default. Use the proxy allocation
ID from `jobs status reverse-proxy` if you want to stream its logs:

```sh
trellis --namespace acme jobs status reverse-proxy
trellis --namespace acme jobs logs <allocation-id> --follow
```

The backend example uses
[traefik/whoami](https://github.com/traefik/whoami), a small HTTP container that
returns request details.

## Operational notes

- The shell implementation polls rather than watching an event stream.
- Removing the last healthy route for a domain removes its generated server
  block; Nginx's default server then returns 404.
- The sync script is intentionally small. Certificate management, request-rate
  limiting, authentication, richer path matching, and other edge-proxy policy
  remain operator concerns.
- The namespace API token is a credential. Do not log or expose it from the
  proxy container.

## Compiled sync helper

`orchestrator/cmd/trellis-proxy-sync/` contains a generic Go helper that uses the
same allocation API but renders a caller-provided template. It also reads the
`trellis/weight` label and exposes it as `.Weight` for each upstream.

Build it from the Go module directory:

```sh
cd orchestrator
go build ./cmd/trellis-proxy-sync
```

Run `./trellis-proxy-sync --help` for its flags. Unlike the bundled shell
reference, the generic helper does not know how to validate a particular proxy
configuration; supply an appropriate `-reload-cmd` and template for the proxy
you integrate it with.
