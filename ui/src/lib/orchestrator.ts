const rawTrellisURL =
  process.env.TRELLIS_API_URL ||
  (process.env.TRELLIS_ADDR && `https://${process.env.TRELLIS_ADDR}`) ||
  "http://localhost:8128";

export const TRELLIS_URL = /^https?:\/\//.test(rawTrellisURL)
  ? rawTrellisURL
  : `https://${rawTrellisURL}`;

export type DashboardAPIAccess = "namespace" | "cluster";

const namespacePattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$/;

export function getClusterName(): string {
  return process.env.TRELLIS_CLUSTER_NAME?.trim() || "Trellis cluster";
}

export function hasConfiguredNamespaceAllowlist(): boolean {
  return (process.env.TRELLIS_NAMESPACES || "")
    .split(",")
    .some((namespace) => namespace.trim() !== "");
}

export function getConfiguredNamespaces(): string[] {
  const configured = (process.env.TRELLIS_NAMESPACES || "")
    .split(",")
    .map((namespace) => namespace.trim())
    .filter(Boolean);
  const defaultNamespace = process.env.TRELLIS_NAMESPACE?.trim() || "";
  if (defaultNamespace && !configured.includes(defaultNamespace)) {
    configured.unshift(defaultNamespace);
  }
  return configured.length > 0 ? configured : [defaultNamespace];
}

export function getDefaultNamespace(): string {
  const namespaces = getConfiguredNamespaces();
  const configuredDefault = process.env.TRELLIS_NAMESPACE?.trim() || "";
  return namespaces.includes(configuredDefault) ? configuredDefault : namespaces[0];
}

export function getConfiguredAPIAccess(): DashboardAPIAccess | null {
  const configured = process.env.TRELLIS_API_ACCESS?.trim().toLowerCase();
  if (configured === "cluster" || configured === "namespace") return configured;
  return null;
}

export async function detectDashboardAPIAccess(): Promise<DashboardAPIAccess> {
  const configured = getConfiguredAPIAccess();
  if (configured) return configured;

  const namespace = getDefaultNamespace();
  if (!namespace) return "namespace";

  try {
    // Secret metadata is administrator-only. A 403 therefore identifies a
    // namespace-scoped token without exposing or mutating secret values.
    const res = await fetch(
      `${TRELLIS_URL}/v1/namespaces/${encodeURIComponent(namespace)}/secrets`,
      { headers: orchestratorHeaders(namespace), cache: "no-store" },
    );
    if (res.status === 401 || res.status === 403) return "namespace";
    return "cluster";
  } catch {
    // Fail closed while the control plane is unavailable. The UI can still
    // operate in its configured namespace without advertising admin controls.
    return "namespace";
  }
}

export function resolveDashboardNamespace(request: Request):
  | { namespace: string; error?: never }
  | { namespace?: never; error: string } {
  const requested = request.headers.get("X-Trellis-Namespace")?.trim();
  const namespace = requested ?? getDefaultNamespace();
  if (namespace && !namespacePattern.test(namespace)) {
    return { error: `Namespace ${JSON.stringify(namespace)} is invalid` };
  }
  if (
    hasConfiguredNamespaceAllowlist() &&
    !getConfiguredNamespaces().includes(namespace)
  ) {
    return {
      error: `Namespace ${JSON.stringify(namespace)} is not configured for this dashboard`,
    };
  }
  return { namespace };
}

export function orchestratorHeaders(
  namespace?: string | null,
): HeadersInit {
  const token =
    process.env.TRELLIS_API_TOKEN || process.env.TRELLIS_TOKEN || "";
  const headers: HeadersInit = {
    Authorization: `Bearer ${token}`,
  };
  const selectedNamespace = namespace === undefined ? getDefaultNamespace() : namespace;
  if (selectedNamespace) {
    headers["X-Trellis-Namespace"] = selectedNamespace;
  }
  return headers;
}

export function getAllowWrites(): boolean {
  return process.env.TRELLIS_ALLOW_WRITES === "true";
}
