import { NextResponse } from "next/server";
import { orchestratorHeaders, TRELLIS_URL } from "@/lib/orchestrator";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  try {
    const res = await fetch(
      `${TRELLIS_URL}/v1/allocations/${encodeURIComponent(id)}/events`,
      { headers: orchestratorHeaders() },
    );
    if (!res.ok) {
      return NextResponse.json(
        { error: `Upstream error: ${res.status}` },
        { status: res.status },
      );
    }
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 },
    );
  }
}
