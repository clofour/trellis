# HTTP API

The control-plane API defaults to port 8128. Send `Authorization: Bearer TOKEN`; namespace-scoped calls also use `X-Trellis-Namespace: NAME`. JSON request bodies use `Content-Type: application/json`. The cluster token is administrative; generated namespace tokens are limited to their namespace. Use TLS outside a local sandbox.

A task group can receive either credential through `api_access`. `api_access: namespace` injects a persistent namespace token restricted to the namespace containing the job; the mode cannot target another namespace. `api_access: cluster` injects the cluster administrator token. Both modes set `TRELLIS_NAMESPACE` to the job namespace as a default request scope, but that value does not restrict a cluster token.

The API uses the same resource vocabulary as the [Trellis user model](../public/user-model.md), but JSON is the transport representation. Humans author jobs as YAML manifests; job submission carries the equivalent JSON `JobSpec` inside the API request.

## Public/operator endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/metrics` | Prometheus metrics. |
| `GET` | `/v1/nodes` | List node capacity/status. |
| `POST` / `DELETE` | `/v1/nodes/{id}/drain` | Drain or undrain. |
| `GET`, `POST` | `/v1/jobs` | List jobs or submit `{"spec": JobSpec}`. |
| `GET`, `DELETE` | `/v1/jobs/{name}` | Read job state/API representation or delete the job. |
| `GET` | `/v1/allocations?label=key:value` | List/filter allocations. |
| `GET` | `/v1/allocations/{id}/events` | Lifecycle event array. |
| `GET` | `/v1/allocations/{id}/logs?tail=100&follow=true` | Plain-text logs. |
| `GET` | `/v1/internal/discovery` | Catalog entries (primarily in-cluster). |
| `GET`, `POST` | `/v1/backup`, `/v1/backup/restore` | Create/restore desired-state snapshot (admin). |
| `PUT` | `/v1/namespaces/{ns}/secrets/{name}` | Set base64 value and optional expected version (admin). |
| `GET` | `/v1/namespaces/{ns}/secrets[/{name}]` | List/get metadata only (admin). |
| `DELETE` | `/v1/namespaces/{ns}/secrets/{name}` | Delete secret (admin). |

Secret write body: `{"value_base64":"...","expected_version":1}`; omit `expected_version` for unconditional update. Lists are JSON arrays. Non-2xx responses are errors; clients must tolerate reconciliation-driven changes between reads.

A namespace token is authorized only for its stored namespace regardless of the namespace header supplied by the caller. A cluster token receives administrator context and may deliberately make cluster-wide or differently scoped requests.

## Cluster-internal endpoints

`POST /v1/nodes`, `POST /v1/nodes/{id}/heartbeat`, `POST /v1/raft/join`, `DELETE /v1/raft/members/{id}`, and `POST /v1/raft/leadership-transfer` support membership/control-plane operation. Agent port 8127 exposes list/start/delete allocation operations and allocation logs. These endpoints exchange internal API types and are not stable third-party contracts.

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
  "$api_url/v1/jobs"
```

For deployments that deliberately use `http://`, omit `--cacert`. See [`examples/api-access/`](../../examples/api-access/) for an in-allocation namespace-mode helper that handles both cases.
