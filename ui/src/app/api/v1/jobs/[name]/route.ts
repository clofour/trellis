import { NextResponse } from "next/server";

const TRELLIS_URL = process.env.TRELLIS_API_URL || "http://localhost:8128";
const TRELLIS_TOKEN = process.env.TRELLIS_API_TOKEN || "";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ name: string }> }
) {
  const { name } = await params;
  try {
    const res = await fetch(
      `${TRELLIS_URL}/v1/jobs/${encodeURIComponent(name)}`,
      {
        headers: {
          Authorization: `Bearer ${TRELLIS_TOKEN}`,
        },
      }
    );

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
