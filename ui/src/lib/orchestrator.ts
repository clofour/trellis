const rawTrellisURL =
  process.env.TRELLIS_API_URL ||
  (process.env.TRELLIS_ADDR && `https://${process.env.TRELLIS_ADDR}`) ||
  "http://localhost:8128";

export const TRELLIS_URL = /^https?:\/\//.test(rawTrellisURL)
  ? rawTrellisURL
  : `https://${rawTrellisURL}`;

export function orchestratorHeaders(): HeadersInit {
  const token =
    process.env.TRELLIS_API_TOKEN || process.env.TRELLIS_TOKEN || "";
  const headers: HeadersInit = {
    Authorization: `Bearer ${token}`,
  };
  const namespace = process.env.TRELLIS_NAMESPACE;
  if (namespace) {
    headers["X-Trellis-Namespace"] = namespace;
  }
  return headers;
}

export function getAllowWrites(): boolean {
  return process.env.TRELLIS_ALLOW_WRITES === "true";
}
