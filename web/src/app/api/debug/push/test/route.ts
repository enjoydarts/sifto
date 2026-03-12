import { NextRequest, NextResponse } from "next/server";
import { getInternalAPISecret, getInternalAPISecretError } from "@/lib/internal-secret";
import { resolveServerAPIURL } from "@/lib/server-api-url";
import { getServerAuthUser } from "@/lib/server-auth";

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
  const externalId = user.email ?? null;
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
