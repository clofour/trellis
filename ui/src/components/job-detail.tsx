"use client";

import { useJob } from "@/hooks/use-api";
import { StatCard } from "./stat-card";
import { AllocationsTable } from "./allocations-table";
import { Skeleton } from "./skeleton";
import { EmptyState } from "./empty-state";
import Link from "next/link";

export function JobDetail({ name }: { name: string }) {
  const { data: job, isLoading, error } = useJob(name);

  if (isLoading) return <JobDetailSkeleton />;
  if (error) {
    return (
      <EmptyState
        title="Job not found"
        description={`Could not load job "${name}". It may have been deleted or the orchestrator is unreachable.`}
      />
    );
  }
  if (!job) return null;

  const allHealthy = job.healthy === job.desired && job.desired > 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link href="/jobs" className="hover:text-foreground transition-colors">
          Jobs
        </Link>
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
          <path d="M5 3l4 4-4 4" />
        </svg>
        <span className="text-foreground font-medium">{job.name}</span>
      </div>

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground">{job.name}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Revision {job.revision}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {allHealthy ? (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700 ring-1 ring-inset ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-400 dark:ring-emerald-500/20">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              All healthy
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700 ring-1 ring-inset ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-400 dark:ring-amber-500/20">
              <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
              {job.healthy} of {job.desired} healthy
            </span>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard label="Desired" value={job.desired} />
        <StatCard label="Running" value={job.running} />
        <StatCard label="Healthy" value={job.healthy} />
      </div>

      <div>
        <h2 className="mb-3 text-sm font-medium text-foreground">
          Allocations
        </h2>
        <AllocationsTable allocations={job.allocations} />
      </div>
    </div>
  );
}

function JobDetailSkeleton() {
  return (
    <div className="space-y-6">
      <Skeleton className="h-4 w-32" />
      <div>
        <Skeleton className="h-6 w-48" />
        <Skeleton className="mt-2 h-4 w-24" />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="rounded-lg border border-border bg-card p-5">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-3 h-7 w-12" />
          </div>
        ))}
      </div>
      <div>
        <Skeleton className="h-4 w-24" />
        <Skeleton className="mt-3 h-40 w-full" />
      </div>
    </div>
  );
}
