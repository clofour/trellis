# Secrets design

> **Status: implemented (initial version).** Secret storage is enabled only
> when every node is configured with `--secrets-key`. The implementation covers
> write-only APIs, encrypted Raft persistence, CLI management, and allocation
> delivery. The rollout criteria remain the operational security contract.

## Purpose and boundaries

Trellis secrets are an opaque, namespace-scoped sink and delivery mechanism
for static byte strings. A user or external system writes a value, a job names
it, and Trellis makes the value available only to the allocations that need
it. Trellis does not interpret the value and never returns it through its API.

This primitive is deliberately **not** a secrets-provider abstraction. Trellis
will not contain integrations for Vault, AWS Secrets Manager, 1Password, or
similar systems. Those systems may push static values through the Trellis API
or CLI. Dynamic credentials, renewable leases, revocation, and provider login
remain under the provider and application control; copying leased credentials
into Trellis would discard those guarantees and is not supported.

The initial version has these non-goals:

- generating credentials or choosing their format;
- fetching, renewing, or revoking provider-managed credentials;
- templating or interpolating secret contents;
- exposing secret values in the dashboard, API, CLI, job status, or events;
- injecting secrets into command-line arguments; and
- providing cross-namespace references or cluster-global secrets.

## User model

### Identity and metadata

A secret is identified by `(namespace, name)`. Names follow the existing
Trellis resource-name rules. Its durable metadata is:

| Field | Meaning |
| --- | --- |
| `namespace`, `name` | Immutable identity. |
| `version` | Monotonically increasing integer, starting at 1. |
| `created_at`, `updated_at` | Server timestamps. |
| `created_by`, `updated_by` | Authenticated principal identifier, when available. Never a submitted free-form value. |
| `ciphertext_size` | Optional operational size information; it is not the plaintext. |
| `key_id` | Operator-visible encryption key identifier, not key material. |

Labels and arbitrary descriptions are excluded initially because users tend to
put sensitive information in them. Secret values are limited to 64 KiB after
decoding. Empty values are rejected.

### CLI

The value is accepted from standard input or a file, never a positional
argument (which commonly leaks through shell history and process listings):

```sh
printf '%s' "$DATABASE_PASSWORD" | trellis --namespace acme secrets set db-password --stdin
trellis --namespace acme secrets set tls-key --file ./tls.key
trellis --namespace acme secrets list
trellis --namespace acme secrets describe db-password
trellis --namespace acme secrets delete db-password
```

`set` creates or atomically replaces the value and returns metadata. It never
echoes the value. `--stdin` refuses a terminal unless `--interactive` is also
specified; interactive entry disables echo and asks for confirmation. The CLI
must not read a value from an environment-variable flag because environments
are often captured by diagnostics. `list` and `describe` display metadata
only. There is intentionally no `secrets get` value operation.

Automation can stream a value to `--stdin`. A Terraform provider can implement
`value` as a sensitive, write-only argument and persist only the returned
version and metadata in Terraform state. Read refreshes cannot reconstruct the
value; drift means metadata/version drift, and changing the configured
write-only value causes a new `set`.

### Job references and delivery

Secrets are referenced from a task, in the same namespace as the job:

```yaml
tasks:
  - name: api
    image: example/api:1
    secrets:
      - name: db-password
        target: env
        env: DATABASE_PASSWORD
      - name: tls-key
        target: file
        path: /run/trellis-secrets/tls.key
        mode: 0400
```

References contain no secret value and become part of the job revision. Names
must be unique per task; environment names and file paths must not collide.
File targets must be absolute and below `/run/trellis-secrets`, may not contain
symlink traversal, and default to mode `0400`. Modes may grant permissions only
to the container user (`0400` or `0600`). Secret files use a per-allocation
memory-backed mount and are never written into the container snapshot or a
persistent volume. Environment delivery is supported for compatibility but is
documented as weaker because child processes and debugging facilities can
expose environments; file delivery is preferred.

Job submission validates reference syntax but not existence. This permits a
job to be submitted ahead of time — for example, via a CI/CD pipeline — before
an out-of-band controller writes its secret.
Scheduling may place such a job, but the agent must not start an allocation
until every referenced secret is available and deliverable. The allocation
reports a sanitized `secret unavailable` or `secret delivery failed` reason,
with secret names visible only to principals already authorized to read that
namespace's metadata.

An allocation resolves each reference to the latest version when it is
prepared. Its resolved version set is recorded as non-secret allocation
metadata. Updating a secret does **not** mutate a running container: new and
restarted allocations receive the new version, while existing allocations
retain their in-memory copy until replacement. Operators rotate deterministically
by writing a new value and rolling/restarting affected jobs. A future explicit
`--restart-allocations` convenience must remain an opt-in action, not implicit
behavior of `set`.

## API contract and authorization

The namespace API exposes write-only values and readable metadata:

| Method and path | Behavior |
| --- | --- |
| `PUT /v1/namespaces/{namespace}/secrets/{name}` | Create or replace from a bounded request body; return metadata. |
| `GET /v1/namespaces/{namespace}/secrets` | List metadata. |
| `GET /v1/namespaces/{namespace}/secrets/{name}` | Return metadata only. |
| `DELETE /v1/namespaces/{namespace}/secrets/{name}` | Delete future availability. |

The write request uses an explicit `value_base64` JSON field so arbitrary bytes
round-trip without content-type ambiguity. The field is accepted only on
`PUT`, marked write-only in any generated schema, discarded immediately after
encryption, and absent from every response. Conditional `expected_version`
supports safe concurrent rotation. Create requires version 0; replacement
requires the current version. A mismatch returns `409 Conflict`.

User-facing authorization separates `secret:write`, `secret:metadata:read`,
`secret:delete`, and job submission. None confers plaintext read access.
Agents use a distinct authenticated internal capability scoped to allocations
assigned to that node. The leader verifies assignment and returns only the
named versions referenced by that allocation. A namespace workload token from
`api_access` is never allowed to request secret delivery.

Deleting a secret prevents new delivery and removes its encrypted record. It
does not remove a value already delivered to a running process, historical
backups, or an old Raft log segment. The API returns this fact in documentation
and CLI confirmation. Names may be recreated at version 1 only after deletion;
allocations record a separate immutable secret record ID so recreation cannot
be mistaken for the deleted generation.

## Security design

### Threat model

The design protects values from disclosure through read APIs, routine logs,
state-file inspection, copied backups, and decommissioned disks. It limits a
compromised agent to values needed by allocations currently assigned to that
node. It does not protect a value from cluster administrators controlling the
key-encryption key (KEK), a compromised leader while decrypting, root on an
assigned worker, or the target application after delivery. Namespace
authorization is an isolation boundary, not cryptographic separation between
applications in the same task.

### Encryption at rest and key management

Trellis uses envelope encryption:

1. Generate a random 256-bit data-encryption key (DEK) for every secret
   version and a 96-bit nonce from the operating-system CSPRNG.
2. Encrypt the value with AES-256-GCM. Authenticate a canonical encoding of
   cluster ID, namespace, immutable record ID, name, and version as additional
   authenticated data (AAD), preventing ciphertext substitution.
3. Wrap the DEK with the active KEK using an authenticated wrapping operation
   and persist only ciphertext, nonce, wrapped DEK, algorithm version, key ID,
   and metadata.
4. Zero short-lived plaintext and DEK buffers on a best-effort basis after
   use. Go cannot guarantee that copies were not made, so the design does not
   claim perfect memory erasure.

The KEK is 256 random bits supplied to every control-plane-eligible node by the
operator, independently of Raft and the data directory (for example, a
root-readable credential file). It is never accepted through the API, stored
in Bolt/Raft, copied into a snapshot, or exposed in diagnostics. Startup fails
closed if encrypted secrets exist and the required key ID is unavailable.
File permissions and length are validated before the node becomes eligible for
leadership. The cluster token must not be reused as a KEK.

The initial implementation accepts one active KEK. Secret-value rotation is a
normal versioned `set`. Changing that KEK is not yet an online operation: keep
the configured KEK stable and rotate values instead. A future keyring/rewrap
operation must distribute the new keyring, rewrap every DEK, verify all records,
and only then remove an old key. Operators must retain the configured KEK for
backups they intend to restore. Losing it makes values unrecoverable by design.

Ciphertext and metadata are deterministic state-machine inputs replicated by
Raft; encryption happens once on the leader before proposing the command.
Followers never replicate plaintext. Bolt files, Raft logs, snapshots, and
backups therefore contain ciphertext only, though operators must still protect
them because metadata and historical ciphertext remain sensitive.

### Agent delivery and persistence

All control-plane and agent traffic carrying secret material requires mutually
authenticated TLS with certificate identity verification; bearer-token-only
HTTP is insufficient. Secret support must remain disabled until this transport
is configured cluster-wide. The leader decrypts only after authorizing an
assigned allocation and sends a versioned, allocation-bound response over that
connection. Responses set `Cache-Control: no-store`, are size-bounded, and are
never retried through generic request logging middleware.

The agent holds plaintext only while preparing or running the allocation. It
passes environment targets directly to the runtime and builds file targets in
a node-private `tmpfs` with `nodev,nosuid,noexec`, restrictive directory
permissions, and atomic file creation. It must reject symlinks and path races.
The mount is read-only inside the container. On stop, failure, or node startup
recovery, the agent unmounts and removes the allocation's secret directory.
Swap and crash dumps can still capture memory; production guidance requires
encrypted swap or disabled swap and restricted core dumps.

Agents do not persist a plaintext cache. If the leader is unavailable and a
secret is not already attached to a still-running allocation, preparation
fails closed and retries with bounded backoff. Existing allocations continue
running during a leader outage. Container checkpoints containing secrets are
unsupported.

### Logging, metrics, and errors

Raw request bodies, decrypted values, DEKs, wrapped DEKs, ciphertext, and
environment blocks are classified sensitive and must never be logged. Logging
middleware records route templates, status, duration, namespace, secret name,
version, byte count, and request/principal IDs only. Errors are fixed/sanitized
at API boundaries; crypto-library errors are not serialized to clients.

Audit events cover set, metadata read, list, delete, delivery, denied delivery,
and KEK rewrap. They record actor/node/allocation identity and version, never a
value or value-derived digest. Metrics use operation/result labels only; secret
names, namespaces, and allocation IDs are forbidden as metric labels. Tests
must install a sentinel secret and assert that it is absent from captured logs,
events, errors, metrics, state snapshots, and CLI output.

## Rotation and failure semantics

- `set` is committed only after ciphertext is durably replicated. A successful
  response identifies the committed version; clients may safely retry with
  `expected_version` and an idempotency key.
- Concurrent writes cannot silently overwrite one another. A rejected or
  uncommitted write does not advance the version.
- Secret-value rotation affects only subsequent allocation preparation.
  Rollback means writing the prior value as a new version; old plaintext cannot
  be retrieved from Trellis.
- Secret deletion is immediately authoritative for new delivery. Running
  allocations are not killed, because deletion cannot erase process memory.
- Restoring a backup restores the secret metadata and ciphertext versions from
  that point in time and requires its KEKs. Operators must treat restored old
  credentials as potentially revoked and rotate them after disaster recovery.
- A corrupt record or missing KEK fails only allocations referencing that
  record, emits a sanitized high-severity event, and never falls back to
  plaintext or an empty value.

## Rollout and acceptance criteria

The implementation and future changes are gated by these criteria:

1. Add authenticated TLS and distinct node identities/capabilities to internal
   traffic; document certificate issuance and rotation.
2. Add operator keyring configuration, envelope encryption, ciphertext-only
   Raft persistence, rewrap tooling, restore procedures, and crypto test
   vectors.
3. Add metadata/write-only APIs, authorization, audit redaction, CLI commands,
   size/rate limits, and concurrency/idempotency behavior.
4. Add manifest references, assignment-scoped delivery, tmpfs file injection,
   environment injection warnings, cleanup/recovery, and rollout tooling.
5. Complete security review and tests for log leakage, API non-disclosure,
   authorization bypass, path traversal/symlinks, leader failover, backup and
   restore, missing/old keys, concurrent rotation, and crash cleanup.

No plaintext-read endpoint, provider plug-in, or delivery over a
bearer-token-only transport may be introduced as an intermediate shortcut.
