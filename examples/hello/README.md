# Hello Trellis

**Level:** Beginner · **Prerequisites:** one healthy node and a CLI context in the `default` namespace

This is the deliberately small first workload used by [Getting Started](../../docs/public/getting-started.md). It contains one job, one task group, one replica, and one nginx task. There are no ports, health-check settings, volumes, secrets, or update-strategy settings to understand yet.

Trellis treats a running task without an explicit health check as healthy. Later examples replace that default with application-aware readiness.

## Complete the workload lifecycle

From the repository root:

```sh
trellis jobs validate --file examples/hello/trellis.yaml
trellis jobs diff --file examples/hello/trellis.yaml
trellis jobs apply --file examples/hello/trellis.yaml --wait
trellis jobs status hello
trellis jobs logs hello --tail 100
```

To create a visible second revision, change `TUTORIAL_STEP: first` to `TUTORIAL_STEP: second`, then preview and apply it:

```sh
trellis jobs diff --file examples/hello/trellis.yaml
trellis jobs apply --file examples/hello/trellis.yaml --wait
trellis jobs status hello
```

Remove the desired state and wait for the job to disappear:

```sh
trellis jobs delete hello --wait
```

Next, continue to the [`web-service` example](../web-service/) to add host networking, a dynamic port, an HTTP health check, multiple replicas, and rolling replacement.
