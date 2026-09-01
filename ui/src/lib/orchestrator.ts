const rawTrellisURL =
  process.env.TRELLIS_API_URL ||
  (process.env.TRELLIS_ADDR && `https://${process.env.TRELLIS_ADDR}`) ||
  "http://localhost:8128";

export const TRELLIS_URL = /^https?:\/\//.test(rawTrellisURL)
  ? rawTrellisURL
  : `https://${rawTrellisURL}`;

export function getClusterName(): string {
  return process.env.TRELLIS_CLUSTER_NAME?.trim() || "Trellis cluster";
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

export function resolveDashboardNamespace(request: Request):
  | { namespace: string; error?: never }
  | { namespace?: never; error: string } {
  const requested = request.headers.get("X-Trellis-Namespace")?.trim();
  const namespace = requested ?? getDefaultNamespace();
  if (!getConfiguredNamespaces().includes(namespace)) {
    return { error: `Namespace ${JSON.stringify(namespace)} is not configured for this dashboard` };
  }
  return { namespace };
}

export function orchestratorHeaders(namespace?: string): HeadersInit {
  const token =
    process.env.TRELLIS_API_TOKEN || process.env.TRELLIS_TOKEN || "";
  const headers: HeadersInit = {
    Authorization: `Bearer ${token}`,
  };
  const selectedNamespace = namespace ?? getDefaultNamespace();
  if (selectedNamespace) {
    headers["X-Trellis-Namespace"] = selectedNamespace;
  }
  return headers;
}

export function getAllowWrites(): boolean {
  return process.env.TRELLIS_ALLOW_WRITES === "true";
}
