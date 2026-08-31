# HTTP API

The control-plane API defaults to port 8128. Send `Authorization: Bearer TOKEN`; namespace-scoped calls also use `X-Trellis-Namespace: NAME`. JSON request bodies use `Content-Type: application/json`. The cluster token is administrative; generated namespace tokens are limited to their namespace. Use TLS outside a local sandbox.

## Public/operator endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/metrics` | Prometheus metrics. |
| `GET` | `/v1/nodes` | List node capacity/status. |
| `POST` / `DELETE` | `/v1/nodes/{id}/drain` | Drain or undrain. |
| `GET`, `POST` | `/v1/jobs` | List jobs or submit `{"spec": JobSpec}`. |
| `GET`, `DELETE` | `/v1/jobs/{name}` | Status/spec or destroy. |
| `GET` | `/v1/allocations?label=key:value` | List/filter allocations. |
| `GET` | `/v1/allocations/{id}/events` | Lifecycle event array. |
| `GET` | `/v1/allocations/{id}/logs?tail=100&follow=true` | Plain-text logs. |
| `GET` | `/v1/internal/discovery` | Catalog entries (primarily in-cluster). |
| `GET`, `POST` | `/v1/backup`, `/v1/backup/restore` | Create/restore desired-state snapshot (admin). |
| `PUT` | `/v1/namespaces/{ns}/secrets/{name}` | Set base64 value and optional expected version (admin). |
| `GET` | `/v1/namespaces/{ns}/secrets[/{name}]` | List/get metadata only (admin). |
| `DELETE` | `/v1/namespaces/{ns}/secrets/{name}` | Delete secret (admin). |

Secret write body: `{"value_base64":"...","expected_version":1}`; omit `expected_version` for unconditional update. Lists are JSON arrays. Non-2xx responses are errors; clients must tolerate reconciliation-driven changes between reads.

## Cluster-internal endpoints

`POST /v1/nodes`, `POST /v1/nodes/{id}/heartbeat`, `POST /v1/raft/join`, `DELETE /v1/raft/members/{id}`, and `POST /v1/raft/leadership-transfer` support membership/control-plane operation. Agent port 8127 exposes list/start/delete allocation operations and allocation logs. These endpoints exchange internal API types and are not stable third-party contracts.

## Example

```sh
curl -fsS \
  -H "Authorization: Bearer $TRELLIS_TOKEN" \
  -H "X-Trellis-Namespace: $TRELLIS_NAMESPACE" \
  "http://$TRELLIS_ADDR/v1/jobs"
```

See [`examples/api-access/`](../../examples/api-access/) for in-allocation access.
