import { NextResponse } from "next/server";
import { orchestratorHeaders, TRELLIS_URL } from "@/lib/orchestrator";

export async function GET() {
  const namespace = process.env.TRELLIS_NAMESPACE || "";
  if (!namespace) {
    return NextResponse.json(
      { error: "TRELLIS_NAMESPACE is required for secret management" },
      { status: 400 },
    );
  }
  try {
    const res = await fetch(
      `${TRELLIS_URL}/v1/namespaces/${encodeURIComponent(namespace)}/secrets`,
      { headers: orchestratorHeaders(), cache: "no-store" },
    );
    if (!res.ok) {
      const text = await res.text();
      return NextResponse.json(
        { error: text || `Upstream error: ${res.status}` },
        { status: res.status },
      );
    }
    return NextResponse.json(await res.json(), {
      headers: { "Cache-Control": "no-store" },
    });
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 },
    );
  }
}
