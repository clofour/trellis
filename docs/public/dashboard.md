# Dashboard

The `ui/` directory is a Next.js administration dashboard for cluster overview statistics, jobs and allocation diagnostics, nodes, and secrets. It follows the same [Trellis user model](user-model.md) as the CLI and examples.

The dashboard shows desired job state separately from runtime allocation lifecycle and health. When writes are enabled, job creation/editing uses the same YAML **job manifest** format accepted by `trellis jobs apply`; JSON is used only when the dashboard talks to the HTTP API.

## Configure and run

```sh
cd ui
cp .env.example .env.local
npm ci
npm run dev
```

Server-side variables:

| Variable | Purpose |
|---|---|
| `TRELLIS_API_URL` | Cluster control-plane URL; defaults to `http://localhost:8128`. |
| `TRELLIS_API_TOKEN` | Bearer cluster or namespace token. |
| `TRELLIS_NAMESPACE` | Namespace scope for jobs, allocations, and secrets. |
| `TRELLIS_ALLOW_WRITES` | Set exactly `true` to enable mutations; defaults false. |

The configured namespace is always shown in the sidebar. The UI uses same-origin Next.js route handlers as a server-side proxy, keeping the token out of browser JavaScript. `TRELLIS_ALLOW_WRITES` is only a UI-side guard, not a substitute for network and API authorization.

Read-only mode disables job apply/edit/delete actions, node drain actions, and secret changes. Secrets pages display metadata, never stored values.

## Current transport limitation

The dashboard currently targets one configured control-plane address. If cluster leadership moves somewhere that address can no longer reach, the dashboard must be reconfigured. This is a control-plane connectivity limitation rather than part of the workload user model; Raft leadership remains an advanced operational/developer concern.

## Production

```sh
npm run build
npm start
# or: docker build -t trellis-dashboard .
```

Place the dashboard behind HTTPS and your own identity-aware proxy. Keep read-write mode disabled unless the deployment is intended to be an administrative surface.
