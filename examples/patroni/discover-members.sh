#!/bin/sh
set -eu
curl --fail --silent --show-error \
  -H "Authorization: Bearer $TRELLIS_TOKEN" \
  -H "X-Trellis-Namespace: $TRELLIS_NAMESPACE" \
  "http://$TRELLIS_ADDR/v1/allocations?label=service:patroni"
