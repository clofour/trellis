# In-cluster API access

**Level:** Advanced · **Prerequisites:** complete the intermediate examples and use a reviewed controller image

This example shows the normal least-privilege pattern: a trusted workload discovers and automates resources in its own namespace without embedding the cluster administrator token in an image.

## Choose an access mode

`api_access` is an explicit task-group mode:

| Mode | Credential | Intended use |
|---|---|---|
| omitted | None | Ordinary application workloads |
| `namespace` | Persistent token restricted to the job's own namespace | Discovery, proxies, and namespace-local controllers |
| `cluster` | Cluster administrator token | Fully trusted operator/control-plane workloads |

This example uses `api_access: namespace`. There is intentionally no namespace selector inside `api_access`: a job in `default` gets access only to `default`; a job in `payments` gets access only to `payments`.

Use `cluster` only when the workload genuinely needs administrative or cross-namespace operations. The injected `TRELLIS_NAMESPACE` still defaults to the job's namespace, but that default does not reduce the authority of a cluster token.

## What Trellis injects

With either enabled mode, Trellis adds these variables to every task in the group:

| Variable | Meaning |
|---|---|
| `TRELLIS_ADDR` | Address of the Trellis control-plane API. |
| `TRELLIS_TOKEN` | Namespace token or cluster administrator token, according to the selected mode. |
| `TRELLIS_NAMESPACE` | The job's namespace; use it as the default scope for namespace-aware requests. |
| `TRELLIS_CA_CERT` | Cluster CA certificate (inline PEM) for TLS verification when configured. |

This is a group-level privilege boundary: every task in the group can read the injected environment and act with the token. Use a reviewed, pinned image and do not mix an untrusted sidecar into the group. This is especially important for `cluster`, because compromise of any task in that group exposes the administrator credential.

## Build a useful client image

The stock nginx image in `trellis.yaml` only makes the privilege visible in the manifest; it does not contain `list-jobs.sh` or curl. For a real controller, copy the helper into an image:

```dockerfile
FROM curlimages/curl:8.12.1
COPY --chmod=0755 list-jobs.sh /usr/local/bin/list-jobs
ENTRYPOINT ["/usr/local/bin/list-jobs"]
```

Build and push that image, then replace the manifest's image. The helper validates the injected address, token, and namespace, sends Bearer authentication and `X-Trellis-Namespace`, treats an address without an explicit scheme as HTTPS, and uses `TRELLIS_CA_CERT` as a curl trust root when TLS is configured.

## Deploy and verify

```sh
trellisctl jobs apply --file examples/api-access/trellis.yaml
trellisctl --namespace default jobs status api-client
```

Use allocation logs to inspect the controller's non-sensitive result. Never print the token, dump the complete environment, return it to browser JavaScript, or include it in metrics and traces.

## Controller behavior

API clients should set request deadlines, retry transient transport/5xx failures with backoff, and tolerate resources changing between reads. Prefer read-only discovery loops unless mutation is essential. A namespace token cannot be broadened by changing the namespace header; administrator operations require `cluster` access or an external cluster credential.

For a long-running process, poll only as often as needed and preserve the last known-good generated configuration through temporary API outages. The reverse-proxy recipe in the public cookbook applies this exact namespace-controller pattern.

[Examples index](../README.md) · [Learning path](../../docs/public/learning-path.md)
