import { withAuth } from "next-auth/middleware";
import { NextResponse } from "next/server";
import { jwtVerify } from "jose";

export default withAuth(
  function middleware() {
    return NextResponse.next();
  },
  {
    callbacks: {
      authorized: ({ token }) => !!token,
    },
    pages: {
      signIn: "/login",
    },
    // auth.ts の encode に合わせて HS256 でデコードする
    jwt: {
      decode: async ({ secret, token }) => {
        if (!token) return null;
        try {
          const key = new TextEncoder().encode(secret as string);
          const { payload } = await jwtVerify(token, key, {
            algorithms: ["HS256"],
          });
          return payload as import("next-auth/jwt").JWT;
        } catch {
          return null;
        }
      },
    },
  }
);

export const config = {
  // Protect all routes except login, NextAuth API, and static assets
  matcher: [
    "/((?!login|api/auth|_next/static|_next/image|favicon\\.ico|logo\\.png).*)",
  ],
};
