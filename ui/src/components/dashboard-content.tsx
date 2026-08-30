"use client";

import { useNodes, useJobs } from "@/hooks/use-api";
import { StatCard } from "./stat-card";
import { ResourceBar } from "./resource-bar";
import { Skeleton } from "./skeleton";
import type { Job, Node } from "@/lib/types";

export function DashboardContent() {
  const { data: nodes, isLoading: nodesLoading } = useNodes();
  const { data: jobs, isLoading: jobsLoading } = useJobs();

  const isLoading = nodesLoading || jobsLoading;

  const healthyNodes = nodes?.filter((n) => n.status === "healthy").length ?? 0;
  const totalNodes = nodes?.length ?? 0;
  const totalJobs = jobs?.length ?? 0;
  const totalAllocations = jobs?.reduce((sum, j) => sum + j.running, 0) ?? 0;
  const totalHealthy = jobs?.reduce((sum, j) => sum + j.healthy, 0) ?? 0;
  const totalDesired = jobs?.reduce((sum, j) => sum + j.desired, 0) ?? 0;

  const { totalCPU, totalMemory } = aggregateResources(nodes ?? []);
  const { requestedCPU, requestedMemory } = requestedResources(jobs ?? []);

  if (isLoading) return <DashboardSkeleton />;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Nodes"
          value={`${healthyNodes} / ${totalNodes}`}
          detail={`${healthyNodes} healthy`}
        />
        <StatCard
          label="Jobs"
          value={totalJobs}
          detail={`${totalJobs} registered`}
        />
        <StatCard
          label="Allocations"
          value={totalAllocations}
          detail={`${totalDesired} desired`}
        />
        <StatCard
          label="Health"
          value={
            totalDesired > 0
              ? `${Math.round((totalHealthy / totalDesired) * 100)}%`
              : "—"
          }
          detail={`${totalHealthy} of ${totalDesired} healthy`}
        />
      </div>

      <div className="rounded-lg border border-border bg-card p-5">
        <h3 className="text-sm font-medium text-card-foreground">
          Cluster Resources
        </h3>
        <div className="mt-4 space-y-4">
          <ResourceBar
            label="CPU requested"
            used={requestedCPU}
            total={totalCPU}
            format="cpu"
          />
          <ResourceBar
            label="Memory requested"
            used={requestedMemory}
            total={totalMemory}
            format="memory"
          />
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          Requested resources from desired job specs versus schedulable cluster
          capacity. This is reservation pressure, not live container utilization.
        </p>
      </div>

      {jobs && jobs.length > 0 && (
        <div className="rounded-lg border border-border bg-card p-5">
          <h3 className="text-sm font-medium text-card-foreground">
            Job Summary
          </h3>
          <div className="mt-3 space-y-2">
            {jobs.map((job) => {
              const pct =
                job.desired > 0
                  ? Math.round((job.healthy / job.desired) * 100)
                  : 0;
              const allHealthy = job.healthy === job.desired && job.desired > 0;

              return (
                <div
                  key={job.name}
                  className="flex items-center justify-between py-1.5"
                >
                  <span className="text-sm text-card-foreground">
                    {job.name}
                  </span>
                  <div className="flex items-center gap-3">
                    <span className="text-xs tabular-nums text-muted-foreground">
                      {job.healthy}/{job.desired}
                    </span>
                    <div className="h-1.5 w-20 overflow-hidden rounded-full bg-muted">
                      <div
                        className={`h-full rounded-full transition-all duration-500 ${
                          allHealthy ? "bg-emerald-500" : "bg-amber-500"
                        }`}
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function aggregateResources(nodes: Node[]) {
  let totalCPU = 0;
  let totalMemory = 0;
  for (const node of nodes) {
    if (node.status !== "draining") {
      totalCPU += node.cpu;
      totalMemory += node.memory;
    }
  }
  return { totalCPU, totalMemory };
}

function requestedResources(jobs: Job[]) {
  let requestedCPU = 0;
  let requestedMemory = 0;
  for (const job of jobs) {
    for (const group of job.spec?.task_groups ?? []) {
      for (const task of group.tasks) {
        if (!task.resources) continue;
        requestedCPU += task.resources.cpu * group.count;
        requestedMemory += task.resources.memory * group.count;
      }
    }
  }
  return { requestedCPU, requestedMemory };
}

function DashboardSkeleton() {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-lg border border-border bg-card p-5">
            <Skeleton className="h-3 w-20" />
            <Skeleton className="mt-3 h-7 w-16" />
            <Skeleton className="mt-2 h-3 w-24" />
          </div>
        ))}
      </div>
      <div className="rounded-lg border border-border bg-card p-5">
        <Skeleton className="h-4 w-32" />
        <div className="mt-4 space-y-4">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
        </div>
      </div>
    </div>
  );
}
