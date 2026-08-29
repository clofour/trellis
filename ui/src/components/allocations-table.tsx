"use client";

import type { Allocation } from "@/lib/types";
import { StatusBadge } from "./status-badge";
import { EmptyState } from "./empty-state";

export function AllocationsTable({
  allocations,
}: {
  allocations: Allocation[] | null;
}) {
  if (!allocations || allocations.length === 0) {
    return (
      <EmptyState
        title="No allocations"
        description="This job has no running allocations."
      />
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/50">
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              ID
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Task Group
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Task
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Node
            </th>
            <th className="px-4 py-3 text-left font-medium text-muted-foreground">
              Status
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {allocations.map((alloc) => (
            <tr
              key={alloc.id}
              className="transition-colors hover:bg-muted/30"
            >
              <td className="px-4 py-3 font-mono text-xs text-card-foreground">
                {alloc.id}
              </td>
              <td className="px-4 py-3 text-card-foreground">{alloc.group}</td>
              <td className="px-4 py-3 text-card-foreground">{alloc.task}</td>
              <td className="px-4 py-3 font-mono text-xs text-muted-foreground">
                {alloc.node_id ? alloc.node_id.substring(0, 8) : "—"}
              </td>
              <td className="px-4 py-3">
                <StatusBadge status={alloc.status} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
