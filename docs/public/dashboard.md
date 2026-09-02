# Operations dashboard

The `ui/` directory is a Next.js operational dashboard for deployment progress, actionable failures, node maintenance, jobs, allocation diagnostics, and secrets. It follows the same [Trellis user model](user-model.md) and ready/converging/degraded job semantics as `trellisctl`.

The Operations page prioritizes what needs attention and what is still changing before presenting resource inventories. Diagnostic links open the relevant allocation events and task logs directly. Desired job state remains separate from runtime allocation lifecycle and health. When writes are enabled, job creation/editing uses the same YAML **job manifest** format accepted by `trellisctl jobs apply`; JSON is used only when the dashboard talks to the HTTP API.

The dashboard deliberately stays close to Trellis rather than adding application-platform abstractions. Applying a manifest is a two-step operation: **Review Plan** shows the semantic desired-state changes, then **Apply Manifest** performs the mutation. Changing the YAML after reviewing invalidates the plan. The manifest must explicitly name the active dashboard namespace; a mismatch is rejected rather than silently rewritten.

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
| `TRELLIS_API_ACCESS` | Optional `namespace` or `cluster` scope hint. When omitted, the dashboard detects whether the configured token has cluster authorization. |
| `TRELLIS_CLUSTER_NAME` | Human-readable cluster name shown in the active context. |
| `TRELLIS_NAMESPACE` | Initial/default namespace for namespaced views. |
| `TRELLIS_NAMESPACES` | Optional comma-separated strict allowlist for cluster-mode namespace selection. When omitted, a cluster-authorized dashboard may select any valid namespace. |
| `TRELLIS_ALLOW_WRITES` | Set exactly `true` to enable mutations; defaults false. |

The active cluster and namespace are always shown in the sidebar. The dashboard keeps the bearer token in server-side Next.js route handlers and sends only the selected namespace from the browser.

With a **cluster credential**, the namespace control becomes a selector. Without `TRELLIS_NAMESPACES`, it is a combobox and accepts any valid namespace name; configured namespace values are offered as suggestions. If `TRELLIS_NAMESPACES` is set, it becomes a strict dropdown limited to that allowlist. Changing the namespace refreshes namespaced jobs, allocations, logs, and secret requests under the new context.

With a **namespace credential**, the context is pinned to `TRELLIS_NAMESPACE` / the workload's own injected namespace. The selector is not editable, and the **Secrets** navigation item is hidden because secret management requires cluster authorization. The token itself enforces the namespace boundary even if a client attempts to send another `X-Trellis-Namespace` value.

The setup script deploys the first-party dashboard with `api_access: cluster`, so it receives cluster authorization and can use the namespace selector. A dashboard intended only for its own namespace should instead use `api_access: namespace`.

Read-only mode disables job apply/edit/delete actions, node drain actions, and secret changes in the UI. It does **not** reduce the authority of the underlying bearer token, so continue to protect a read-only dashboard that holds a cluster credential. `TRELLIS_ALLOW_WRITES` is a UI-side guard, not an authorization boundary.

## Manifest and log ergonomics

The editor accepts the same human YAML forms as `trellisctl`: Go-style durations such as `10s` and readable memory such as `64MiB`. Before submission those are normalized to the numeric JSON API representation. Existing numeric API values are formatted back into readable YAML when possible.

Allocation logs are task-specific. An allocation containing one task selects it automatically. For a multi-task group, choose the task whose stream you want to inspect. This mirrors `trellisctl jobs logs JOB --task TASK` and avoids exposing internal runtime container IDs as a user-facing log selector.

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
