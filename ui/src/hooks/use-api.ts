"use client";

import useSWR from "swr";
import type {
  AllocationEvent,
  Job,
  Node,
  SecretMetadata,
} from "@/lib/types";
import { useConfig } from "@/components/config-provider";

const REFRESH_INTERVAL = 5000;

async function fetcher<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function namespacedFetcher<T>([url, namespace]: [string, string]): Promise<T> {
  const res = await fetch(url, {
    headers: { "X-Trellis-Namespace": namespace },
  });
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function namespacedTextFetcher([url, namespace]: [string, string]): Promise<string> {
  const res = await fetch(url, {
    cache: "no-store",
    headers: { "X-Trellis-Namespace": namespace },
  });
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
  const { namespace } = useConfig();
  return useSWR<Job[]>(["/api/v1/jobs", namespace], namespacedFetcher, {
    refreshInterval: REFRESH_INTERVAL,
  });
}

export function useJob(name: string) {
  const { namespace } = useConfig();
  return useSWR<Job>(
    [`/api/v1/jobs/${encodeURIComponent(name)}`, namespace],
    namespacedFetcher,
    {
      refreshInterval: REFRESH_INTERVAL,
    },
  );
}

export function useAllocationEvents(id: string | null) {
  const { namespace } = useConfig();
  return useSWR<AllocationEvent[]>(
    id
      ? [`/api/v1/allocations/${encodeURIComponent(id)}/events`, namespace]
      : null,
    namespacedFetcher,
    { refreshInterval: REFRESH_INTERVAL },
  );
}

export function useAllocationLogs(
  id: string | null,
  task: string | null,
  tail = 200,
) {
  const { namespace } = useConfig();
  const query = new URLSearchParams({ tail: String(tail) });
  if (task) query.set("task", task);
  return useSWR<string>(
    id
      ? [
          `/api/v1/allocations/${encodeURIComponent(id)}/logs?${query.toString()}`,
          namespace,
        ]
      : null,
    namespacedTextFetcher,
    { refreshInterval: REFRESH_INTERVAL },
  );
}

export function useSecrets() {
  const { apiAccess, namespace } = useConfig();
  return useSWR<SecretMetadata[]>(
    apiAccess === "cluster" ? ["/api/v1/secrets", namespace] : null,
    namespacedFetcher,
    {
      refreshInterval: REFRESH_INTERVAL,
    },
  );
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
