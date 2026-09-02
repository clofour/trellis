# Trellis operations dashboard

The dashboard is a Next.js operational interface for deployments, failures,
cluster maintenance, and namespace-scoped configuration. It uses the same
[user-facing vocabulary](../docs/public/user-model.md) and YAML job manifests as
`trellisctl`, the documentation, and examples. Runtime data refreshes every five seconds.

By default the dashboard is read-only. Set `TRELLIS_ALLOW_WRITES=true` only on
a trusted administration network to enable job apply/edit/delete actions, node
draining, and secret create/rotate/delete operations.

## Operator surface

The operations page starts with the questions an operator normally has: what
needs attention, which deployments are still converging, why a rollout is
blocked, and where to look next. Resource inventories remain available as
secondary views. The dashboard intentionally exposes operator workflows rather
than every internal API endpoint:

- cluster readiness, explicit failures, blocked progress, and maintenance activity
- deployment state using the same ready/converging/degraded semantics as `trellisctl`
- node health, labels, available host volumes, and optional drain actions
- job rollout summaries, YAML manifests, semantic review-before-apply, revision changes, and delete actions
- allocation lifecycle and health as separate states, plus diagnostics, retries, placement, ports, events, and per-task log tails
- write-only secret metadata plus optional create/rotate/delete actions

Node registration, heartbeats, Raft joining, leadership epochs, internal
discovery, and raw transport operations remain implementation concerns and are
not part of the primary dashboard model.

## How API access works

Browser requests go to same-origin routes under `/api/v1`. Those server-side
route handlers forward requests to the Trellis control-plane API and add the
cluster bearer token. Keep all connection values server-side; do not expose the
token through a `NEXT_PUBLIC_` environment variable.

The dashboard connects to one cluster and exposes only the namespaces listed in
its server-side configuration. The selected namespace is attached to every
scoped request. A browser cannot select an unconfigured namespace. Job manifests
must explicitly name the selected namespace; the dashboard rejects a mismatch
rather than silently rewriting desired state.

## Configuration

Copy the example file and set the cluster API URL and token:

```sh
cp .env.example .env.local
```

```dotenv
TRELLIS_API_URL=http://localhost:8128
TRELLIS_API_TOKEN=replace-with-the-cluster-token
TRELLIS_CLUSTER_NAME=production
TRELLIS_NAMESPACE=default
TRELLIS_NAMESPACES=default,staging
TRELLIS_ALLOW_WRITES=false
```

| Variable | Required | Description |
| --- | --- | --- |
| `TRELLIS_API_URL` | Recommended | Base URL for the cluster control-plane API; defaults to `http://localhost:8128`. |
| `TRELLIS_API_TOKEN` | Yes | Token sent as a bearer token. Cluster authorization is required for secret management. |
| `TRELLIS_CLUSTER_NAME` | No | Human-readable cluster name shown in the dashboard; defaults to `Trellis cluster`. |
| `TRELLIS_NAMESPACE` | Recommended | Default namespace used to scope jobs, allocations, and secrets. |
| `TRELLIS_NAMESPACES` | No | Comma-separated allowlist shown in the namespace selector. Defaults to only `TRELLIS_NAMESPACE`. |
| `TRELLIS_ALLOW_WRITES` | No | Set to `true` to enable dashboard mutations. Defaults to `false`. |

The active `cluster / namespace` context is shown in the sidebar. If several
namespaces are configured, changing the selector refreshes jobs, allocations,
logs, and secrets in that scope. When using a namespace-scoped token, configure
only that token's namespace; the control plane independently enforces the scope
carried by the token. Secret management requires cluster authorization by design.

### Job manifests

The dashboard edits the same YAML **job manifest** used by `trellisctl jobs apply`
and by the repository examples. It parses YAML in the browser and converts the
manifest to the numeric JSON API representation before planning and submitting
it. The dashboard first shows a semantic plan; applying is a separate action.
Editing the YAML after planning invalidates the plan and requires another review.

This keeps one authoring format across first-party interfaces while still
supporting every manifest field, including networking, runtimes, labels,
restart policies, constraints, update strategy, volumes, secrets, and health
checks. Duration values are shown in manifest form such as `10s` and `1m30s`
rather than raw API nanoseconds. Memory values are shown as readable sizes such
as `64MiB` when possible and are converted to byte counts for the API.

Allocation logs follow the same task model as `trellisctl jobs logs`: a
single-task allocation opens that task directly, while a multi-task allocation
requires selecting the task whose stream you want to inspect.

### Cluster connection

`TRELLIS_API_URL` can point at any reachable control-plane node. Followers proxy
requests to the active leader, so normal dashboard operation does not require
leader discovery or dashboard restarts when leadership changes. Use a stable
node address or load balancer if the configured node itself may be unavailable.

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

- **Dashboard API requests return 502:** verify the API URL/token and confirm the configured address reaches the active control plane.
- **Dashboard API requests return 401:** the UI token does not match an accepted cluster or namespace token.
- **Secret requests return 403:** secret administration requires cluster authorization rather than a namespace-scoped token.
- **A manifest is rejected for namespace mismatch:** select the manifest's namespace in the dashboard or change the manifest explicitly; the dashboard will not rewrite it.
- **Write controls are absent:** set `TRELLIS_ALLOW_WRITES=true` and restart the dashboard.
- **Data appears stale:** runtime data polls every five seconds; check the browser network panel and dashboard service logs for failed proxy requests.
