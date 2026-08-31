#!/bin/sh
set -eu
: "${TRELLIS_ADDR:?api_access must inject TRELLIS_ADDR}"
: "${TRELLIS_TOKEN:?api_access must inject TRELLIS_TOKEN}"
: "${TRELLIS_NAMESPACE:?api_access must inject TRELLIS_NAMESPACE}"

curl --fail --silent --show-error \
  -H "Authorization: Bearer $TRELLIS_TOKEN" \
  -H "X-Trellis-Namespace: $TRELLIS_NAMESPACE" \
  "http://$TRELLIS_ADDR/v1/jobs"
