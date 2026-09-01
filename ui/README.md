# Trellis dashboard

The dashboard is a Next.js operations view of cluster health, nodes, jobs,
allocations, and namespace-scoped secret metadata. It uses the same
[user-facing vocabulary](../docs/public/user-model.md) and YAML job manifests as
the CLI, documentation, and examples. Runtime data refreshes every five seconds.

By default the dashboard is read-only. Set `TRELLIS_ALLOW_WRITES=true` only on
a trusted administration network to enable job apply/edit/delete actions, node
draining, and secret create/rotate/delete operations.

## Operator surface

The dashboard intentionally exposes operator workflows rather than every
internal API endpoint:

- cluster health and requested-vs-capacity resource pressure
- node health, labels, available host volumes, and optional drain actions
- job status, YAML job manifests, revision changes, and delete actions
- allocation lifecycle and health as separate states, plus diagnostics, retries, placement, ports, events, and log tails
- write-only secret metadata plus optional create/rotate/delete actions

Node registration, heartbeats, Raft joining, leadership epochs, internal
discovery, and raw transport operations remain implementation concerns and are
not part of the primary dashboard model.

## How API access works

Browser requests go to same-origin routes under `/api/v1`. Those server-side
route handlers forward requests to the Trellis control-plane API and add the
cluster bearer token. Keep all connection values server-side; do not expose the
token through a `NEXT_PUBLIC_` environment variable.

The dashboard is scoped to one namespace. Job writes are forced to that
configured namespace server-side, even if a browser request supplies a
different value.

## Configuration

Copy the example file and set the cluster API URL and token:

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
| `TRELLIS_API_URL` | Recommended | Base URL for the cluster control-plane API; defaults to `http://localhost:8128`. |
| `TRELLIS_API_TOKEN` | Yes | Token sent as a bearer token. Cluster authorization is required for secret management. |
| `TRELLIS_NAMESPACE` | Recommended | Namespace used to scope jobs, allocations, and secrets; defaults to the API's empty namespace when omitted. |
| `TRELLIS_ALLOW_WRITES` | No | Set to `true` to enable dashboard mutations. Defaults to `false`. |

The configured namespace is shown in the sidebar. The dashboard sends it as
`X-Trellis-Namespace` for API requests. When using a namespace-scoped token, set
this value to the token's namespace; the control plane enforces the scope carried
by the token. Secret management requires cluster authorization by design.

### Job manifests

The dashboard edits the same YAML **job manifest** used by `trellis jobs apply`
and by the repository examples. It parses YAML in the browser, preserves the
complete manifest schema, and converts the manifest to the JSON API
representation only when submitting it.

This keeps one authoring format across first-party interfaces while still
supporting every manifest field, including networking, runtimes, labels,
restart policies, constraints, update strategy, volumes, secrets, and health
checks. Duration values are shown in manifest form such as `10s` and `1m30s`
rather than raw API nanoseconds.

### Current control-plane connectivity limitation

The current dashboard configuration still points at one control-plane API
address. If leadership moves somewhere that address no longer reaches, the
dashboard must be reconfigured/restarted. This is a transport limitation, not a
user-facing resource concept; ordinary workload documentation should continue
to describe the target as a Trellis cluster rather than teaching Raft leadership
as part of the job model.

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

- **Dashboard API requests return 502:** verify the API URL/token and confirm the configured address reaches the active control plane.
- **Dashboard API requests return 401:** the UI token does not match an accepted cluster or namespace token.
- **Secret requests return 403:** secret administration requires cluster authorization rather than a namespace-scoped token.
- **Write controls are absent:** set `TRELLIS_ALLOW_WRITES=true` and restart the dashboard.
- **Data appears stale:** runtime data polls every five seconds; check the browser network panel and dashboard service logs for failed proxy requests.
