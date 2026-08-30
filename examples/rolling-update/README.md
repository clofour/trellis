# Rolling update

This example shows how to deploy a new version of a job without downtime
using Trellis's rolling update strategy.

## How it works

By default, when you reapply a job with a changed revision Trellis stops all
existing allocations before starting replacements (`recreate` strategy). That
causes a brief outage.

Setting `update.strategy: rolling` instead replaces allocations in batches:

1. Drain `max_parallel` old allocations.
2. Wait for their replacements to become healthy.
3. Repeat until all allocations are on the new revision.

At most `count - max_parallel` allocations are unavailable at any point during
the update, so the group keeps serving traffic throughout.

## Manifests

`app-v1.yaml` and `app-v2.yaml` are identical except for the image tag. Both
set:

```yaml
update:
  strategy: rolling
  max_parallel: 1
```

With three replicas and `max_parallel: 1`, Trellis replaces one allocation per
batch. Two healthy replicas serve traffic throughout the update.

## Deploying

### Initial deploy

```sh
trellis --namespace acme jobs apply --file app-v1.yaml
trellis --namespace acme jobs status api
```

Wait until all three allocations are healthy.

### Rolling update to v2

```sh
trellis --namespace acme jobs apply --file app-v2.yaml
```

Watch the update proceed:

```sh
trellis --namespace acme jobs status api
```

The status output shows desired, running, and healthy counts. During the
update you will see one allocation stopping and its replacement starting while
the other two remain healthy. The group never drops below two healthy
allocations.

### Rollback

Rolling back is identical to updating — reapply the previous manifest:

```sh
trellis --namespace acme jobs apply --file app-v1.yaml
```

Trellis detects the revision change and rolls back one allocation at a time
using the same strategy.

## Tuning

| `max_parallel` | Behavior |
| --- | --- |
| `1` (default) | One replacement at a time. Slowest but keeps the most replicas healthy. |
| `2` | Two replacements at a time. Faster update, one fewer healthy replica during each batch. |
| equal to `count` | Effectively `recreate` — all replaced at once. |

Larger values reduce update duration but increase the number of allocations
temporarily unavailable. Set `max_parallel` to at most `count - 1` to keep at
least one allocation healthy at all times.

## Health-check timing

Trellis gates each batch on the replacement allocations becoming healthy. If a
deployment introduces a regression that causes health checks to fail, the
update stalls: the remaining old allocations continue serving traffic, and you
can either fix the image tag and reapply, or roll back to the previous version.
