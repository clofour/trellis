# Trellis dashboard

The dashboard is a Next.js operations view of cluster health, nodes, jobs,
allocations, and namespace-scoped secret metadata. It refreshes runtime data
every five seconds.

By default the dashboard is read-only. Set `TRELLIS_ALLOW_WRITES=true` only on
a trusted administration network to enable job submission/editing, job stops,
node draining, and secret create/rotate/delete operations.

## Operator surface

The dashboard intentionally exposes operator workflows rather than every
internal API endpoint:

- cluster health and requested-vs-capacity resource pressure
- node health, labels, available host volumes, and optional drain actions
- job status, desired configuration, full-schema job submission/editing, and stop actions
- allocation lifecycle/health, diagnostics, retry state, placement, ports, lifecycle events, and log tails
- write-only secret metadata plus optional create/rotate/delete actions

Node registration, heartbeats, Raft joining, internal discovery, and raw metrics
remain API/daemon concerns and are not duplicated as dashboard controls.

## How API access works

Browser requests go to same-origin routes under `/api/v1`. Those server-side
route handlers forward requests to the elected Trellis leader and add the
cluster bearer token. Keep all connection values server-side; do not expose the
token through a `NEXT_PUBLIC_` environment variable.

The dashboard is scoped to one namespace. Job writes are forced to that
configured namespace server-side, even if a browser request supplies a
different value.

## Configuration

Copy the example file and set the leader API URL and cluster token:

```sh
cp .env.example .env.local
```

```dotenv
TRELLIS_API_URL=http://localhost:8128
TRELLIS_API_TOKEN=replace-with-the-cluster-token
TRELLIS_NAMESPACE=default
TRELLIS_ALLOW_WRITES=false
```

| Variable | Required | Description |
| --- | --- | --- |
| `TRELLIS_API_URL` | Recommended | Base URL of the current leader API; defaults to `http://localhost:8128`. |
| `TRELLIS_API_TOKEN` | Yes | Token sent as a bearer token. Cluster authorization is required for secret management. |
| `TRELLIS_NAMESPACE` | Recommended | Namespace used to scope jobs, allocations, and secrets; defaults to the orchestrator's empty namespace when omitted. |
| `TRELLIS_ALLOW_WRITES` | No | Set to `true` to enable dashboard mutations. Defaults to `false`. |

The configured namespace is shown in the sidebar. The dashboard sends it as
`X-Trellis-Namespace` for orchestrator requests. When using a namespace-scoped
token, set this value to the token's namespace; the orchestrator enforces the
scope carried by the token. Secret management requires cluster authorization
by design.

### Job editor

The dashboard job editor uses the complete JSON representation of Trellis's job
spec, rather than maintaining a second partial form schema. This means all
manifest fields round-trip without loss, including networking, runtimes,
labels, restart policies, constraints, update strategy, volumes, secrets, and
health-check timing. The CLI remains the natural choice when authoring YAML
files in a repository or CI/CD pipeline.

In the current single-leader setup, the configured URL must reach the node that
owns leadership. Restart the dashboard with a new URL after leadership moves to
an address it cannot already reach.

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
network. Keep read-write mode disabled unless the deployment is intended to be
an administrative surface.

For systemd, store the application at `/opt/trellis/ui`, place the variables in
`/opt/trellis/ui/.env.local`, and create `/etc/systemd/system/trellis-ui.service`:

```ini
[Unit]
Description=Trellis dashboard
After=network-online.target trellis.service
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

- **Dashboard API requests return 502:** verify the API URL/token and confirm `TRELLIS_API_URL` points to the elected leader.
- **Dashboard API requests return 401:** the UI token does not match an accepted cluster or namespace token.
- **Secret requests return 403:** secret administration requires cluster authorization rather than a namespace-scoped token.
- **Write controls are absent:** set `TRELLIS_ALLOW_WRITES=true` and restart the dashboard.
- **Data appears stale:** runtime data polls every five seconds; check the browser network panel and dashboard service logs for failed proxy requests.
