# WordPress + MySQL

This example deploys a WordPress blog backed by MySQL. It demonstrates
two core Trellis features:

- **DNS discovery**: WordPress connects to MySQL using the Trellis DNS
  name `mysql.blog.trellis`. No manual IP addresses, no service objects —
  Trellis resolves the name to the healthy MySQL allocation automatically.
- **Persistent volumes**: the MySQL data directory is a named volume that
  survives allocation replacement on the same node.

## Files

| File | Description |
| --- | --- |
| `mysql.yaml` | MySQL 8 database job with a persistent volume and TCP health check |
| `wordpress.yaml` | WordPress job that connects to MySQL via Trellis DNS |

## Deploying

Deploy MySQL first and wait for it to become healthy before starting
WordPress. WordPress reads the database host at startup; if MySQL is not
yet ready, WordPress will fail to connect.

```sh
trellis jobs apply --file mysql.yaml
trellis --namespace blog jobs status mysql
# Wait until the mysql allocation shows status: healthy

trellis jobs apply --file wordpress.yaml
trellis --namespace blog jobs status wordpress
```

Find the allocated host port for WordPress in `jobs status` and open it
in a browser. WordPress will show the installation wizard — the database
connection is already configured.

## Pairing with a reverse proxy

The WordPress task group carries `trellis.expose: "true"` and
`trellis/domain: localhost`. The [reverse-proxy example](../reverse-proxy/)
uses these labels to route traffic automatically — deploy the proxy job
from that example and it will pick up WordPress without any additional
configuration.

## Customization

- **Scale WordPress**: increase `count` on the `app` task group. WordPress
  is stateless; multiple replicas share the same MySQL database via
  `mysql.blog.trellis`.
- **Different namespace**: change `namespace` in both manifests to the
  same value and update `WORDPRESS_DB_HOST` to match the new namespace
  (e.g., `mysql.<namespace>.trellis`).
- **Credentials**: update the `MYSQL_*` and `WORDPRESS_DB_*` environment
  variables consistently across both manifests.
