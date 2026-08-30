# Secrets

This example shows how to store secrets in Trellis and inject them into
containers at runtime.

## How it works

Trellis stores secrets per namespace, encrypted at rest with AES-256-GCM.
A job manifest references secrets by name; the scheduler decrypts them at
allocation time and injects them into the container either as environment
variables or as files mounted inside the container.

Secrets are write-only through the API — you cannot read a stored secret value
back out, only update or delete it. This limits blast radius if the control
plane is compromised.

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
| `path` | if target=file | Absolute path inside the container |
| `mode` | no | File permission bits (default `0400`) |

## Deploying

### Store the secrets

Write each secret before applying the job. The CLI reads the value from stdin:

```sh
echo -n "s3cr3t-p4ssw0rd" | trellis --namespace acme secrets set db-password
```

To write from a file:

```sh
trellis --namespace acme secrets set tls-cert < ./tls.crt
```

### Apply the job

```sh
trellis --namespace acme jobs apply --file app.yaml
```

The scheduler resolves the secret references when placing allocations. If a
referenced secret does not exist, the allocation fails to start.

### Rotate a secret

Write a new value to the same secret name:

```sh
echo -n "n3w-p4ssw0rd" | trellis --namespace acme secrets set db-password
```

The scheduler does not automatically restart running allocations on a secret
rotation. To pick up the new value, redeploy the job:

```sh
trellis --namespace acme jobs apply --file app.yaml
```

### Optimistic concurrency

To prevent lost updates when multiple operators rotate the same secret
concurrently, pass `--version` with the current version number. The write is
rejected if the stored version has changed since you last read it:

```sh
# Read the current version first.
trellis --namespace acme secrets describe db-password
# => version: 4

echo -n "n3w-p4ssw0rd" | trellis --namespace acme secrets set --version 4 db-password
```

### List and delete secrets

```sh
trellis --namespace acme secrets list
trellis --namespace acme secrets delete db-password
```

## Security notes

- Secrets are namespace-scoped — a job in `acme` cannot access secrets from
  `other-namespace`.
- Tasks without a `secrets:` reference never receive the values.
- File-target secrets are written to the container's private filesystem; they
  are not visible to other tasks in the task group.
- Trellis does not log secret values.
