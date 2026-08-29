"use client";

import useSWR from "swr";
import type { Node, Job } from "@/lib/types";

const REFRESH_INTERVAL = 5000;

async function fetcher<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json();
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
