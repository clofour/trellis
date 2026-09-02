import { NextResponse } from "next/server";
import { detectDashboardAPIAccess } from "@/lib/orchestrator";

export async function GET() {
  return NextResponse.json({ api_access: await detectDashboardAPIAccess() });
}
