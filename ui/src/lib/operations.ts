import type { Allocation, Job, Node } from "./types";

export type JobState = "ready" | "converging" | "degraded";
export type IssueSeverity = "critical" | "warning" | "info";

export interface OperationalIssue {
  id: string;
  severity: IssueSeverity;
  title: string;
  description: string;
  action: string;
  href: string;
  job?: string;
  allocation?: Allocation;
}

export function jobReady(job: Job): boolean {
  if (job.desired <= 0) return false;
  const allocations = job.allocations ?? [];
  if (allocations.length === 0) {
    return job.running >= job.desired && job.healthy >= job.desired;
  }
  const current = allocations.filter(
    (allocation) => allocation.job_revision === job.revision && !allocation.draining,
  );
  const running = current.filter((allocation) => allocation.phase === "running").length;
  const healthy = current.filter(
    (allocation) => allocation.phase === "running" && allocation.health === "healthy",
  ).length;
  return running >= job.desired && healthy >= job.desired;
}

export function jobState(job: Job): JobState {
  if (jobReady(job)) return "ready";
  if (
    (job.allocations ?? []).some(
      (allocation) =>
        !(allocation.draining && allocation.job_revision < job.revision) &&
        (allocation.health === "unhealthy" ||
          allocation.phase === "failed" ||
          allocation.phase === "lost"),
    )
  ) {
    return "degraded";
  }
  return "converging";
}

export function allocationNeedsAttention(
  allocation: Allocation,
  currentRevision: number,
): boolean {
  const oldHealthyReplacement =
    allocation.draining &&
    allocation.job_revision < currentRevision &&
    allocation.health !== "unhealthy" &&
    allocation.phase !== "failed" &&
    allocation.phase !== "lost";
  if (oldHealthyReplacement) return false;

  return (
    allocation.phase !== "running" ||
    allocation.health !== "healthy" ||
    !!allocation.reason ||
    !!allocation.message ||
    !!allocation.next_retry_at
  );
}

export function attentionAllocations(job: Job): Allocation[] {
  return (job.allocations ?? []).filter((allocation) =>
    allocationNeedsAttention(allocation, job.revision),
  );
}

export function jobStateLabel(state: JobState): string {
  switch (state) {
    case "ready":
      return "Ready";
    case "degraded":
      return "Degraded";
    case "converging":
      return "Converging";
  }
}

export function jobStateDescription(job: Job): string {
  const state = jobState(job);
  if (state === "ready") {
    return `${job.healthy} of ${job.desired} desired allocations are healthy.`;
  }
  if (state === "degraded") {
    const problem = attentionAllocations(job)[0];
    if (problem?.message) return problem.message;
    if (problem?.reason) return `Allocation reports ${humanizeReason(problem.reason)}.`;
    return `${job.healthy} of ${job.desired} desired allocations are healthy.`;
  }
  if (!job.allocations || job.allocations.length === 0) {
    return "No allocations have been placed yet.";
  }
  return `${job.running} of ${job.desired} allocations are running; ${job.healthy} are healthy.`;
}

export function operationalIssues(jobs: Job[], nodes: Node[]): OperationalIssue[] {
  const issues: OperationalIssue[] = [];

  for (const node of nodes) {
    if (node.status === "unhealthy") {
      issues.push({
        id: `node-unhealthy-${node.id}`,
        severity: "critical",
        title: `${node.host} is unhealthy`,
        description: "The node is not reporting healthy and its workloads may be unavailable.",
        action: "Inspect node",
        href: "/nodes",
      });
    } else if (node.status === "draining") {
      issues.push({
        id: `node-draining-${node.id}`,
        severity: "info",
        title: `${node.host} is draining`,
        description: "New allocations are blocked while existing workloads move away from this node.",
        action: "View progress",
        href: "/nodes",
      });
    }
  }

  for (const job of jobs) {
    const state = jobState(job);
    if (state === "ready") continue;

    const problems = attentionAllocations(job);
    const problem = mostImportantAllocation(problems);
    const href = problem
      ? `/jobs/${encodeURIComponent(job.name)}?allocation=${encodeURIComponent(problem.id)}`
      : `/jobs/${encodeURIComponent(job.name)}`;

    if (state === "degraded") {
      issues.push({
        id: `job-degraded-${job.name}`,
        severity: "critical",
        title: `${job.name} is degraded`,
        description: allocationDiagnostic(problem) ?? jobStateDescription(job),
        action: problem ? "Open diagnostic" : "Diagnose job",
        href,
        job: job.name,
        allocation: problem,
      });
      continue;
    }

    if (!job.allocations || job.allocations.length === 0) {
      issues.push({
        id: `job-unplaced-${job.name}`,
        severity: "warning",
        title: `${job.name} is waiting for placement`,
        description:
          "No allocations exist yet. Check node capacity, constraints, required host volumes, and node health.",
        action: "Inspect job",
        href,
        job: job.name,
      });
    } else if (problem?.reason || problem?.message || problem?.next_retry_at) {
      issues.push({
        id: `job-retrying-${job.name}`,
        severity: "warning",
        title: `${job.name} needs attention while converging`,
        description: allocationDiagnostic(problem) ?? jobStateDescription(job),
        action: "Open diagnostic",
        href,
        job: job.name,
        allocation: problem,
      });
    }
  }

  const rank: Record<IssueSeverity, number> = { critical: 0, warning: 1, info: 2 };
  return issues.sort((a, b) => rank[a.severity] - rank[b.severity]);
}

export function humanizeReason(reason: string): string {
  return reason.replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function allocationDiagnostic(allocation?: Allocation): string | null {
  if (!allocation) return null;
  const identity = `${allocation.group}/${allocation.task || "*"}`;
  if (allocation.message) return `${identity}: ${allocation.message}`;
  if (allocation.reason) return `${identity}: ${humanizeReason(allocation.reason)}`;
  if (allocation.next_retry_at) return `${identity}: a retry is scheduled.`;
  return `${identity}: lifecycle ${allocation.phase}, health ${allocation.health}.`;
}

function mostImportantAllocation(allocations: Allocation[]): Allocation | undefined {
  return [...allocations].sort((a, b) => allocationRank(a) - allocationRank(b))[0];
}

function allocationRank(allocation: Allocation): number {
  if (allocation.phase === "failed" || allocation.phase === "lost") return 0;
  if (allocation.health === "unhealthy") return 1;
  if (allocation.reason || allocation.message) return 2;
  if (allocation.next_retry_at) return 3;
  return 4;
}
