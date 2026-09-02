"use client";

import { useState } from "react";
import type { Allocation } from "@/lib/types";
import { StatusBadge } from "./status-badge";
import { EmptyState } from "./empty-state";
import { AllocationDetail } from "./allocation-detail";

export function AllocationsTable({
  allocations,
  initialAllocationId,
}: {
  allocations: Allocation[] | null;
  initialAllocationId?: string;
}) {
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [initialDismissed, setInitialDismissed] = useState(false);
  const effectiveID = selectedID ?? (initialDismissed ? undefined : initialAllocationId);
  const selected = (allocations ?? []).find(
    (allocation) => allocation.id === effectiveID,
  );

  if (!allocations || allocations.length === 0) {
    return (
      <EmptyState
        title="No allocations"
        description="This job has no allocations."
      />
    );
  }

  return (
    <>
      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/50">
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">ID</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Task Group</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Node</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Lifecycle</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Health</th>
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Attempt</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {allocations.map((alloc) => (
              <tr
                key={alloc.id}
                onClick={() => setSelectedID(alloc.id)}
                className="cursor-pointer transition-colors hover:bg-muted/30"
                title="Open allocation diagnostics"
              >
                <td className="px-4 py-3 font-mono text-xs text-card-foreground">
                  <span className="underline-offset-2 hover:underline">{alloc.id}</span>
                </td>
                <td className="px-4 py-3 text-card-foreground">{alloc.group}</td>
                <td className="px-4 py-3 font-mono text-xs text-muted-foreground">
                  {alloc.node_id ? alloc.node_id.substring(0, 8) : "—"}
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={alloc.phase} />
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={alloc.health} />
                </td>
                <td className="px-4 py-3 tabular-nums text-muted-foreground">
                  {alloc.attempt ?? 0}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {selected && (
        <AllocationDetail
          allocation={selected}
          onClose={() => {
            setSelectedID(null);
            setInitialDismissed(true);
          }}
        />
      )}
    </>
  );
}
