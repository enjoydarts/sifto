import { NextRequest, NextResponse } from "next/server";
import { getServerSession } from "next-auth";
import { resolveServerAPIURL } from "@/lib/server-api-url";
import { authOptions } from "@/lib/auth";

export async function GET(req: NextRequest) {
  const session = await getServerSession(authOptions);
  if (!session) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const apiUrl = resolveServerAPIURL();
  const secret = process.env.NEXTAUTH_SECRET ?? "";
  if (!secret) {
    return NextResponse.json({ error: "NEXTAUTH_SECRET is not set" }, { status: 500 });
  }

  const q = new URLSearchParams();
  const userID = req.nextUrl.searchParams.get("user_id")?.trim();
  if (userID) q.set("user_id", userID);
  const qs = q.toString();

  const start = Date.now();
  const res = await fetch(`${apiUrl}/api/internal/debug/system-status${qs ? `?${qs}` : ""}`, {
    method: "GET",
    headers: {
      "X-Internal-Secret": secret,
    },
    cache: "no-store",
  });
  const latencyMs = Date.now() - start;
  const text = await res.text();

  let parsed: unknown = null;
  try {
    parsed = text ? JSON.parse(text) : null;
  } catch {
    parsed = { raw: text };
  }

  return NextResponse.json(
    {
      proxy_status: res.status,
      proxy_latency_ms: latencyMs,
      data: parsed,
    },
    { status: res.ok ? 200 : res.status }
  );
}
