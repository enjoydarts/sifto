import { NextRequest, NextResponse } from "next/server";
import { resolveServerAPIURL } from "@/lib/server-api-url";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";
export const maxDuration = 300;

export async function POST(
  req: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const itemID = id?.trim();
  if (!itemID) {
    return NextResponse.json({ error: "invalid item id" }, { status: 400 });
  }

  const apiURL = resolveServerAPIURL();
  const authorization = req.headers.get("authorization")?.trim();
  const contentType = req.headers.get("content-type")?.trim();

  const upstream = await fetch(`${apiURL}/api/summary-audio/items/${encodeURIComponent(itemID)}/synthesize`, {
    method: "POST",
    headers: {
      ...(authorization ? { Authorization: authorization } : {}),
      ...(contentType ? { "Content-Type": contentType } : {}),
    },
    cache: "no-store",
  });

  const body = await upstream.arrayBuffer();
  const headers = new Headers();
  headers.set("Content-Type", upstream.headers.get("Content-Type") ?? "audio/mpeg");
  headers.set("Cache-Control", "no-store");
  const durationSec = upstream.headers.get("X-Summary-Audio-Duration-Sec");
  if (durationSec) {
    headers.set("X-Summary-Audio-Duration-Sec", durationSec);
  }
  const preprocessedText = upstream.headers.get("X-Summary-Audio-Preprocessed-Text-B64");
  if (preprocessedText) {
    headers.set("X-Summary-Audio-Preprocessed-Text-B64", preprocessedText);
  }
  return new NextResponse(body, {
    status: upstream.status,
    headers,
  });
}
