# Blue-green deployment

This example shows how to deploy a new version of a service with zero downtime
and an instant rollback path using a blue-green strategy.

## How it works

Two jobs — `api-blue` and `api-green` — run identical infrastructure but carry
different image tags. Only the active slot exposes itself to the reverse proxy
via the `trellis.expose: "true"` label. Traffic reaches only the active slot;
the inactive slot runs (and stays warm) but receives no traffic.

To promote the green slot, remove `trellis.expose` from blue and add it to
green. Because the green allocations are already running and healthy, promotion
is instant. Rolling back is the same operation in reverse.

## Manifests

| File | Purpose |
| --- | --- |
| `app-blue.yaml` | Active slot — labeled `trellis.expose: "true"` |
| `app-green.yaml` | Staging slot — no `trellis.expose` label |

The proxy reads the `trellis/domain` label and routes `app.example.com` to
whichever allocations carry `trellis.expose: "true"`. See
`examples/reverse-proxy` for the proxy setup.

## Deploying

### Initial deploy

Deploy the first version (blue) and the proxy:

```sh
trellis --namespace acme jobs apply --file app-blue.yaml
trellis --namespace acme jobs status api-blue
```

Wait until all three allocations are healthy.

### Stage the new version (green)

Deploy the new version without exposing it:

```sh
trellis --namespace acme jobs apply --file app-green.yaml
trellis --namespace acme jobs status api-green
```

Wait until all three green allocations are healthy. At this point v2 is running
and accepting health checks, but no user traffic reaches it.

### Validate

Run smoke tests against the green slot directly. You can reach it by querying
the allocations API for its host and port:

```sh
trellis --namespace acme jobs status api-green
```

Or from inside the cluster, the job is accessible at `api-green.acme.trellis`
(Trellis DNS resolves to healthy allocations in the namespace).

### Cut over

Promote green by adding `trellis.expose: "true"` to its labels and removing it
from blue. Edit `app-green.yaml` to add the label and `app-blue.yaml` to remove
it, then apply both:

```sh
trellis --namespace acme jobs apply --file app-green.yaml
trellis --namespace acme jobs apply --file app-blue.yaml
```

The proxy's sync loop (default 5-second interval) picks up the label change and
reroutes traffic to the green allocations. The cutover is near-instant once the
proxy reloads.

### Rollback

If v2 is faulty, revert the labels — green back to no expose, blue back to
exposed — and apply again:

```sh
trellis --namespace acme jobs apply --file app-blue.yaml
trellis --namespace acme jobs apply --file app-green.yaml
```

Traffic returns to blue before any user notices. Then fix v2 and try again.

### Cleanup

Once you are confident in v2, stop the blue slot:

```sh
trellis --namespace acme jobs delete api-blue
```

On the next cycle, rename green to blue (the slot names are just labels) so
the pattern stays consistent for the next deployment.

## Slot naming

The `slot: blue` / `slot: green` labels are for human bookkeeping only; Trellis
does not interpret them. The routing-relevant label is `trellis.expose: "true"`.
You can use any naming scheme — `active`/`canary`, `v1`/`v2`, etc.
