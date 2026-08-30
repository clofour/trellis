#!/bin/sh
set -e

# Install the base Nginx configuration.
cp /etc/nginx/nginx.conf.template /etc/nginx/nginx.conf

# Start the upstream sync daemon in the background.
# It polls the Trellis API and rewrites /etc/nginx/conf.d/upstreams.conf,
# reloading Nginx only when the config changes.
/usr/local/bin/sync-upstreams.sh &

# Run Nginx in the foreground.  Exiting Nginx stops the container.
exec nginx -g "daemon off;"
