# Hello Trellis

**Level:** Beginner · **Prerequisites:** one healthy node and a `trellisctl` context in the `default` namespace

This is the deliberately small first workload used by [Getting Started](../../docs/public/getting-started.md). It contains one job, one task group, one replica, and one nginx task. There are no ports, health-check settings, volumes, secrets, or update-strategy settings to understand yet.

Trellis treats a running task without an explicit health check as healthy. Later examples replace that default with application-aware readiness.

## Complete the workload lifecycle

From the repository root:

```sh
trellisctl jobs validate --file examples/hello/trellis.yaml
trellisctl jobs diff --file examples/hello/trellis.yaml
trellisctl jobs apply --file examples/hello/trellis.yaml --wait
trellisctl jobs status hello
trellisctl jobs logs hello --tail 100
```

To create a visible second revision, change `TUTORIAL_STEP: first` to `TUTORIAL_STEP: second`, then preview and apply it:

```sh
trellisctl jobs diff --file examples/hello/trellis.yaml
trellisctl jobs apply --file examples/hello/trellis.yaml --wait
trellisctl jobs status hello
```

Remove the desired state and wait for the job to disappear:

```sh
trellisctl jobs delete hello --wait
```

Next, continue to the [`web-service` example](../web-service/) to add host networking, a fixed host port, an HTTP health check, multiple replicas, and rolling replacement.
