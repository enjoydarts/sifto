import { NextResponse } from "next/server";
import { getServerSession } from "next-auth";
import { authOptions } from "@/lib/auth";

export async function GET() {
  const session = await getServerSession(authOptions);
  if (!session) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  const secret = process.env.NEXTAUTH_SECRET ?? "";
  if (!secret) {
    return NextResponse.json({ error: "NEXTAUTH_SECRET is not set" }, { status: 500 });
  }

  const start = Date.now();
  const res = await fetch(`${apiUrl}/api/internal/debug/system-status`, {
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

