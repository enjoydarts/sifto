import { NextRequest } from "next/server";
import { proxySourceSuggestionRequest } from "../source-suggestion-proxy";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";
export const maxDuration = 300;

export async function GET(req: NextRequest) {
  return proxySourceSuggestionRequest(req, "/api/sources/recommended");
}
