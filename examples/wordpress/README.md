# WordPress development stack

This example colocates WordPress and MariaDB in one allocation for a compact demonstration. It exercises sidecars, environment and secret injection, host volumes, health checks, restart policy, and service routing in one manifest.

It is **not** a recommended production topology: the database and web tier share placement and lifecycle, and the group cannot be scaled safely by increasing `count`.

## Prepare the node

The single allocation requires two host volumes. Create them with ownership appropriate for the container images, then advertise them when starting a node:

```sh
sudo install -d -m 0750 /srv/trellis/wordpress-db
sudo install -d -m 0750 /srv/trellis/wordpress-content
sudo trellis \
  --cluster-token "$TRELLIS_TOKEN" \
  --host-volume wordpress-db=/srv/trellis/wordpress-db \
  --host-volume wordpress-content=/srv/trellis/wordpress-content
```

The example uses host networking so WordPress can reach MariaDB at `127.0.0.1:3306`. MariaDB reserves host port 3306 and WordPress reserves host port 80, so the selected node must have both ports free. Host networking is an explicit tradeoff: the containers share the node's network surface rather than receiving normal isolation.

## Create credentials

Use independent values for the application account and MariaDB root account:

```sh
openssl rand -base64 32 | \
  trellisctl --namespace default secrets set wordpress-db-password --stdin
openssl rand -base64 32 | \
  trellisctl --namespace default secrets set mariadb-root-password --stdin
```

Trellis injects the application password into both tasks under the variable names each image expects. The secret value is not stored in the manifest.

## Deploy and observe

```sh
trellis jobs apply --file examples/wordpress/trellis.yaml
trellis --namespace default jobs status wordpress
trellis nodes list
```

Browse to `http://NODE_ADDRESS` after the allocation becomes healthy. MariaDB's TCP health check establishes reachability, while the WordPress HTTP check gates overall readiness. First-time database initialization may take longer than steady-state startup; adjust thresholds for your hardware rather than weakening the check indefinitely.

## Operate and tear down

Back up both host paths using database-aware procedures; copying a live MariaDB directory is not automatically a consistent backup. Destroying the job stops the containers but does not make host-volume data portable:

```sh
trellis --namespace default jobs destroy wordpress
```

Before production, separate MariaDB into a managed or replicated database service, use isolated networking and TLS, place the web tier behind the reverse-proxy pattern, pin images by digest, and test password rotation and restores. Scaling only the stateless WordPress tier requires separate task groups/jobs because a Trellis task group scales every contained task together.
