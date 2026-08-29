"use client";

import { useNodes } from "@/hooks/use-api";
import { StatusBadge } from "./status-badge";
import { formatCPU, formatBytes, timeAgo } from "@/lib/utils";
import { EmptyState } from "./empty-state";
import { Skeleton } from "./skeleton";

export function NodesTable() {
  const { data: nodes, isLoading, error } = useNodes();

  if (isLoading) return <TableSkeleton />;
  if (error) {
    return (
      <EmptyState
        title="Unable to load nodes"
        description="Could not connect to the orchestrator. Ensure it is running and the UI is configured."
      />
    );
  }
  if (!nodes || nodes.length === 0) {
    return (
      <EmptyState
        title="No nodes"
        description="No nodes have registered with the orchestrator yet."
      />
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/50">
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Host
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Status
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              CPU
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Memory
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Last Heartbeat
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {nodes.map((node) => (
            <tr
              key={node.id}
              className="transition-colors hover:bg-muted/30"
            >
              <td className="px-4 py-3">
                <div>
                  <p className="font-medium text-card-foreground">
                    {node.host}
                  </p>
                  <p className="mt-0.5 font-mono text-xs text-muted-foreground">
                    {node.id.substring(0, 8)}
                  </p>
                </div>
              </td>
              <td className="px-4 py-3">
                <StatusBadge status={node.status} />
              </td>
              <td className="px-4 py-3 tabular-nums text-card-foreground">
                {formatCPU(node.cpu)}
              </td>
              <td className="px-4 py-3 tabular-nums text-card-foreground">
                {formatBytes(node.memory)}
              </td>
              <td className="px-4 py-3 text-muted-foreground">
                {timeAgo(node.last_heartbeat)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
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
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-4 w-16" />
        </div>
      ))}
    </div>
  );
}
