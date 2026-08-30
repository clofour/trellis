# Canary deployment

This example shows how to release a new version to a small fraction of traffic
before rolling it out fully.

## How it works

Two separate jobs — `api` (stable) and `api-canary` — both carry the
`trellis.expose: "true"` and `trellis/domain: app.example.com` labels. The
reverse proxy sees allocations from both jobs as upstreams for the same domain
and distributes requests across all of them.

Traffic split is determined by the ratio of healthy allocations: 9 stable
allocations and 1 canary allocation means roughly 10% of requests hit v2.
Adjusting `count` on either job shifts the proportion.

```
stable (9 replicas × v1)  ]
                            ]── nginx upstream for app.example.com
canary (1 replica  × v2)  ]
```

## Manifests

| File | Purpose |
| --- | --- |
| `stable.yaml` | Stable release — 9 replicas of v1 |
| `canary.yaml` | Canary release — 1 replica of v2 |

## Deploying

### Initial deploy

Start the stable release:

```sh
trellis --namespace acme jobs apply --file stable.yaml
trellis --namespace acme jobs status api
```

Wait until all nine allocations are healthy.

### Launch the canary

Deploy one canary replica alongside the stable fleet:

```sh
trellis --namespace acme jobs apply --file canary.yaml
trellis --namespace acme jobs status api-canary
```

Once the canary allocation is healthy the proxy includes it in the upstream pool.
About 10% of incoming requests will reach v2.

### Monitor

Watch error rates, latency, and any application-level signals for the canary
allocation. The `track: canary` label makes it easy to correlate logs or metrics
with the canary job:

```sh
trellis --namespace acme jobs status api-canary
trellis --namespace acme jobs logs  api-canary
```

### Increase the canary share

Raise `count` in `canary.yaml` and reapply to shift more traffic:

| stable count | canary count | Canary share |
| --- | --- | --- |
| 9 | 1 | ~10% |
| 9 | 3 | ~25% |
| 9 | 9 | ~50% |

### Promote to stable

When the canary looks healthy, update `stable.yaml` to point at v2 and scale
`canary.yaml` back down:

```sh
# Update stable.yaml: image: your-registry/api:v2
trellis --namespace acme jobs apply --file stable.yaml
trellis --namespace acme jobs delete api-canary
```

The stable fleet rolls over to v2 (using the `recreate` strategy by default, or
`rolling` if set). Once all stable allocations are healthy on v2 the canary job
is no longer needed.

### Rollback

If the canary shows problems, delete it. The stable fleet keeps serving v1
without interruption:

```sh
trellis --namespace acme jobs delete api-canary
```

## Traffic weighting

By default the split is proportional to replica count. For finer control you can
add a `trellis/weight` label to each job's task group:

```yaml
labels:
  trellis/weight: "3"
```

The `trellis-proxy-sync` binary reads this label and emits `weight=N` directives
in the nginx upstream block, shifting the per-allocation weight independently of
count. See `examples/reverse-proxy` for the proxy setup that supports this.
