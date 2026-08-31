# Secrets

Create values first, then apply the manifest:

```sh
printf %s 'token-value' | trellis --namespace default secrets set api-token --stdin
trellis --namespace default secrets set tls-key --file ./server.key
trellis jobs apply --file examples/secrets/trellis.yaml
```

Decimal `256` is file mode `0400`. The example image does not print credentials. Rotate with `--expected-version`, then replace/reapply consumers because running allocations retain delivered values.
