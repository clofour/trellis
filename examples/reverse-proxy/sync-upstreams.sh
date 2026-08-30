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

mkdir -p "$CONF_DIR"

prev_hash=""

while true; do
    # Fetch allocations whose task group opted in to proxy exposure.
    response=$(curl -sf \
        -H "Authorization: Bearer $TRELLIS_TOKEN" \
        "$TRELLIS_ADDR/v1/allocations?label=trellis.expose:true" 2>/dev/null) || {
        echo "warn: failed to reach allocations API, retrying in ${INTERVAL}s"
        sleep "$INTERVAL"
        continue
    }

    # Build the Nginx config from the allocation response.
    # Each allocation entry has: job, group, labels, address, ports[], status.
    #
    # We group by (domain, path-prefix) to form upstreams, then emit a
    # server block per domain and a location block per path-prefix.
    config=$(echo "$response" | awk '
    BEGIN {
        RS = "[{},]"
        FS = ":"
        domain = ""
        prefix = ""
        expose = ""
        address = ""
        host_port = ""
        status = ""
    }
    /"trellis.expose"/ { gsub(/[ "\t]/, "", $2); expose = $2 }
    /"trellis\/domain"/ { gsub(/[ "\t]/, "", $2); domain = $2 }
    /"trellis\/path-prefix"/ { gsub(/[ "\t]/, "", $2); prefix = $2 }
    /"address"/ { gsub(/[ "\t]/, "", $2); address = $2 }
    /"host_port"/ { gsub(/[ "\t]/, "", $2); host_port = $2 }
    /"status"/ {
        gsub(/[ "\t]/, "", $2)
        status = $2
        if (status == "healthy" && expose == "true" && domain != "" && address != "" && host_port != "") {
            key = domain ":" (prefix != "" ? prefix : "/")
            if (!(key in upstreams)) {
                upstream_order[++n] = key
                domains[key] = domain
                prefixes[key] = (prefix != "" ? prefix : "/")
            }
            upstreams[key] = upstreams[key] "    server " address ":" host_port ";\n"
        }
        domain = ""; prefix = ""; expose = ""; address = ""; host_port = ""; status = ""
    }
    END {
        for (i = 1; i <= n; i++) {
            key = upstream_order[i]
            name = domains[key]
            gsub(/[^a-zA-Z0-9]/, "_", name)
            p = prefixes[key]
            if (p != "/") {
                pname = p
                gsub(/[^a-zA-Z0-9]/, "_", pname)
                name = name pname
            }
            print "upstream " name " {"
            printf "%s", upstreams[key]
            print "}"
            print ""
        }

        # Group by domain for server blocks.
        for (i = 1; i <= n; i++) {
            key = upstream_order[i]
            d = domains[key]
            if (!(d in seen_domain)) {
                seen_domain[d] = 1
                domain_order[++dn] = d
            }
            p = prefixes[key]
            domain_locations[d] = domain_locations[d] key "|" p "\n"
        }

        for (di = 1; di <= dn; di++) {
            d = domain_order[di]
            print "server {"
            print "    listen 80;"
            print "    server_name " d ";"
            print ""

            split(domain_locations[d], locs, "\n")
            for (li in locs) {
                if (locs[li] == "") continue
                split(locs[li], parts, "|")
                key = parts[1]
                p = parts[2]

                uname = domains[key]
                gsub(/[^a-zA-Z0-9]/, "_", uname)
                if (p != "/") {
                    pname = p
                    gsub(/[^a-zA-Z0-9]/, "_", pname)
                    uname = uname pname
                }

                print "    location " p " {"
                print "        proxy_pass http://" uname ";"
                print "        proxy_set_header Host $host;"
                print "        proxy_set_header X-Real-IP $remote_addr;"
                print "        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;"
                print "        proxy_set_header X-Forwarded-Proto $scheme;"
                print "    }"
                print ""
            }
            print "}"
            print ""
        }
    }
    ')

    # Only reload Nginx if the config actually changed.
    new_hash=$(echo "$config" | sha256sum | cut -d' ' -f1)
    if [ "$new_hash" != "$prev_hash" ]; then
        echo "$config" > "$CONF_FILE"
        nginx -t 2>/dev/null && nginx -s reload 2>/dev/null && \
            echo "nginx: config reloaded with new upstreams" || \
            echo "warn: nginx config test failed, keeping previous config"
        prev_hash="$new_hash"
    fi

    sleep "$INTERVAL"
done
