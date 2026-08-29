export const TRELLIS_URL =
  process.env.TRELLIS_API_URL || "http://localhost:8128";

export function orchestratorHeaders(): HeadersInit {
  const headers: HeadersInit = {
    Authorization: `Bearer ${process.env.TRELLIS_API_TOKEN || ""}`,
  };
  const namespace = process.env.TRELLIS_NAMESPACE;
  if (namespace) {
    headers["X-Trellis-Namespace"] = namespace;
  }
  return headers;
}
