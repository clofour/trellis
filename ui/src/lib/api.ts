import type { Node, Job, JobSpec } from "./types";

const API_BASE = process.env.NEXT_PUBLIC_TRELLIS_API_URL || "";

async function apiFetch<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  return res.json();
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

export async function submitJob(spec: JobSpec): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/jobs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spec }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || `API error: ${res.status}`);
  }
}

export async function deleteJob(name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/jobs/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || `API error: ${res.status}`);
  }
}
