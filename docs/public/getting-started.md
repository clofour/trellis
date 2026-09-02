# Getting Started

This is the shortest complete Trellis journey: install one node, connect the CLI, deploy a deliberately small workload, inspect and update it, read its logs, and remove it again. You do not need to clone or build the repository.

## 1. Install one node

You need a Debian or Ubuntu x86-64 machine with `sudo`. The installer can install containerd when it is missing.

```sh
curl -fsSL https://raw.githubusercontent.com/clofour/trellis-experimental/main/scripts/setup.sh | sudo bash
```

For this first cluster, accept the detected address, do not join another cluster, and start `trellis` when prompted. WireGuard and the dashboard are optional and are not needed for the first workload.

Verify the service and cluster:

```sh
sudo systemctl status trellis --no-pager
sudo trellisctl nodes list
```

The installer puts `trellis` and `trellisctl` in `/usr/local/bin`, creates the systemd service, and writes a root-readable local connection file containing the node API address, token, and CA certificate.

## 2. Save the local connection

Save that effective connection as a named context so the cluster and namespace are visible and reusable:

```sh
sudo trellisctl --namespace default context save local --use
sudo trellisctl context current
```

These commands use `sudo` because the installer's local connection file and cluster credential are root-readable. For a remote workstation or a non-root context, configure an endpoint and credential explicitly as described in [CLI workflows](cli.md).

## 3. Create the first manifest

Create an empty working directory and save this as `trellis.yaml`:

```yaml
name: hello
namespace: default
task_groups:
  - name: web
    count: 1
    tasks:
      - name: nginx
        image: docker.io/library/nginx:1.27-alpine
        env:
          TUTORIAL_STEP: first
        resources:
          cpu: 100
          memory: 64MiB
```

This is one job containing one task group, one desired allocation, and one nginx task. It intentionally has no networking, explicit health check, volume, secret, or update policy yet. A running task without an explicit health check is considered healthy.

The same file is maintained at [`examples/hello/trellis.yaml`](../../examples/hello/trellis.yaml).

## 4. Validate, preview, and deploy

Validation is local. Diff compares the manifest with current cluster state without changing it. Apply creates the job and waits for its current revision to become healthy.

```sh
sudo trellisctl jobs validate --file trellis.yaml
sudo trellisctl jobs diff --file trellis.yaml
sudo trellisctl jobs apply --file trellis.yaml --wait
```

## 5. Inspect the job

Start with the job, then use its allocations only when you need runtime detail:

```sh
sudo trellisctl jobs list
sudo trellisctl jobs status hello
```

The output separates desired capacity from allocation lifecycle and health. If the job is not ready, ask Trellis for the failure-oriented view:

```sh
sudo trellisctl jobs diagnose hello
```

## 6. Update it

Change `TUTORIAL_STEP: first` to `TUTORIAL_STEP: second` in `trellis.yaml`, then preview and apply the new revision:

```sh
sudo trellisctl jobs diff --file trellis.yaml
sudo trellisctl jobs apply --file trellis.yaml --wait
sudo trellisctl jobs status hello
```

The environment value is only a visible tutorial change; Trellis still replaces the old allocation because the desired task configuration changed.

## 7. Read logs

Read the job's current task output without copying an allocation UUID:

```sh
sudo trellisctl jobs logs hello --tail 100
```

When a job has several allocations or tasks, use `--group`, `--task`, or the short `--allocation` reference shown by `jobs status`.

## 8. Remove it

Delete the desired state and wait until the job has disappeared:

```sh
sudo trellisctl jobs delete hello --wait
sudo trellisctl jobs list
```

You have now completed the full workload lifecycle.

## Optional: follow the same job in the dashboard

If you installed the read-write dashboard, open `http://NODE_ADDRESS:3000`. Confirm that the active namespace is `default`, use **Apply Manifest** to paste the same YAML, review the semantic plan, then apply it. Open the job to inspect its revision, allocations, events, and task logs. Editing and deleting use the same manifest vocabulary and lifecycle as `trellisctl`. A read-only dashboard intentionally hides write actions.

## Troubleshooting

- `sudo journalctl -u trellis -n 200` shows node and control-plane service logs.
- `sudo trellisctl jobs diagnose hello` summarizes placement, start, retry, and health failures.
- Image-pull failures usually mean the node cannot reach the registry or the image/tag is unavailable.
- A context or authentication error can be checked with `sudo trellisctl context current` and `sudo trellisctl nodes list`.

Continue with the [deliberate learning path](learning-path.md). It introduces replicated services and health checks first, then secrets, volumes, sidecars, networking, in-cluster API access, release patterns, and finally stateful architectures such as WordPress and Patroni. To build Trellis itself, use the separate [developer setup](../developer/development.md).

[Documentation index](../README.md) · [Next: Learning path](learning-path.md)
