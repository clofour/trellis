import { NextRequest, NextResponse } from "next/server";
import { orchestratorHeaders, TRELLIS_URL, getAllowWrites } from "@/lib/orchestrator";

export async function GET() {
  try {
    const res = await fetch(`${TRELLIS_URL}/v1/jobs`, {
      headers: orchestratorHeaders(),
    });

    if (!res.ok) {
      return NextResponse.json(
        { error: `Upstream error: ${res.status}` },
        { status: res.status }
      );
    }

    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 }
    );
  }
}

export async function POST(request: NextRequest) {
  if (!getAllowWrites()) {
    return NextResponse.json({ error: "Dashboard is read-only" }, { status: 403 });
  }
  try {
    const body = await request.json();
    const res = await fetch(`${TRELLIS_URL}/v1/jobs`, {
      method: "POST",
      headers: { ...orchestratorHeaders(), "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      const text = await res.text();
      return NextResponse.json(
        { error: text || `Upstream error: ${res.status}` },
        { status: res.status }
      );
    }

    return new NextResponse(null, { status: 202 });
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 }
    );
  }
}
