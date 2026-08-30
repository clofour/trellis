import { NextRequest, NextResponse } from "next/server";
import {
  orchestratorHeaders,
  TRELLIS_URL,
  getAllowWrites,
} from "@/lib/orchestrator";

function namespacePath(name: string) {
  const namespace = process.env.TRELLIS_NAMESPACE || "";
  if (!namespace) return null;
  return `${TRELLIS_URL}/v1/namespaces/${encodeURIComponent(namespace)}/secrets/${encodeURIComponent(name)}`;
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ name: string }> },
) {
  if (!getAllowWrites()) {
    return NextResponse.json(
      { error: "Dashboard is read-only" },
      { status: 403 },
    );
  }
  const { name } = await params;
  const url = namespacePath(name);
  if (!url) {
    return NextResponse.json(
      { error: "TRELLIS_NAMESPACE is required for secret management" },
      { status: 400 },
    );
  }
  try {
    const body = await request.json();
    const res = await fetch(url, {
      method: "PUT",
      headers: {
        ...orchestratorHeaders(),
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });
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

export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ name: string }> },
) {
  if (!getAllowWrites()) {
    return NextResponse.json(
      { error: "Dashboard is read-only" },
      { status: 403 },
    );
  }
  const { name } = await params;
  const url = namespacePath(name);
  if (!url) {
    return NextResponse.json(
      { error: "TRELLIS_NAMESPACE is required for secret management" },
      { status: 400 },
    );
  }
  try {
    const res = await fetch(url, {
      method: "DELETE",
      headers: orchestratorHeaders(),
    });
    if (!res.ok) {
      const text = await res.text();
      return NextResponse.json(
        { error: text || `Upstream error: ${res.status}` },
        { status: res.status },
      );
    }
    return new NextResponse(null, { status: 204 });
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to orchestrator" },
      { status: 502 },
    );
  }
}
