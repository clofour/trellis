# Canary deployment

This example shows how to release a new version to a small fraction of traffic
before rolling it out fully.

## How it works

Two separate jobs — `api` (stable) and `api-canary` — both carry the
`trellis.expose: "true"` and `trellis/domain: app.example.com` labels. The
reverse proxy sees allocations from both jobs as upstreams for the same domain
and distributes requests across all of them.

Without explicit weights, traffic share is approximately proportional to the
number of healthy allocations: 9 stable allocations and 1 canary allocation
means roughly 10% of requests hit v2. Adjusting `count` on either job shifts the
proportion.

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
About 10% of incoming requests will reach v2 when no explicit weights are set.

### Monitor

Watch error rates, latency, and application-level signals for the canary. The
`jobs status` output includes allocation IDs; pass one of those IDs to
`jobs logs`:

```sh
trellis --namespace acme jobs status api-canary
trellis --namespace acme jobs logs <allocation-id>
```

### Increase the canary share

Raise `count` in `canary.yaml` and reapply to shift more traffic:

| stable count | canary count | Canary share |
| --- | --- | --- |
| 9 | 1 | ~10% |
| 9 | 3 | ~25% |
| 9 | 9 | ~50% |

### Promote to stable

When the canary looks healthy, update `stable.yaml` to point at v2 and apply it:

```sh
# Update stable.yaml: image: your-registry/api:v2
trellis --namespace acme jobs apply --file stable.yaml
```

The stable fleet uses `recreate` unless you configure an `update` strategy. If
you want promotion without intentionally taking the stable fleet down first,
configure `strategy: rolling` and ensure the cluster has spare capacity for the
replacement allocations.

After the stable job is healthy on v2, remove the canary job:

```sh
trellis --namespace acme jobs destroy api-canary
```

### Rollback

If the canary shows problems, destroy it. The stable fleet keeps serving v1:

```sh
trellis --namespace acme jobs destroy api-canary
```

## Traffic weighting

For finer control, add a positive integer `trellis/weight` label to a task
group:

```yaml
labels:
  trellis.expose: "true"
  trellis/domain: app.example.com
  trellis/weight: "3"
```

The reverse-proxy sync script emits the value as Nginx's per-upstream
`weight=N`. The compiled `trellis-proxy-sync` helper also reads the same label
and exposes it to templates as `.Weight`.
