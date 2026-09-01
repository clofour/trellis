import { NextRequest, NextResponse } from "next/server";
import {
  orchestratorHeaders,
  TRELLIS_URL,
  resolveDashboardNamespace,
} from "@/lib/orchestrator";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const selected = resolveDashboardNamespace(request);
  if (selected.error) {
    return NextResponse.json({ error: selected.error }, { status: 403 });
  }
  const { id } = await params;
  const tail = request.nextUrl.searchParams.get("tail") || "200";
  try {
    const res = await fetch(
      `${TRELLIS_URL}/v1/allocations/${encodeURIComponent(id)}/logs?tail=${encodeURIComponent(tail)}`,
      { headers: orchestratorHeaders(selected.namespace), cache: "no-store" },
    );
    if (!res.ok) {
      return NextResponse.json(
        { error: `Upstream error: ${res.status}` },
        { status: res.status },
      );
    }
    return new NextResponse(await res.text(), {
      status: 200,
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 },
    );
  }
}
