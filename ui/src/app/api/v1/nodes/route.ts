import { NextResponse } from "next/server";
import { orchestratorHeaders, TRELLIS_URL } from "@/lib/orchestrator";

export async function GET() {
  try {
    const res = await fetch(`${TRELLIS_URL}/v1/nodes`, {
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
