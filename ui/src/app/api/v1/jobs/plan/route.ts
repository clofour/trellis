import { NextRequest, NextResponse } from "next/server";
import {
  orchestratorHeaders,
  TRELLIS_URL,
  resolveDashboardNamespace,
} from "@/lib/orchestrator";

export async function POST(request: NextRequest) {
  const selected = resolveDashboardNamespace(request);
  if (selected.error) {
    return NextResponse.json({ error: selected.error }, { status: 403 });
  }

  try {
    const body = await request.json();
    if (!body?.spec || typeof body.spec !== "object") {
      return NextResponse.json({ error: "Missing job spec" }, { status: 400 });
    }
    if (body.spec.namespace !== selected.namespace) {
      return NextResponse.json(
        {
          error: `Manifest namespace ${JSON.stringify(body.spec.namespace)} does not match active namespace ${JSON.stringify(selected.namespace)}. Change the manifest or select the intended namespace.`,
        },
        { status: 422 },
      );
    }

    const res = await fetch(`${TRELLIS_URL}/v1/jobs/plan`, {
      method: "POST",
      headers: {
        ...orchestratorHeaders(selected.namespace),
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    const data: unknown = await res.json().catch(() => null);
    if (!res.ok) {
      if (typeof data === "object" && data !== null) {
        return NextResponse.json(data, { status: res.status });
      }
      return NextResponse.json(
        { error: `Upstream error: ${res.status}` },
        { status: res.status },
      );
    }
    return NextResponse.json(data);
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 },
    );
  }
}
