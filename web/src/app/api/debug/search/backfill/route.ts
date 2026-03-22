import { NextRequest, NextResponse } from "next/server";
import { getInternalAPISecret, getInternalAPISecretError } from "@/lib/internal-secret";
import { resolveServerAPIURL } from "@/lib/server-api-url";
import { getServerAuthUser } from "@/lib/server-auth";

export async function GET(req: NextRequest) {
  const user = await getServerAuthUser();
  if (!user) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const apiUrl = resolveServerAPIURL();
  const secret = getInternalAPISecret();
  if (!secret) {
    return NextResponse.json({ error: getInternalAPISecretError() }, { status: 500 });
  }

  const qs = new URLSearchParams();
  const limit = req.nextUrl.searchParams.get("limit");
  if (limit) qs.set("limit", limit);

  const res = await fetch(`${apiUrl}/api/internal/debug/search/backfill${qs.size ? `?${qs.toString()}` : ""}`, {
    method: "GET",
    headers: {
      "X-Internal-Secret": secret,
    },
    cache: "no-store",
  });

  const text = await res.text();
  return new NextResponse(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") ?? "application/json" },
  });
}

export async function POST(req: NextRequest) {
  const user = await getServerAuthUser();
  if (!user) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const apiUrl = resolveServerAPIURL();
  const secret = getInternalAPISecret();
  if (!secret) {
    return NextResponse.json({ error: getInternalAPISecretError() }, { status: 500 });
  }

  let payload: { offset?: number; limit?: number; all?: boolean } = {};
  try {
    payload = (await req.json()) as { offset?: number; limit?: number; all?: boolean };
  } catch {
    payload = {};
  }

  const qs = new URLSearchParams();
  if (typeof payload.offset === "number") qs.set("offset", String(payload.offset));
  if (typeof payload.limit === "number") qs.set("limit", String(payload.limit));
  if (payload.all === true) qs.set("all", "1");

  const res = await fetch(`${apiUrl}/api/internal/debug/search/backfill${qs.size ? `?${qs.toString()}` : ""}`, {
    method: "POST",
    headers: {
      "X-Internal-Secret": secret,
    },
    cache: "no-store",
  });

  const text = await res.text();
  return new NextResponse(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") ?? "application/json" },
  });
}
