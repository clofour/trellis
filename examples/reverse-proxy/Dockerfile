FROM docker.io/library/nginx:alpine

# curl polls the Trellis API; jq parses allocation JSON without depending on
# object field ordering.
RUN apk add --no-cache curl jq

COPY nginx.conf.template /etc/nginx/nginx.conf.template
COPY sync-upstreams.sh /usr/local/bin/sync-upstreams.sh
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/sync-upstreams.sh \
             /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
