# Getting Started

This is the shortest complete Trellis journey: install one node, use the CLI as your normal user, deploy a deliberately small workload, inspect and update it, read its logs, and remove it again. You do not need to clone or build the repository.

## 1. Install one node

You need a Debian or Ubuntu x86-64 machine with `sudo`. The installer can install containerd when it is missing.

```sh
curl -fsSL https://raw.githubusercontent.com/clofour/trellis-experimental/main/scripts/setup.sh | sudo bash
```

For this first cluster, accept the detected address and do not join another cluster. The dashboard and WireGuard are optional and are not needed for the first workload.

The installer uses a root-only bootstrap credential for node/cluster bootstrap, then mints a normal `cluster/write` operator credential and saves a `local` context for the user who invoked `sudo`. Routine `trellisctl` commands therefore do **not** need `sudo` and do not receive the bootstrap credential.

Verify the service and saved context:

```sh
sudo systemctl status trellis --no-pager
trellisctl context current
trellisctl nodes list
```

`trellis` and `trellisctl` are installed in `/usr/local/bin`. The daemon keeps its bootstrap credential root-readable under `/etc/trellis`; your user context contains the scoped operator token plus the cluster CA.

## 2. Create the first manifest

Create an empty working directory and save this as `trellis.yaml`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/clofour/trellis-experimental/main/schemas/trellis-job.schema.json
name: hello
namespace: default
task_groups:
  - name: web
    count: 1
    tasks:
      - name: hello
        image: ghcr.io/clofour/trellis-tutorial:v1
        resources:
          cpu: 100
          memory: 64MiB
```

This is one job containing one task group, one desired allocation, and one task. It intentionally has no networking, explicit health check, volume, secret, API access, or update policy yet. A running task without an explicit health check is considered healthy.

The image is a tiny first-party tutorial workload. It stays running and emits a recognizable `Trellis tutorial v1` log line, so the first deployment has something concrete to inspect. The same file is maintained at [`examples/hello/trellis.yaml`](../../examples/hello/trellis.yaml).

## 3. Validate, preview, and deploy

```sh
trellisctl jobs validate --file trellis.yaml
trellisctl jobs diff --file trellis.yaml
trellisctl jobs apply --file trellis.yaml --wait
```

Validation parses the human YAML locally. Planning and apply use canonical JSON and Trellis's server-owned semantics.

## 4. Inspect the job

```sh
trellisctl jobs list
trellisctl jobs status hello
trellisctl jobs logs hello --tail 100
```

You should see the tutorial v1 startup/log message. If the job is not ready, ask for the failure-oriented view:

```sh
trellisctl jobs diagnose hello
```

## 5. Update it

Change only the image tag:

```yaml
image: ghcr.io/clofour/trellis-tutorial:v2
```

Then preview and apply the new revision:

```sh
trellisctl jobs diff --file trellis.yaml
trellisctl jobs apply --file trellis.yaml --wait
trellisctl jobs status hello
trellisctl jobs logs hello --tail 100
```

The semantic plan shows the image change, Trellis replaces the allocation, and the new logs identify tutorial v2. This makes the first update visible both before and after it happens.

## 6. Remove it

```sh
trellisctl jobs delete hello --wait
trellisctl jobs list
```

You have now completed the full workload lifecycle: install → connect → deploy → inspect → update → logs → remove.

## Optional: dashboard

If you installed the dashboard, open `http://NODE_ADDRESS:3000`. The default installer mode uses a real `cluster/read` credential, not the bootstrap token. Read-write mode instead uses `cluster/write` and enables mutation controls. In either mode the dashboard stays close to `trellisctl`: it edits the same YAML, asks the control plane for the same semantic plan, and exposes Trellis resources rather than adding application-platform abstractions.

## Troubleshooting

- `sudo journalctl -u trellis -n 200` shows daemon/control-plane logs.
- `trellisctl jobs diagnose hello` summarizes placement, start, retry, and health failures.
- Image-pull failures usually mean the node cannot reach GHCR or the image/tag is unavailable.
- `trellisctl context current` and `trellisctl nodes list` verify the saved operator connection.

Continue with the [learning path](learning-path.md). It reuses the tutorial application and adds concepts one at a time: first host networking and `/health`, then replicas and placement, then rolling-update overlap before moving on to secrets, volumes, sidecars, namespace networking, API access, release patterns, and stateful architectures.

[Documentation index](../README.md) · [Next: Learning path](learning-path.md)
