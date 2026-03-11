import { clerkMiddleware, createRouteMatcher } from "@clerk/nextjs/server";
import { NextResponse } from "next/server";

const isPublicRoute = createRouteMatcher([
  "/login(.*)",
  "/health(.*)",
  "/api/auth(.*)",
  "/onesignal/(.*)",
  "/OneSignalSDKWorker.js",
  "/OneSignalSDKUpdaterWorker.js",
  "/_next/static/(.*)",
  "/_next/image(.*)",
  "/favicon.ico",
  "/logo.png",
]);

function hasLegacySessionCookie(req: Request & { cookies: { get: (name: string) => { value?: string } | undefined } }) {
  return Boolean(
    req.cookies.get("next-auth.session-token")?.value ||
      req.cookies.get("__Secure-next-auth.session-token")?.value
  );
}

export default clerkMiddleware(async (auth, req) => {
  if (isPublicRoute(req) || hasLegacySessionCookie(req)) {
    return NextResponse.next();
  }

  const { userId } = await auth();
  if (userId) {
    return NextResponse.next();
  }

  const loginURL = new URL("/login", req.url);
  const callbackURL = `${req.nextUrl.pathname}${req.nextUrl.search}`;
  if (callbackURL && callbackURL !== "/login") {
    loginURL.searchParams.set("callbackUrl", callbackURL);
  }
  return NextResponse.redirect(loginURL);
});

export const config = {
  matcher: ["/((?!.*\\..*|_next).*)", "/", "/(api|trpc)(.*)"],
};
