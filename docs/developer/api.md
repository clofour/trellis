# HTTP API

The control-plane API defaults to port 8128. Send `Authorization: Bearer TOKEN`; cluster-scoped callers may additionally select a namespaced view with `X-Trellis-Namespace: NAME`. JSON request bodies use `Content-Type: application/json`. Use TLS outside a local sandbox.

Trellis distinguishes three credential kinds:

- `bootstrap` — the root node/cluster credential used for node registration, Raft membership, backup/restore, and minting operator credentials;
- `operator` — an explicitly minted API credential with `namespace` or `cluster` scope and `read` or `write` access;
- `workload` — a scoped credential injected through task-group `api_access`.

Credential prefixes (`trls_boot_`, `trls_op_`, `trls_wl_`) are descriptive only. The server authenticates the complete bearer value and uses its authoritative stored principal metadata for generated credentials.

A task group requests workload access with an object such as `{"scope":"namespace","access":"read"}`. Namespace scope is restricted to the namespace containing the job. Cluster scope grants only the ordinary read/write API authority represented by the credential; it never turns into the bootstrap credential. Both scopes set `TRELLIS_NAMESPACE` to the job namespace as a default request scope.

The API uses the same resource vocabulary as the [Trellis user model](../public/user-model.md), but JSON is the transport representation. Humans author jobs as YAML manifests; job submission carries the equivalent JSON `JobSpec` inside the API request. Human-readable YAML memory sizes are normalized to byte counts in JSON.

## Public/operator endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/metrics` | Prometheus metrics. |
| `GET` | `/v1/auth/whoami` | Return the current credential kind, scope, access, namespace, and available provenance metadata. |
| `GET` | `/v1/nodes` | List node capacity/status; requires cluster scope. |
| `POST` / `DELETE` | `/v1/nodes/{id}/drain` | Drain or undrain; requires `cluster/write`. |
| `GET`, `POST` | `/v1/jobs` | List jobs or submit `{"spec": JobSpec}`. |
| `POST` | `/v1/jobs/plan` | Validate and calculate the authoritative semantic plan for a `JobSpec`. |
| `GET`, `DELETE` | `/v1/jobs/{name}` | Read job state/API representation or delete the job. |
| `GET` | `/v1/allocations?label=key:value` | List/filter allocations. |
| `GET` | `/v1/allocations/{id}/events` | Lifecycle event array. |
| `GET` | `/v1/allocations/{id}/logs?task=NAME&tail=100&follow=true` | Plain-text logs for one task in an allocation. |
| `PUT` | `/v1/namespaces/{ns}/secrets/{name}` | Set a secret; requires `cluster/write`. |
| `GET` | `/v1/namespaces/{ns}/secrets[/{name}]` | List/get secret metadata only; requires cluster scope. |
| `DELETE` | `/v1/namespaces/{ns}/secrets/{name}` | Delete a secret; requires `cluster/write`. |

`GET /v1/auth/whoami` is the capability/introspection primitive clients should use instead of probing protected endpoints. Typical generated-token response:

```json
{
  "kind": "operator",
  "scope": "namespace",
  "access": "write",
  "namespace": "payments",
  "created_at": "2026-09-02T20:00:00Z"
}
```

A bootstrap credential reports `kind: "bootstrap"`, `scope: "cluster"`, and `access: "write"`, but callers must still treat `bootstrap` as more privileged than ordinary `cluster/write`: root-only endpoint checks use the credential kind/context, not merely those two effective fields.

For allocation logs, `task` selects the task name from the allocation's task group. It may be omitted when the allocation has exactly one task; a multi-task allocation returns `400` until the caller selects one. The allocation ID is the Trellis allocation identity, not an agent/container runtime ID.

Secret write body: `{"value_base64":"...","expected_version":1}`; omit `expected_version` for unconditional update. Lists are JSON arrays. Non-2xx responses are errors; clients must tolerate reconciliation-driven changes between reads.

A namespace credential is authorized only for its stored namespace regardless of the namespace header supplied by the caller. A cluster credential may deliberately select different namespaces but receives only the read/write authority encoded in its principal.

## Bootstrap and cluster-internal endpoints

`POST /v1/credentials`, `GET /v1/backup`, `POST /v1/backup/restore`, `POST /v1/nodes`, `POST /v1/nodes/{id}/heartbeat`, `POST /v1/raft/join`, `DELETE /v1/raft/members/{id}`, and `POST /v1/raft/leadership-transfer` require the bootstrap credential. Agent port 8127 exposes internal allocation operations authenticated with the node bootstrap credential. These cluster-internal APIs are not a substitute for ordinary scoped operator access.

## Example

First-party clients treat a server address without an explicit scheme as HTTPS. In an API-enabled workload, use the injected namespace and CA rather than falling back to plaintext HTTP:

```sh
case "$TRELLIS_ADDR" in
  http://*|https://*) api_url=${TRELLIS_ADDR%/} ;;
  *) api_url="https://${TRELLIS_ADDR%/}" ;;
esac

printf '%s\n' "$TRELLIS_CA_CERT" > /tmp/trellis-ca.pem
curl -fsS --cacert /tmp/trellis-ca.pem \
  -H "Authorization: Bearer $TRELLIS_TOKEN" \
  -H "X-Trellis-Namespace: $TRELLIS_NAMESPACE" \
  "$api_url/v1/auth/whoami"
```

For deployments that deliberately use `http://`, omit `--cacert`. See [`examples/api-access/`](../../examples/api-access/) for an in-allocation namespace-scoped helper that handles both cases.
