import { NextRequest, NextResponse } from "next/server";
import { auth } from "@clerk/nextjs/server";
import { getInternalAPISecret } from "@/lib/internal-secret";
import { createInstallationState, GITHUB_FLOW_COOKIE } from "@/lib/github-install-flow";

export async function GET(req: NextRequest) {
  const { userId } = await auth();
  if (!userId) return NextResponse.redirect(new URL("/login", req.url));
  if (!process.env.GITHUB_APP_INSTALL_URL || !process.env.GITHUB_APP_CLIENT_ID || !process.env.GITHUB_APP_CLIENT_SECRET || getInternalAPISecret().length < 32) {
    return NextResponse.json({ error: "github app user authorization is not configured" }, { status: 503 });
  }
  const url = new URL(process.env.GITHUB_APP_INSTALL_URL);
  if (url.origin !== "https://github.com") return NextResponse.json({ error: "invalid install URL" }, { status: 503 });
  const state = createInstallationState(userId, getInternalAPISecret());
  url.searchParams.set("state", state);
  const response = NextResponse.redirect(url);
  response.cookies.set(GITHUB_FLOW_COOKIE, state, { httpOnly: true, secure: req.nextUrl.protocol === "https:", sameSite: "lax", path: "/api/settings/obsidian-github", maxAge: 600 });
  return response;
}
