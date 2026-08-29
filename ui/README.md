# Trellis dashboard

The dashboard is a read-only Next.js view of cluster health, nodes, jobs, and
allocations. It refreshes API data every five seconds.

## How API access works

Browser requests go to same-origin routes under `/api/v1`. Those server-side
route handlers forward requests to the elected Trellis leader and add the
cluster bearer token. Keep both configuration values server-side; do not expose
the token through a `NEXT_PUBLIC_` environment variable.

## Configuration

Copy the example file and set the leader API URL and cluster token:

```sh
cp .env.example .env.local
```

```dotenv
TRELLIS_API_URL=http://localhost:8128
TRELLIS_API_TOKEN=replace-with-the-cluster-token
TRELLIS_NAMESPACE=default
```

| Variable | Required | Description |
| --- | --- | --- |
| `TRELLIS_API_URL` | Recommended | Base URL of the current leader API; defaults to `http://localhost:8128`. |
| `TRELLIS_API_TOKEN` | Yes | Shared cluster token sent as a bearer token. |
| `TRELLIS_NAMESPACE` | Recommended | Namespace used to scope jobs and allocations; defaults to the orchestrator's empty namespace when omitted. |

The configured namespace is shown in the sidebar. The dashboard sends it as
`X-Trellis-Namespace` for all orchestrator requests. When using a
namespace-scoped token, set this value to the token's namespace; the
orchestrator enforces the scope carried by the token.

In the current single-leader setup, the configured URL must reach the node
that owns leadership. Restart the dashboard with a new URL after leadership
moves to an address it cannot already reach.

## Local development

Requires Node.js 20 or later and npm.

```sh
npm ci
npm run dev
```

Open <http://localhost:3000>. Source lives under `src/app`, shared UI pieces
under `src/components`, and browser data hooks under `src/hooks`.

## Quality checks

```sh
npm run lint
npm run build
```

## Production

Install locked dependencies, build, and start the production server:

```sh
npm ci
npm run build
npm run start
```

The default address is <http://localhost:3000>. Put the application behind your
normal TLS reverse proxy if it is exposed outside a trusted administration
network.

For systemd, store the application at `/opt/trellis/ui`, place the two
variables in `/opt/trellis/ui/.env.local`, and create
`/etc/systemd/system/trellis-ui.service`:

```ini
[Unit]
Description=Trellis dashboard
After=network-online.target trellis-node.service
Wants=network-online.target

[Service]
Type=simple
User=trellis-ui
Group=trellis-ui
WorkingDirectory=/opt/trellis/ui
Environment=NODE_ENV=production
ExecStart=/usr/bin/npm run start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Protect `.env.local` so only the service account can read it, then enable the
service:

```sh
sudo chown trellis-ui:trellis-ui /opt/trellis/ui/.env.local
sudo chmod 600 /opt/trellis/ui/.env.local
sudo systemctl daemon-reload
sudo systemctl enable --now trellis-ui
```

## Troubleshooting

- **Dashboard API requests return 502:** verify both environment variables and
  confirm `TRELLIS_API_URL` points to the elected leader.
- **Dashboard API requests return 401:** the UI token does not match the
  cluster token used by the node.
- **Data appears stale:** the dashboard polls every five seconds; check the
  browser network panel and dashboard service logs for failed proxy requests.
