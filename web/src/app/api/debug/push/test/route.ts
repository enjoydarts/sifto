import { NextRequest, NextResponse } from "next/server";
import { getServerSession } from "next-auth";
import { authOptions } from "@/lib/auth";

export async function POST(req: NextRequest) {
  const session = await getServerSession(authOptions);
  if (!session) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  const secret = process.env.NEXTAUTH_SECRET ?? "";
  if (!secret) {
    return NextResponse.json({ error: "NEXTAUTH_SECRET is not set" }, { status: 500 });
  }
  const externalId = session.user?.email ?? null;
  if (!externalId) {
    return NextResponse.json({ error: "session email is missing" }, { status: 400 });
  }

  const payload = await req.json().catch(() => ({}));
  const subscriptionId =
    payload && typeof payload.subscription_id === "string"
      ? payload.subscription_id.trim()
      : "";
  const body = JSON.stringify({
    ...payload,
    external_id: subscriptionId ? undefined : externalId,
    subscription_id: subscriptionId || undefined,
  });

  const res = await fetch(`${apiUrl}/api/internal/debug/push/test`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Internal-Secret": secret,
    },
    body,
    cache: "no-store",
  });
  const text = await res.text();
  return new NextResponse(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") ?? "application/json" },
  });
}
