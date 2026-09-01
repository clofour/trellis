# Cookbook

This cookbook starts with an operational outcome and shows the Trellis pattern that achieves it. It is not an example index: each recipe explains the moving pieces, why they work, and the tradeoffs to preserve when adapting the pattern.

All manifests use the schema in the [job manifest reference](job-specification.md). Commands assume a selected context or explicit connection variables as described in [Getting Started](getting-started.md) and [CLI workflows](cli.md).

## Put a reverse proxy in front of dynamic allocations

**Outcome:** expose a stable HTTP endpoint even though allocation addresses change during scaling, rescheduling, and deployment.

1. Give every backend task group a routing label and a dynamically allocated host port:

   ```yaml
   labels:
     route: storefront
   tasks:
     - name: web
       image: registry.example.com/storefront:2026-08-31
       networking:
         mode: host
         ports:
           - host_port: 0
             container_port: 8080
       health_check:
         type: http
         port: 8080
         path: /healthz
   ```

2. Run a trusted proxy controller with `api_access: true`. The bundled `trellis-proxy-sync` polls allocations matching `route:storefront`, excludes endpoints that are not healthy, renders a Go template, and optionally reloads the proxy.
3. Keep the public listener on a stable node or external load balancer. The controller should update only the upstream block; it should not expose its namespace token.

A minimal nginx template looks like this:

```nginx
upstream storefront {
{{- range .Upstreams }}
    server {{ .Address }}:{{ .Port }} weight={{ .Weight }};
{{- end }}
}
server {
    listen 80;
    location / { proxy_pass http://storefront; }
}
```

Start the synchronizer inside the proxy image:

```sh
trellis-proxy-sync \
  -label route:storefront \
  -template /etc/nginx/upstreams.conf.tmpl \
  -output /etc/nginx/conf.d/default.conf \
  -reload-cmd 'nginx -s reload'
```

Use a narrow namespace and trusted image because `api_access` injects an API credential. Health checks are essential: without them, the proxy cannot distinguish a newly started but unready backend. During a control-plane interruption the last rendered configuration remains in place, so choose proxy timeouts and retry behavior accordingly.

## Perform a rolling update

**Outcome:** replace replicas gradually while retaining healthy service capacity.

Configure multiple replicas, a meaningful health check, and a rolling strategy:

```yaml
count: 3
update:
  strategy: rolling
  max_parallel: 1
tasks:
  - name: web
    image: registry.example.com/storefront:v2
    health_check:
      type: http
      port: 8080
      path: /ready
      interval: 5s
      timeout: 2s
      threshold: 2
```

Apply the changed manifest with `trellis jobs apply --file trellis.yaml`. Trellis marks old-revision allocations as draining, starts at most `max_parallel` replacements that are not yet healthy, and stops old allocations as healthy replacements make them surplus.

This pattern needs spare cluster capacity because old and new allocations overlap. `max_parallel` limits concurrency, not a percentage of replicas. A weak readiness check can promote a broken release; a check that depends on an unavailable downstream can stall the rollout. Inspect progress with `trellis jobs status NAME` and allocation events. To roll back, restore the earlier image/configuration and apply again as a new revision.

A complete starting manifest is available at [`examples/deployment-strategies/rolling.yaml`](../../examples/deployment-strategies/rolling.yaml).

## Switch releases with blue/green deployment

**Outcome:** validate a complete new release before moving production traffic, with a fast routing rollback.

Trellis has no `blue_green` strategy keyword. Model blue and green as two independently named jobs:

1. Deploy `storefront-blue` with `route: storefront-blue` and leave the production proxy on that label.
2. Deploy `storefront-green` with `route: storefront-green`.
3. Exercise green directly, including its health, migrations, and external dependencies.
4. Change the proxy synchronizer's label from `route:storefront-blue` to `route:storefront-green` and reload it.
5. Keep blue temporarily for rollback; delete it only after the observation window.

The separate job names prevent one apply from replacing the other color. They also double capacity during the overlap. Database changes must be backward-compatible with both colors until blue is retired. Routing state lives in the proxy configuration, not in the Trellis scheduler, so store and review that change like application code.

See [`blue.yaml`](../../examples/deployment-strategies/blue.yaml) and [`green.yaml`](../../examples/deployment-strategies/green.yaml) for the two-job shape.

## Send a small share of traffic to a canary

**Outcome:** expose a new release to limited traffic while the stable release continues serving most requests.

Run stable and canary as separate jobs with the same route label and different weights:

```yaml
# stable task-group labels
labels:
  route: storefront-weighted
  track: stable
  trellis/weight: "100"
```

```yaml
# canary task-group labels
labels:
  route: storefront-weighted
  track: canary
  trellis/weight: "5"
```

Point `trellis-proxy-sync` at `route:storefront-weighted`. It passes each positive `trellis/weight` value to the proxy template. Start with one canary replica, compare error rate/latency/business metrics by `track`, then increase its weight or replica count. Delete the canary job to remove it from discovery; promote it by deploying the new version as stable and retiring the old stable release.

Weights are per discovered allocation. Four stable allocations at weight 100 and one canary at weight 5 do **not** yield the same percentage as one stable allocation at weight 100 and one canary at weight 5. Calculate the effective pool and confirm how the chosen proxy interprets weights. Sticky sessions and long-lived connections further affect observed traffic share.

See [`stable.yaml`](../../examples/deployment-strategies/stable.yaml) and [`canary.yaml`](../../examples/deployment-strategies/canary.yaml).

## Run a sidecar beside every application replica

**Outcome:** colocate a helper—metrics exporter, log forwarder, local proxy, or configuration watcher—with each application replica.

Put both tasks in the same task group. Trellis places and scales the group as a unit, so `count: 3` produces three allocations containing both tasks:

```yaml
task_groups:
  - name: web
    count: 3
    tasks:
      - name: app
        image: registry.example.com/app:v1
      - name: metrics
        image: registry.example.com/exporter:v1
```

Use loopback only when the selected network mode and runtime configuration make it appropriate; otherwise communicate through declared endpoints. Give the helper its own resource request and health check when its readiness matters. A failing task can affect the allocation's lifecycle, so do not colocate unrelated services merely to reduce manifest count.

The [`sidecar` manifest](../../examples/sidecar/trellis.yaml) expands this pattern with ports, health checks, and resource reservations.

## Rotate a credential without placing it in a manifest

**Outcome:** deliver credentials to containers while keeping plaintext out of Git and job manifests.

Create the value through standard input or a protected file:

```sh
printf %s "$NEW_PASSWORD" | \
  trellis --namespace payments secrets set database-password --stdin
```

Reference only the secret name. Prefer a file target when the application supports it:

```yaml
secrets:
  - name: database-password
    target: file
    path: /run/trellis-secrets/database-password
    mode: 256 # decimal representation of 0400
```

For safe concurrent rotation, read the metadata version and write with `--expected-version N`. Secret updates affect allocations started afterward; existing containers retain their delivered value. Apply a workload revision or otherwise replace consumers in an order compatible with the old and new credential. Deleting metadata also does not erase values already delivered to running containers.

Keep the node's 32-byte secrets encryption key outside the Trellis data directory and back it up separately. A desired-state backup contains encrypted records but cannot recover them without that key. The full environment/file mapping pattern is in [`examples/secrets/trellis.yaml`](../../examples/secrets/trellis.yaml).

## Keep data across allocation replacement

**Outcome:** retain application data when a container or allocation is recreated.

Use `host_volume` to require operator-provisioned node storage:

```yaml
constraints:
  - attribute: storage
    value: fast
tasks:
  - name: database
    image: registry.example.com/database:v1
    volumes:
      - name: data
        path: /var/lib/database
        host_volume: database-data
```

Provision matching nodes with the `storage=fast` label and advertise the `database-data` volume name. The scheduler excludes every node that lacks it. An unnamed volume is allocation-local scratch space and should not hold irreplaceable state.

A host volume is a placement capability, not distributed storage: Trellis does not replicate, snapshot, or restore its contents. If several nodes advertise the same name, each may still contain unrelated local data. Pair the pattern with application replication or an external storage system, backups, restore drills, and deliberate node constraints. See [`examples/volumes/trellis.yaml`](../../examples/volumes/trellis.yaml).

## Let a workload discover its namespace

**Outcome:** allow a trusted controller, proxy, or automation task to query jobs and allocations without embedding a cluster administrator token.

Set `api_access: true` on its task group. Trellis injects `TRELLIS_ADDR`, `TRELLIS_TOKEN`, and `TRELLIS_NAMESPACE`. Call the API with both authentication and namespace scope:

```sh
curl --fail --silent --show-error \
  -H "Authorization: Bearer $TRELLIS_TOKEN" \
  -H "X-Trellis-Namespace: $TRELLIS_NAMESPACE" \
  "http://$TRELLIS_ADDR/v1/allocations?label=route:storefront"
```

Treat all tasks in that group as holders of the credential. Do not print the environment, send the token to a browser, or enable API access on third-party images without review. Make controllers tolerate retries, non-2xx responses, and eventual reconciliation. The helper in [`examples/api-access/list-jobs.sh`](../../examples/api-access/list-jobs.sh) demonstrates the request shape.

## Assemble a small stateful web stack

**Outcome:** colocate a web application and its database for a compact development or demonstration deployment.

Place both tasks in one group, explicitly give both tasks host networking so they can communicate over node loopback, inject database passwords from Trellis secrets, attach separate host volumes, and put the public route label only on the group. The [`WordPress manifest`](../../examples/wordpress/trellis.yaml) demonstrates that composition.

This trades simplicity for coupled placement, scaling, updates, and failure. Scaling the group also creates another database, which is usually incorrect. For production, separate the database behind its own replication/failover design or use a managed service; then scale only stateless web replicas.

## Schedule a replicated database without confusing scheduling with consensus

**Outcome:** place and monitor several database members while leaving replication and leader election to the database system.

A task group with `count: 3`, host-volume requirements, anti-concentration from normal scheduling, API discovery, and a database-native health endpoint provides the container layer. Patroni still needs a supported distributed configuration store, unique member identity, PostgreSQL replication, fencing, backups, and recovery procedures. Trellis service discovery is not Patroni consensus.

The [`Patroni example`](../../examples/patroni/) deliberately documents the remaining operator work. Use it as an architecture skeleton, not a turnkey HA database.

## Choose the right pattern

| Desired outcome | Pattern | Primary tradeoff |
|---|---|---|
| Stable ingress for changing replicas | Label-driven reverse proxy | Trusted API-aware controller required |
| Gradual in-place replacement | Rolling update | Requires spare capacity and reliable readiness |
| Instant route switch and rollback | Blue/green | Roughly doubles deployment capacity |
| Limited real-traffic exposure | Weighted canary | Effective weight depends on replica count/proxy behavior |
| Per-replica helper | Sidecar task in one group | Coupled lifecycle and placement |
| Credential delivery and rotation | Versioned Trellis secret | Running allocations require replacement to receive changes |
| Node-local persistence | Advertised host volume | No built-in replication or backup |
| In-cluster automation | Namespace-scoped API access | Workload becomes a credential holder |
| Replicated database | Scheduler plus database-native HA | Trellis does not replace database consensus |

[Documentation index](../README.md) · [Previous: Operations](operations.md) · [Next: Dashboard](dashboard.md)
