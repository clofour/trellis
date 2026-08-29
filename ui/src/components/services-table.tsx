"use client";

import { useServices } from "@/hooks/use-api";
import type { PortMapping } from "@/lib/types";
import { EmptyState } from "./empty-state";
import { Skeleton } from "./skeleton";
import { StatusBadge } from "./status-badge";

function formatEndpoint(address: string, port: PortMapping) {
  const host = address.includes(":") && !address.startsWith("[")
    ? `[${address}]`
    : address;
  return `${host}:${port.host_port}`;
}

export function ServicesTable() {
  const { data: services, isLoading, error } = useServices();

  if (isLoading) return <TableSkeleton />;
  if (error) {
    return (
      <EmptyState
        title="Unable to load services"
        description="Could not read the service catalog from the orchestrator."
      />
    );
  }
  if (!services || services.length === 0) {
    return (
      <EmptyState
        title="No services"
        description="Healthy allocations with exposed ports will appear here."
      />
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/50">
            {['Service', 'Job / Group', 'Endpoint', 'Labels', 'Status'].map((heading) => (
              <th key={heading} className="px-4 py-3 text-left font-medium text-muted-foreground">
                {heading}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {services.map((service) => (
            <tr key={service.id} className="transition-colors hover:bg-muted/30">
              <td className="px-4 py-3">
                <p className="font-medium text-card-foreground">{service.id}</p>
                {service.namespace && (
                  <p className="mt-0.5 text-xs text-muted-foreground">{service.namespace}</p>
                )}
              </td>
              <td className="px-4 py-3">
                <p className="text-card-foreground">{service.job}</p>
                <p className="mt-0.5 text-xs text-muted-foreground">{service.group}</p>
              </td>
              <td className="px-4 py-3 font-mono text-xs text-card-foreground">
                {service.ports?.length ? (
                  <div className="space-y-1">
                    {service.ports.map((port) => (
                      <div key={`${port.host_port}-${port.container_port}`}>
                        {formatEndpoint(service.address, port)}
                        <span className="ml-1.5 text-muted-foreground">→ {port.container_port}</span>
                      </div>
                    ))}
                  </div>
                ) : service.address || "—"}
              </td>
              <td className="px-4 py-3">
                <div className="flex max-w-64 flex-wrap gap-1">
                  {Object.entries(service.labels ?? {}).length ? (
                    Object.entries(service.labels ?? {}).map(([key, value]) => (
                      <span key={key} className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground">
                        {key}={value}
                      </span>
                    ))
                  ) : <span className="text-muted-foreground">—</span>}
                </div>
              </td>
              <td className="px-4 py-3"><StatusBadge status={service.status} /></td>
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
      <div className="border-b border-border bg-muted/50 px-4 py-3"><Skeleton className="h-4 w-64" /></div>
      {Array.from({ length: 3 }).map((_, index) => (
        <div key={index} className="flex gap-8 border-b border-border px-4 py-3 last:border-0">
          <Skeleton className="h-8 w-36" />
          <Skeleton className="h-8 w-28" />
          <Skeleton className="h-8 w-40" />
        </div>
      ))}
    </div>
  );
}
