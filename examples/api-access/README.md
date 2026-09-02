# Namespace-scoped API access

**Level:** Advanced · **Prerequisites:** complete the intermediate examples and use a reviewed controller image

This example shows how a trusted workload can discover and automate resources in its own namespace without embedding the cluster administrator token in an image.

## What `api_access` does

Setting `api_access: true` on a task group causes Trellis to obtain a persistent namespace token and add these variables to every task in the group:

| Variable | Meaning |
|---|---|
| `TRELLIS_ADDR` | Address of the Trellis control-plane API. |
| `TRELLIS_TOKEN` | Bearer token scoped to the job namespace. |
| `TRELLIS_NAMESPACE` | Namespace to send in scoped requests. |
| `TRELLIS_CA_CERT` | Cluster CA certificate (inline PEM) for TLS verification when configured. |

This is a group-level privilege boundary: every task in the group can read the injected environment and act with the token. Use a reviewed, pinned image and do not mix an untrusted sidecar into the group.

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

API clients should set request deadlines, retry transient transport/5xx failures with backoff, and tolerate resources changing between reads. Prefer read-only discovery loops unless mutation is essential. A namespace token must not be treated as a cluster-administration credential; secret management, backups, Raft membership, and other administrator operations require cluster authorization.

For a long-running process, poll only as often as needed and preserve the last known-good generated configuration through temporary API outages. The reverse-proxy recipe in the public cookbook applies this exact controller pattern.

[Examples index](../README.md) · [Learning path](../../docs/public/learning-path.md)
