# API access

`api_access: true` asks Trellis to inject `TRELLIS_ADDR`, a namespace-scoped `TRELLIS_TOKEN`, and `TRELLIS_NAMESPACE` into every task in the group. Copy `list-jobs.sh` into your own trusted image and invoke it there. The stock nginx image in the manifest merely demonstrates the privilege switch; it does not contain the script or curl.

Do not enable API access on untrusted images or groups that do not need it. The token is a credential and must never be logged or exposed through a public web response.
