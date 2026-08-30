FROM docker.io/library/nginx:alpine

# curl is needed by sync-upstreams.sh to poll the Trellis API.
RUN apk add --no-cache curl

COPY nginx.conf.template /etc/nginx/nginx.conf.template
COPY sync-upstreams.sh /usr/local/bin/sync-upstreams.sh
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/sync-upstreams.sh \
             /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
