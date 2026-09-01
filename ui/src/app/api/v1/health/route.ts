import { NextResponse } from "next/server";
import { orchestratorHeaders, TRELLIS_URL } from "@/lib/orchestrator";

export async function GET() {
  try {
    const res = await fetch(`${TRELLIS_URL}/v1/nodes`, {
      headers: orchestratorHeaders(),
      signal: AbortSignal.timeout(5000),
    });

    if (!res.ok) {
      return NextResponse.json(
        { status: "error", message: `Upstream error: ${res.status}` },
        { status: 502 },
      );
    }

    return NextResponse.json({ status: "ok" });
  } catch {
    return NextResponse.json(
      { status: "error", message: "Failed to connect to orchestrator" },
      { status: 502 },
    );
  }
}
