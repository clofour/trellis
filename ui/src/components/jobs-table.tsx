"use client";

import Link from "next/link";
import { useJobs } from "@/hooks/use-api";
import { EmptyState } from "./empty-state";
import { Skeleton } from "./skeleton";

export function JobsTable() {
  const { data: jobs, isLoading, error } = useJobs();

  if (isLoading) return <TableSkeleton />;
  if (error) {
    return (
      <EmptyState
        title="Unable to load jobs"
        description="Could not connect to the cluster. Ensure the dashboard connection is configured and reachable."
      />
    );
  }
  if (!jobs || jobs.length === 0) {
    return (
      <EmptyState
        title="No jobs"
        description="No job manifests have been applied in this namespace yet."
      />
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/50">
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Name
            </th>
            <th className="px-4 py-3 text-right font-medium text-muted-foreground">
              Desired
            </th>
            <th className="px-4 py-3 text-right font-medium text-muted-foreground">
              Running
            </th>
            <th className="px-4 py-3 text-right font-medium text-muted-foreground">
              Healthy
            </th>
            <th className="px-4 py-3 text-right font-medium text-muted-foreground">
              Revision
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Summary
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {jobs.map((job) => {
            const allHealthy = job.healthy === job.desired && job.desired > 0;
            const degraded = job.healthy < job.running;

            return (
              <tr
                key={job.name}
                className="transition-colors hover:bg-muted/30"
              >
                <td className="px-4 py-3">
                  <Link
                    href={`/jobs/${encodeURIComponent(job.name)}`}
                    className="font-medium text-card-foreground hover:underline"
                  >
                    {job.name}
                  </Link>
                </td>
                <td className="px-4 py-3 text-right tabular-nums text-card-foreground">
                  {job.desired}
                </td>
                <td className="px-4 py-3 text-right tabular-nums text-card-foreground">
                  {job.running}
                </td>
                <td className="px-4 py-3 text-right tabular-nums text-card-foreground">
                  {job.healthy}
                </td>
                <td className="px-4 py-3 text-right tabular-nums text-muted-foreground">
                  {job.revision}
                </td>
                <td className="px-4 py-3">
                  <HealthIndicator
                    allHealthy={allHealthy}
                    degraded={degraded}
                    running={job.running}
                    desired={job.desired}
                  />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function HealthIndicator({
  allHealthy,
  degraded,
  running,
  desired,
}: {
  allHealthy: boolean;
  degraded: boolean;
  running: number;
  desired: number;
}) {
  if (allHealthy) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400">
        <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
        Healthy
      </span>
    );
  }
  if (running < desired) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
        <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
        Converging
      </span>
    );
  }
  if (degraded) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-red-600 dark:text-red-400">
        <span className="h-1.5 w-1.5 rounded-full bg-red-500" />
        Degraded
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
      <span className="h-1.5 w-1.5 rounded-full bg-zinc-400" />
      Unknown
    </span>
  );
}

function TableSkeleton() {
  return (
    <div className="overflow-hidden rounded-lg border border-border">
      <div className="border-b border-border bg-muted/50 px-4 py-3">
        <Skeleton className="h-4 w-64" />
      </div>
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="flex gap-8 border-b border-border px-4 py-3 last:border-0">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-4 w-12" />
          <Skeleton className="h-4 w-16" />
        </div>
      ))}
    </div>
  );
}
