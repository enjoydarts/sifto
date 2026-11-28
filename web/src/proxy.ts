import { clerkMiddleware, createRouteMatcher } from "@clerk/nextjs/server";
import { NextResponse } from "next/server";
import { ITEMS_DOCUMENT_HEADER, itemsSSREnabled } from "@/lib/items-ssr-request";

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
  "/logo-transparent.png",
]);

export default clerkMiddleware(async (auth, req) => {
  const requestHeaders = new Headers(req.headers);
  requestHeaders.delete(ITEMS_DOCUMENT_HEADER);
  if (isPublicRoute(req)) {
    return NextResponse.next({ request: { headers: requestHeaders } });
  }

  const { userId } = await auth();
  if (userId) {
    if (itemsSSREnabled() && req.nextUrl.pathname === "/items" && req.headers.get("rsc") !== "1") {
      requestHeaders.set(ITEMS_DOCUMENT_HEADER, "1");
    }
    return NextResponse.next({ request: { headers: requestHeaders } });
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
