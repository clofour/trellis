"use client";

import useSWR from "swr";
import type {
  AllocationEvent,
  Job,
  Node,
  SecretMetadata,
} from "@/lib/types";

const REFRESH_INTERVAL = 5000;

async function fetcher<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function textFetcher(url: string): Promise<string> {
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `${res.status} ${res.statusText}`);
  }
  return res.text();
}

export function useNodes() {
  return useSWR<Node[]>("/api/v1/nodes", fetcher, {
    refreshInterval: REFRESH_INTERVAL,
  });
}

export function useJobs() {
  return useSWR<Job[]>("/api/v1/jobs", fetcher, {
    refreshInterval: REFRESH_INTERVAL,
  });
}

export function useJob(name: string) {
  return useSWR<Job>(`/api/v1/jobs/${encodeURIComponent(name)}`, fetcher, {
    refreshInterval: REFRESH_INTERVAL,
  });
}

export function useAllocationEvents(id: string | null) {
  return useSWR<AllocationEvent[]>(
    id ? `/api/v1/allocations/${encodeURIComponent(id)}/events` : null,
    fetcher,
    { refreshInterval: REFRESH_INTERVAL },
  );
}

export function useAllocationLogs(id: string | null, tail = 200) {
  return useSWR<string>(
    id
      ? `/api/v1/allocations/${encodeURIComponent(id)}/logs?tail=${tail}`
      : null,
    textFetcher,
    { refreshInterval: REFRESH_INTERVAL },
  );
}

export function useSecrets() {
  return useSWR<SecretMetadata[]>("/api/v1/secrets", fetcher, {
    refreshInterval: REFRESH_INTERVAL,
  });
}

export function useOrchestratorStatus() {
  const { data, error } = useSWR<{ status: string }>(
    "/api/v1/health",
    fetcher,
    { refreshInterval: REFRESH_INTERVAL },
  );
  return {
    connected: data?.status === "ok",
    error: !!error,
    loading: !data && !error,
  };
}
