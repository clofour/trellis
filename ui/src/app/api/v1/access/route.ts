import { NextResponse } from "next/server";
import { getDashboardCredentialInfo } from "@/lib/orchestrator";

export async function GET() {
  try {
    return NextResponse.json(await getDashboardCredentialInfo());
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "Unable to inspect dashboard credential" },
      { status: 502 },
    );
  }
}
