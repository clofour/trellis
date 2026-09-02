# Operations dashboard

The `ui/` directory is a Next.js operational dashboard for deployment progress, actionable failures, node maintenance, jobs, allocation diagnostics, and secrets. It follows the same [Trellis user model](user-model.md) and ready/converging/degraded job semantics as the CLI.

The Operations page prioritizes what needs attention and what is still changing before presenting resource inventories. Diagnostic links open the relevant allocation events and logs directly. Desired job state remains separate from runtime allocation lifecycle and health. When writes are enabled, job creation/editing uses the same YAML **job manifest** format accepted by `trellisctl jobs apply`; JSON is used only when the dashboard talks to the HTTP API.

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
| `TRELLIS_API_TOKEN` | Bearer cluster or namespace token. `TRELLIS_TOKEN` from workload API access is also accepted. |
| `TRELLIS_CLUSTER_NAME` | Human-readable cluster name shown in the active context. |
| `TRELLIS_NAMESPACE` | Default namespace for jobs, allocations, and secrets. |
| `TRELLIS_NAMESPACES` | Optional comma-separated namespace allowlist for the context selector. Defaults to only `TRELLIS_NAMESPACE`. |
| `TRELLIS_ALLOW_WRITES` | Set exactly `true` to enable mutations; defaults false. |

The active `cluster / namespace` context is always shown in the sidebar. With multiple configured namespaces, selecting another scope refreshes every namespaced view and request. Only server-configured namespaces can be selected. The UI uses same-origin Next.js route handlers as a server-side proxy, keeping the token out of browser JavaScript. `TRELLIS_ALLOW_WRITES` is only a UI-side guard, not a substitute for network and API authorization.

A dashboard that only displays one namespace can run with a namespace token, but cluster-level pages and actions such as node maintenance require a cluster credential. The setup script therefore deploys the first-party dashboard with `api_access: cluster`. It still sets `TRELLIS_NAMESPACE` to the dashboard job's namespace and does not automatically configure other namespaces in the selector. If you deploy a namespace-only dashboard yourself, use `api_access: namespace`; that token can access only the namespace containing the dashboard job.

Read-only mode disables job apply/edit/delete actions, node drain actions, and secret changes in the UI. It does **not** reduce the authority of the underlying bearer token, so continue to protect a read-only dashboard that holds a cluster credential.

## Cluster connection

The configured API URL may point at any reachable control-plane node. A follower proxies dashboard requests to the active leader, so leadership changes do not require operator action or a dashboard restart. A stable node address or load balancer is still useful when the configured node itself may be unavailable.

## Production

```sh
npm run build
npm start
# or: docker build -t trellis-dashboard .
```

Place the dashboard behind HTTPS and your own identity-aware proxy. Keep read-write mode disabled unless the deployment is intended to be an administrative surface.

[Documentation index](../README.md) · [Previous: Cookbook](cookbook.md)
