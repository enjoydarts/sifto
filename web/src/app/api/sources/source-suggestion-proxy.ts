import { NextRequest, NextResponse } from "next/server";
import { resolveServerAPIURL } from "@/lib/server-api-url";

export async function proxySourceSuggestionRequest(req: NextRequest, upstreamPath: string) {
  const apiURL = resolveServerAPIURL();
  const authorization = req.headers.get("authorization")?.trim();
  const qs = req.nextUrl.searchParams.toString();
  const upstream = await fetch(`${apiURL}${upstreamPath}${qs ? `?${qs}` : ""}`, {
    method: "GET",
    headers: {
      ...(authorization ? { Authorization: authorization } : {}),
    },
    cache: "no-store",
  });

  const text = await upstream.text();
  return new NextResponse(text, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("Content-Type") ?? "application/json",
      "Cache-Control": "no-store",
    },
  });
}
