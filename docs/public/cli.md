# CLI workflows

The `trellis` CLI is the first-party operator interface to the [Trellis user model](user-model.md). Resource commands remain available, but routine usage is organized around a small workflow: select a cluster context, validate and plan desired state, apply it, observe convergence, diagnose failures, inspect logs, and delete the job when it is no longer desired.

## Named cluster contexts

A **context** stores the connection information needed to operate one cluster/namespace: API address, cluster token, namespace, CA certificate, and optional client certificate/key paths.

Save the effective connection and select it:

```sh
export TRELLIS_TOKEN='replace-me'
trellis --server-addr trellis.example:8128 \
  --namespace production \
  --ca-cert ./cluster-ca.pem \
  context save production --use
```

After that, ordinary commands need no connection flags:

```sh
trellis context current
trellis nodes list
trellis jobs list
```

Useful context commands:

```sh
trellis context list
trellis context show production
trellis context use staging
trellis --context production jobs list   # one command only
trellis context delete old-cluster
```

`context show` never prints the stored token. The user config is written with mode `0600` because saved contexts can contain credentials.

Existing flat config files remain valid. Effective connection precedence is:

```text
local node run file
< legacy flat config values
< selected named context
< TRELLIS_* environment variables
< explicit command-line flags
```

The selected context itself comes from `current_context`, then `TRELLIS_CONTEXT`, then the explicit `--context` flag. This preserves script compatibility while making named contexts the convenient interactive path.

## Validate and plan a manifest

Validation is local and does not modify the cluster:

```sh
trellis jobs validate --file trellis.yaml
```

Preview what would change compared with the current job:

```sh
trellis jobs diff --file trellis.yaml
# `jobs plan` is an alias
trellis jobs plan --file trellis.yaml
```

The diff is semantic rather than a textual YAML diff. Task groups and tasks are identified by name, so reformatting a manifest does not look like a deployment. Example output:

```text
Plan: update production/web from revision 7
  ~ task_groups[frontend].tasks[app].image: "registry.example/app:v7" -> "registry.example/app:v8"
  ~ task_groups[frontend].update.max_parallel: 1 -> 2
```

`trellis jobs apply --dry-run --file trellis.yaml` uses the same planner when a CI/CD workflow wants one apply-shaped command for preview and execution.

## Apply and observe convergence

A normal apply reports whether it created a job, changed its revision, or was already up to date:

```sh
trellis jobs apply --file trellis.yaml
```

To make deployment completion part of the command result, wait for desired capacity to become healthy:

```sh
trellis jobs apply --file trellis.yaml --wait --timeout 5m
```

The command prints only meaningful state changes while the revision converges. The same observer is available separately:

```sh
trellis jobs watch web --timeout 5m
```

A job is reported as `ready` when at least its desired allocation count is running and healthy. `converging` means Trellis is still placing/starting/replacing work. `degraded` means an allocation explicitly reports an unhealthy, failed, or lost state.

## Inspect and diagnose

Start at the job level:

```sh
trellis jobs list
trellis jobs status web
```

Table output shows short allocation references and node addresses instead of requiring full internal UUIDs. Full IDs and API fields remain available through `--output json`.

When a job is not healthy, ask for the failure-oriented view:

```sh
trellis jobs diagnose web
```

`diagnose` surfaces allocation lifecycle/health, reason codes, human-readable messages, retry timing, and attempt count. It intentionally omits normal healthy allocations and old draining allocations unless they report a real problem.

## Read logs without copying allocation IDs

For non-following output, a job name is enough. If several allocations match, Trellis prints each log stream with a short header:

```sh
trellis jobs logs web --tail 200
```

Filter by task group or task when appropriate:

```sh
trellis jobs logs web --group frontend
trellis jobs logs web --task app
```

Following needs one stream. Select it using the short prefix displayed by `jobs status`:

```sh
trellis jobs logs web --allocation a1b2c3d4 --follow
```

Passing a full allocation ID directly is still supported for compatibility, and a unique allocation prefix also works.

## Delete and wait for removal

```sh
trellis jobs delete web
```

Use `--wait` when a script should not continue until the job resource has disappeared:

```sh
trellis jobs delete web --wait --timeout 2m
```

`jobs destroy` remains an alias for compatibility; documentation uses `delete`.

## Node maintenance without UUID copying

`nodes list` puts the human-meaningful address first and shows a short ID. Drain, undrain, and remove accept any of:

- the node host, such as `worker-2`
- the displayed address, such as `worker-2:8128`
- a unique UUID prefix
- the complete UUID

For example:

```sh
trellis nodes drain worker-2
trellis nodes undrain worker-2
trellis nodes remove 9cf13a2b
```

Ambiguous prefixes are rejected and the CLI shows the matching nodes rather than guessing.

## Automation and compatibility

`--output json` continues to expose complete API-shaped data for scripts. Explicit `--server-addr`, `--cluster-token`, `--namespace`, TLS flags, and `TRELLIS_*` environment variables remain supported and override saved context values. Named contexts are therefore an interactive convenience, not a hidden requirement for automation.
