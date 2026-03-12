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

  const body = await req.text();
  const res = await fetch(`${apiUrl}/api/internal/debug/digests/send`, {
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
