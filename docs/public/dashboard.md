# Dashboard

The `ui/` directory is a Next.js administration dashboard for overview statistics, jobs and allocation details/logs/events, nodes, and secrets.

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
| `TRELLIS_API_URL` | Control-plane URL; defaults to `http://localhost:8128`. |
| `TRELLIS_API_TOKEN` | Bearer cluster or namespace token. |
| `TRELLIS_NAMESPACE` | Scope for jobs, allocations, and secrets. |
| `TRELLIS_ALLOW_WRITES` | Set exactly `true` to enable mutations; defaults false. |

The UI uses same-origin Next.js route handlers as a server-side proxy, keeping the token out of browser JavaScript. Do not set a public API URL unless you intentionally want browser-side routing.

## Production

```sh
npm run build
npm start
# or: docker build -t trellis-dashboard .
```

Place the dashboard behind HTTPS and your own identity-aware proxy. `TRELLIS_ALLOW_WRITES` is only a UI-side guard, not a substitute for network and API authorization. Read-only mode disables job create/edit/stop, node drain, and secret changes. Secrets pages display metadata, never stored values.
