# Rolling update

This example shows how to replace a job revision incrementally with Trellis's
rolling update strategy.

## How it works

By default, when a task-group execution change creates a new job revision,
Trellis stops the old allocations before placing replacements (`recreate`).

Setting `update.strategy: rolling` changes that sequence. Trellis keeps the old
allocations running while it places up to `max_parallel` replacement
allocations. Once replacements become healthy, Trellis stops the corresponding
number of old allocations and continues with the next replacements.

This is a **surge-style** rolling update: Trellis does not intentionally stop a
healthy old allocation before a healthy replacement exists. As a consequence,
the cluster needs enough spare capacity to place the in-flight replacements. If
there is no spare CPU, memory, port, volume, or constraint-compatible capacity,
the rollout waits rather than taking an old allocation down to make room.

## Manifests

`app-v1.yaml` and `app-v2.yaml` are identical except for the image tag. Both
set:

```yaml
update:
  strategy: rolling
  max_parallel: 1
```

With three replicas and `max_parallel: 1`, Trellis starts one v2 replacement
while the three v1 allocations remain desired. After that replacement is
healthy, one v1 allocation is stopped and the next v2 replacement can start.

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

With sufficient spare capacity and healthy replacements, the old allocations
remain available until replacements are ready. During a `max_parallel: 1`
rollout you may briefly see four running allocations: three old allocations plus
one in-flight replacement.

### Rollback

Rolling back is the same operation in the other direction: reapply the previous
manifest.

```sh
trellis --namespace acme jobs apply --file app-v1.yaml
```

The image change creates another revision and Trellis rolls back using the same
strategy.

## Tuning

| `max_parallel` | Behavior |
| --- | --- |
| `1` (default) | At most one not-yet-healthy replacement is in flight. Requires capacity for roughly one extra allocation. |
| `2` | Up to two replacements can be in flight. Faster when the cluster can fit them. |
| equal to `count` | Trellis may start a full replacement set before stopping the old set; this is not the same as `recreate`. |

A larger value increases possible rollout concurrency and the temporary surge
in resource usage. It does not tell Trellis to make that many old allocations
unavailable.

## Health-check timing

Trellis waits for replacement allocations to become healthy before retiring the
old allocations they replace. If a new revision never becomes healthy, the
rollout stalls and the remaining healthy old allocations stay in service.

Jobs without a health check are considered healthy once they are running, so
use a meaningful health check when a replacement must prove readiness before an
old allocation is removed.
