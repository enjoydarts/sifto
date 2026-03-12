import { NextResponse } from "next/server";
import { auth } from "@clerk/nextjs/server";

export async function GET() {
  const clerkAuth = await auth();
  if (!clerkAuth.userId) {
    return NextResponse.redirect(new URL("/login", process.env.NEXTAUTH_URL ?? "http://localhost:3000"));
  }

  const installURL = process.env.GITHUB_APP_INSTALL_URL?.trim();
  if (!installURL) {
    return NextResponse.json({ error: "github app is not configured" }, { status: 500 });
  }
  return NextResponse.redirect(installURL);
}
