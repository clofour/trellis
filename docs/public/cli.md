# CLI workflows

The `trellisctl` CLI is the first-party operator interface to the [Trellis user model](user-model.md). Resource commands remain available, but routine usage is organized around a small workflow: select a cluster context, validate and plan desired state, apply it, observe convergence, diagnose failures, inspect logs, and delete the job when it is no longer desired.

## Named cluster contexts

A **context** stores the connection information needed to operate one cluster/namespace: API address, cluster token, namespace, CA certificate, and optional client certificate/key paths.

Save the effective connection and select it:

```sh
export TRELLIS_TOKEN='replace-me'
trellisctl --server-addr trellis.example:8128 \
  --namespace production \
  --ca-cert ./cluster-ca.pem \
  context save production --use
```

After that, ordinary commands need no connection flags:

```sh
trellisctl context current
trellisctl nodes list
trellisctl jobs list
```

Useful context commands:

```sh
trellisctl context list
trellisctl context show production
trellisctl context use staging
trellisctl --context production jobs list   # one command only
trellisctl context delete old-cluster
```

`context show` never prints the stored token. The user config is written with mode `0600` because saved contexts can contain credentials.

Effective connection precedence is:

```text
local node run file
< selected named context
< TRELLIS_* environment variables
< explicit command-line flags
```

The selected context itself comes from `current_context`, then `TRELLIS_CONTEXT`, then the explicit `--context` flag.

## Validate and plan a manifest

Validation is local and does not modify the cluster:

```sh
trellisctl jobs validate --file trellis.yaml
```

Preview what would change compared with the current job:

```sh
trellisctl jobs diff --file trellis.yaml
# `jobs plan` is an alias
trellisctl jobs plan --file trellis.yaml
```

The diff is semantic rather than a textual YAML diff. Task groups and tasks are identified by name, so reformatting a manifest does not look like a deployment. Example output:

```text
Plan: update production/web from revision 7
  ~ task_groups[frontend].tasks[app].image: "registry.example/app:v7" -> "registry.example/app:v8"
  ~ task_groups[frontend].update.max_parallel: 1 -> 2
```

`trellisctl jobs apply --dry-run --file trellis.yaml` uses the same planner when a CI/CD workflow wants one apply-shaped command for preview and execution.

## Apply and observe convergence

A normal apply reports whether it created a job, changed its revision, or was already up to date:

```sh
trellisctl jobs apply --file trellis.yaml
```

To make deployment completion part of the command result, wait for desired capacity to become healthy:

```sh
trellisctl jobs apply --file trellis.yaml --wait --timeout 5m
```

The command prints only meaningful state changes while the revision converges. The same observer is available separately:

```sh
trellisctl jobs watch web --timeout 5m
```

A job is reported as `ready` when at least its desired allocation count from the current revision is running and healthy. Old or draining allocations cannot make a new revision look complete. `converging` means Trellis is still placing, starting, or replacing work. `degraded` means a current allocation explicitly reports an unhealthy, failed, or lost state.

## Inspect and diagnose

Start at the job level:

```sh
trellisctl jobs list
trellisctl jobs status web
```

Table output shows short allocation references and node addresses instead of requiring full internal UUIDs. Full IDs and API fields remain available through `--output json` on commands that expose structured output.

When a job is not healthy, ask for the failure-oriented view:

```sh
trellisctl jobs diagnose web
```

`diagnose` surfaces allocation lifecycle/health, reason codes, human-readable messages, retry timing, and attempt count. It intentionally omits normal healthy allocations and old draining allocations unless they report a real problem.

## Read logs by job, allocation, group, or task

For non-following output, a job name is enough. Trellis prints every matching task stream, with headers when more than one stream matches:

```sh
trellisctl jobs logs web --tail 200
```

Narrow the streams by task group or task when appropriate:

```sh
trellisctl jobs logs web --group frontend
trellisctl jobs logs web --task app
```

Following needs exactly one task stream. Combine the short allocation reference displayed by `jobs status` with a task selector when a task group has multiple tasks:

```sh
trellisctl jobs logs web --allocation a1b2c3d4 --task app --follow
```

## Delete and wait for removal

```sh
trellisctl jobs delete web
```

Use `--wait` when a script should not continue until the job resource has disappeared:

```sh
trellisctl jobs delete web --wait --timeout 2m
```

## Inspect and maintain nodes without UUID copying

`nodes list` puts the human-meaningful address first, shows a short ID, and formats CPU/memory for humans. When placement depends on a node's labels or advertised host volumes, inspect that node directly:

```sh
trellisctl nodes status worker-2
```

`nodes status` shows the full ID, scheduling state, version, CPU/memory capacity, last heartbeat, labels, and advertised host-volume names. Add `--output json` when automation needs the API representation.

Node references for status, drain, undrain, and remove may be any of:

- the node host, such as `worker-2`
- the displayed address, such as `worker-2:8127`
- a unique UUID prefix
- the complete UUID

For example:

```sh
trellisctl nodes status worker-2
trellisctl nodes drain worker-2
trellisctl nodes undrain worker-2
trellisctl nodes remove 9cf13a2b
```

Ambiguous prefixes are rejected and the CLI shows the matching nodes rather than guessing.

## Structured output and automation

`--output` / `-o` is deliberately **command-local**, not a global promise. Commands that can return one coherent structured result expose `--output table|json`; streaming or action-oriented commands do not expose the flag and therefore cannot silently ignore `--output json` while printing prose.

Current structured-output commands are:

```text
jobs validate, diff/plan, list, status, diagnose
nodes list, status
secrets set, list, describe
credentials create
```

For example:

```sh
trellisctl jobs status web --output json
trellisctl nodes status worker-2 -o json
```

`jobs logs` remains a log byte stream, while `jobs apply`, `jobs watch`, `jobs delete`, node mutation commands, backup operations, and context mutation commands remain human/action workflows rather than pretending to produce a stable JSON document.

Explicit `--server-addr`, `--token`, `--namespace`, TLS flags, and `TRELLIS_*` environment variables override saved context values. Named contexts are therefore an interactive convenience, not a hidden requirement for automation.

[Documentation index](../README.md) · [Previous: Job manifest reference](job-specification.md) · [Next: Operations](operations.md)
