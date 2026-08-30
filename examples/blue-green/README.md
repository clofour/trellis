# Blue-green deployment

This example shows how to keep two independent releases running and switch
proxy routing between them without replacing the active allocations during the
cutover.

## How it works

Two jobs — `api-blue` and `api-green` — run identical infrastructure but carry
different image tags. Only the active slot exposes itself to the reverse proxy
via the `trellis.expose: "true"` label. Traffic reaches only the active slot;
the inactive slot stays warm but is not selected by the proxy.

To promote the green slot, remove `trellis.expose` from blue and add it to
green. Because the green allocations are already running and healthy, no
application allocation has to be restarted for the cutover. The proxy observes
the label changes on its next sync cycle. Rolling back is the same operation in
reverse.

## Manifests

| File | Purpose |
| --- | --- |
| `app-blue.yaml` | Active slot — labeled `trellis.expose: "true"` |
| `app-green.yaml` | Staging slot — no `trellis.expose` label |

The proxy reads the `trellis/domain` label and routes `app.example.com` to
whichever healthy allocations carry `trellis.expose: "true"`. See
`examples/reverse-proxy` for the proxy setup.

## Deploying

### Initial deploy

Deploy the first version (blue):

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

Wait until all three green allocations are healthy. At this point v2 is
running and being health-checked, but the proxy does not route user traffic to
it.

### Validate

Inspect `jobs status api-green` to get the allocation addresses and port
mappings exposed by the control plane, then run whatever smoke tests are
appropriate for your environment.

### Cut over

Promote green by adding `trellis.expose: "true"` to its labels and removing it
from blue. Edit `app-green.yaml` to add the label and `app-blue.yaml` to remove
it, then apply both:

```sh
trellis --namespace acme jobs apply --file app-green.yaml
trellis --namespace acme jobs apply --file app-blue.yaml
```

Label-only changes do not create a new execution revision, so the existing
allocations stay running. The proxy's sync loop (five seconds by default) picks
up the change and reloads its routing configuration.

### Rollback

If v2 is faulty, revert the labels — green back to unexposed, blue back to
exposed — and apply again:

```sh
trellis --namespace acme jobs apply --file app-blue.yaml
trellis --namespace acme jobs apply --file app-green.yaml
```

### Cleanup

Once you are confident in v2, stop the old job:

```sh
trellis --namespace acme jobs destroy api-blue
```

`api-blue` and `api-green` are separate **job names**, not labels. For the next
deployment you can either reuse the now-free blue job name for the next staged
release or keep whatever naming convention is clearest to you; Trellis does
not have a slot abstraction.

## Slot labels

The `slot: blue` / `slot: green` labels are for human bookkeeping only; Trellis
does not interpret them. The routing-relevant convention in this example is
`trellis.expose: "true"`.
