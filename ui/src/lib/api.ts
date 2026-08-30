import type {
  AllocationEvent,
  Job,
  JobSpec,
  Node,
  SecretMetadata,
} from "./types";

const API_BASE = process.env.NEXT_PUBLIC_TRELLIS_API_URL || "";

async function apiFetch<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `API error: ${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function apiMutation(
  path: string,
  init: RequestInit,
): Promise<Response> {
  const res = await fetch(`${API_BASE}${path}`, init);
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `API error: ${res.status} ${res.statusText}`);
  }
  return res;
}

export async function fetchNodes(): Promise<Node[]> {
  return apiFetch<Node[]>("/api/v1/nodes");
}

export async function fetchJobs(): Promise<Job[]> {
  return apiFetch<Job[]>("/api/v1/jobs");
}

export async function fetchJob(name: string): Promise<Job> {
  return apiFetch<Job>(`/api/v1/jobs/${encodeURIComponent(name)}`);
}

export async function fetchAllocationEvents(
  id: string,
): Promise<AllocationEvent[]> {
  return apiFetch<AllocationEvent[]>(
    `/api/v1/allocations/${encodeURIComponent(id)}/events`,
  );
}

export async function fetchAllocationLogs(
  id: string,
  tail = 200,
): Promise<string> {
  const res = await fetch(
    `${API_BASE}/api/v1/allocations/${encodeURIComponent(id)}/logs?tail=${tail}`,
    { cache: "no-store" },
  );
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `API error: ${res.status} ${res.statusText}`);
  }
  return res.text();
}

export async function submitJob(spec: JobSpec): Promise<void> {
  await apiMutation("/api/v1/jobs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spec }),
  });
}

export async function deleteJob(name: string): Promise<void> {
  await apiMutation(`/api/v1/jobs/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
}

export async function drainNode(id: string): Promise<void> {
  await apiMutation(`/api/v1/nodes/${encodeURIComponent(id)}/drain`, {
    method: "POST",
  });
}

export async function fetchSecrets(): Promise<SecretMetadata[]> {
  return apiFetch<SecretMetadata[]>("/api/v1/secrets");
}

function encodeUTF8Base64(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

export async function setSecret(
  name: string,
  value: string,
  expectedVersion?: number,
): Promise<SecretMetadata> {
  const body: { value_base64: string; expected_version?: number } = {
    value_base64: encodeUTF8Base64(value),
  };
  if (expectedVersion !== undefined) body.expected_version = expectedVersion;
  const res = await apiMutation(`/api/v1/secrets/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return res.json();
}

export async function deleteSecret(name: string): Promise<void> {
  await apiMutation(`/api/v1/secrets/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
}
