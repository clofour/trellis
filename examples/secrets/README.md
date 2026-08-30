# Secrets

This example shows how to store secrets in Trellis and inject them into
containers at runtime.

## How it works

Trellis stores secrets per namespace, encrypted at rest with AES-256-GCM.
A job manifest references secrets by name; the scheduler decrypts them at
allocation time and injects them into the container either as environment
variables or as files mounted inside the container.

Secret-management API responses expose metadata, not the stored secret value.
Values are delivered only to tasks that explicitly reference the secret.

## Manifests

`app.yaml` declares two secret references on the `server` task:

```yaml
secrets:
  - name: db-password
    target: env
    env: DB_PASSWORD

  - name: tls-cert
    target: file
    path: /run/trellis-secrets/tls.crt
    mode: 0400
```

`db-password` is injected as `$DB_PASSWORD`. `tls-cert` is written to
`/run/trellis-secrets/tls.crt` with mode `0400` (owner-read only).

## Secret reference fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Name of the secret in the namespace |
| `target` | yes | `env` or `file` |
| `env` | if target=env | Environment variable name |
| `path` | if target=file | Absolute path inside the container, below `/run/trellis-secrets/` |
| `mode` | no | `0400` or `0600` (default `0400`) |

## Deploying

### Store the secrets

Write each secret before applying the job. Use `--stdin` when piping the value:

```sh
printf %s "s3cr3t-p4ssw0rd" | trellis --namespace acme secrets set db-password --stdin
```

To write from a file:

```sh
trellis --namespace acme secrets set tls-cert --file ./tls.crt
```

### Apply the job

```sh
trellis --namespace acme jobs apply --file app.yaml
```

The scheduler resolves the secret references when an allocation starts. If a
referenced secret does not exist, the allocation fails to start.

### Rotate a secret

Write a new value to the same secret name:

```sh
printf %s "n3w-p4ssw0rd" | trellis --namespace acme secrets set db-password --stdin
```

Updating the stored value does **not** modify or restart allocations that are
already running. Re-applying an otherwise unchanged manifest also does not
create a new execution revision, so it is not sufficient by itself to deliver
the new value.

To roll the new secret into the workload, make an execution-affecting manifest
change (for example, increment a benign environment value such as
`SECRET_REVISION`) and apply the manifest. New allocations resolve the current
secret value when they start. If an outage is acceptable, destroying and
re-applying the job also starts fresh allocations.

### Optimistic concurrency

To prevent lost updates when multiple operators rotate the same secret
concurrently, pass `--expected-version` with the current version number. The
write is rejected if the stored version changed since you last read the
metadata:

```sh
# Read the current version first.
trellis --namespace acme secrets describe db-password
# => Version: 4

printf %s "n3w-p4ssw0rd" | \
  trellis --namespace acme secrets set db-password --stdin --expected-version 4
```

Passing `--expected-version 0` performs a create-only write.

### List and delete secrets

```sh
trellis --namespace acme secrets list
trellis --namespace acme secrets delete db-password
```

## Security notes

- Secret management requires the cluster-authorized CLI credential; the
  namespace token injected by `api_access: true` is not a secret-management
  credential.
- Tasks without a `secrets:` reference never receive the value.
- File-target secrets are materialized in a memory-backed host directory and
  bind-mounted read-only into the target task.
- Trellis does not intentionally include secret values in its API responses or
  log messages.
