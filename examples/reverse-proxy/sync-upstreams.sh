#!/bin/sh
#
# Polls the Trellis allocations API and regenerates Nginx upstream
# configuration for healthy allocations labeled with trellis.expose=true.
#
# Expects TRELLIS_TOKEN and TRELLIS_ADDR to be set (injected by Trellis
# when api_access is true on the task group).
#
# Usage: sync-upstreams.sh [poll-interval-seconds]

set -eu

INTERVAL="${1:-5}"
CONF_DIR="/etc/nginx/conf.d"
CONF_FILE="$CONF_DIR/upstreams.conf"
BACKUP_FILE="$CONF_DIR/.upstreams.conf.previous"

mkdir -p "$CONF_DIR"

prev_hash=""

restore_previous() {
    if [ "${had_previous:-false}" = true ]; then
        mv "$BACKUP_FILE" "$CONF_FILE"
    else
        rm -f "$CONF_FILE" "$BACKUP_FILE"
    fi
}

while true; do
    response=$(curl -sf \
        -H "Authorization: Bearer $TRELLIS_TOKEN" \
        "$TRELLIS_ADDR/v1/allocations?label=trellis.expose:true" 2>/dev/null) || {
        echo "warn: failed to reach allocations API, retrying in ${INTERVAL}s"
        sleep "$INTERVAL"
        continue
    }

    # Parse allocations structurally. Label values are restricted before they
    # are embedded into Nginx directives so arbitrary metadata cannot inject
    # configuration syntax.
    config=$(printf '%s' "$response" | jq -r '
        def safe_name:
            gsub("[^A-Za-z0-9]"; "_");
        def upstream_name:
            "trellis_" + (.domain | safe_name) +
            (if .path == "/" then "" else (.path | safe_name) end);
        def endpoint:
            if (.address | contains(":"))
            then "[\(.address)]:\(.port)"
            else "\(.address):\(.port)"
            end;

        [
            .[]
            | select(.status == "healthy")
            | select(.labels["trellis.expose"] == "true")
            | (.labels["trellis/domain"] // "") as $domain
            | (.labels["trellis/path-prefix"] // "/") as $path
            | select($domain | test("^[A-Za-z0-9.-]+$"))
            | select($path | test("^/[A-Za-z0-9._/-]*$"))
            | select((.address // "") != "")
            | (.ports[0].host_port // 0) as $port
            | select($port > 0)
            | {
                domain: $domain,
                path: $path,
                address: .address,
                port: $port,
                weight: ((.labels["trellis/weight"] // "1") | tonumber? // 1)
              }
            | .weight = (if .weight > 0 then .weight else 1 end)
        ] as $routes

        | (
            $routes
            | group_by([.domain, .path])[] as $group
            | $group[0] as $route
            | "upstream \($route | upstream_name) {",
              ($group[] | "    server \(endpoint) weight=\(.weight);"),
              "}",
              ""
          ),
          (
            $routes
            | group_by(.domain)[] as $domain_routes
            | "server {",
              "    listen 80;",
              "    server_name \($domain_routes[0].domain);",
              "",
              (
                $domain_routes
                | group_by(.path)[] as $path_routes
                | $path_routes[0] as $route
                | "    location \($route.path) {",
                  "        proxy_pass http://\($route | upstream_name);",
                  "        proxy_set_header Host $host;",
                  "        proxy_set_header X-Real-IP $remote_addr;",
                  "        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
                  "        proxy_set_header X-Forwarded-Proto $scheme;",
                  "    }",
                  ""
              ),
              "}",
              ""
          )
    ') || {
        echo "warn: invalid allocations response, retrying in ${INTERVAL}s"
        sleep "$INTERVAL"
        continue
    }

    new_hash=$(printf %s "$config" | sha256sum | cut -d' ' -f1)
    if [ "$new_hash" != "$prev_hash" ]; then
        had_previous=false
        if [ -f "$CONF_FILE" ]; then
            cp "$CONF_FILE" "$BACKUP_FILE"
            had_previous=true
        fi

        printf '%s\n' "$config" > "$CONF_FILE"
        if ! nginx -t 2>/dev/null; then
            echo "warn: generated Nginx config is invalid; restoring previous config"
            restore_previous
            sleep "$INTERVAL"
            continue
        fi

        # On container startup the sync process can run before the foreground
        # Nginx master exists. In that case keep the validated file so the
        # initial Nginx start loads it; later changes use a normal reload.
        if [ -s /run/nginx.pid ] && kill -0 "$(cat /run/nginx.pid)" 2>/dev/null; then
            if ! nginx -s reload 2>/dev/null; then
                echo "warn: Nginx reload failed; restoring previous config"
                restore_previous
                sleep "$INTERVAL"
                continue
            fi
            echo "nginx: config reloaded with new upstreams"
        else
            echo "nginx: initial upstream config prepared"
        fi

        prev_hash="$new_hash"
        rm -f "$BACKUP_FILE"
    fi

    sleep "$INTERVAL"
done
