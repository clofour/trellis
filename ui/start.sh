#!/bin/sh
# If the cluster CA cert is injected as an env var, write it to a file and
# point NODE_EXTRA_CA_CERTS at it so Node.js trusts it for all TLS connections.
if [ -n "$TRELLIS_CA_CERT" ]; then
  printf '%s' "$TRELLIS_CA_CERT" > /tmp/trellis-ca.pem
  export NODE_EXTRA_CA_CERTS=/tmp/trellis-ca.pem
fi
exec node server.js
