import { NextResponse } from "next/server";
import {
  detectDashboardAPIAccess,
  getConfiguredNamespaces,
  getDefaultNamespace,
  hasConfiguredNamespaceAllowlist,
  orchestratorHeaders,
  TRELLIS_URL,
} from "@/lib/orchestrator";

interface JobSummary {
  spec?: {
    namespace?: string;
  };
  allocations?: Array<{
    namespace?: string;
  }> | null;
}

export async function GET() {
  const defaultNamespace = getDefaultNamespace();
  if ((await detectDashboardAPIAccess()) !== "cluster") {
    return NextResponse.json(defaultNamespace ? [defaultNamespace] : []);
  }

  if (hasConfiguredNamespaceAllowlist()) {
    return NextResponse.json(getConfiguredNamespaces().filter(Boolean));
  }

  try {
    const res = await fetch(`${TRELLIS_URL}/v1/jobs`, {
      headers: orchestratorHeaders(null),
      cache: "no-store",
    });
    if (!res.ok) {
      return NextResponse.json(
        { error: `Upstream error: ${res.status}` },
        { status: res.status },
      );
    }

    const jobs = (await res.json()) as JobSummary[];
    const namespaces = new Set<string>();
    if (defaultNamespace) namespaces.add(defaultNamespace);
    for (const job of jobs) {
      const specNamespace = job.spec?.namespace?.trim();
      if (specNamespace) namespaces.add(specNamespace);
      for (const allocation of job.allocations ?? []) {
        const allocationNamespace = allocation.namespace?.trim();
        if (allocationNamespace) namespaces.add(allocationNamespace);
      }
    }

    const result = Array.from(namespaces);
    result.sort((a, b) => {
      if (a === defaultNamespace) return -1;
      if (b === defaultNamespace) return 1;
      return a.localeCompare(b);
    });
    return NextResponse.json(result);
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 },
    );
  }
}
