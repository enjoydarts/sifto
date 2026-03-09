import type { NextAuthOptions } from "next-auth";
import GoogleProvider from "next-auth/providers/google";
import CredentialsProvider from "next-auth/providers/credentials";
import { SignJWT, jwtVerify } from "jose";
import { resolveServerAPIURL } from "@/lib/server-api-url";

function isUUID(value: string | undefined | null): boolean {
  if (!value) return false;
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

export const authOptions: NextAuthOptions = {
  secret: process.env.NEXTAUTH_SECRET,
  session: { strategy: "jwt" },
  pages: {
    signIn: "/login",
  },
  providers: [
    // Google OAuth — registered only when credentials are present
    ...(process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET
      ? [
          GoogleProvider({
            clientId: process.env.GOOGLE_CLIENT_ID,
            clientSecret: process.env.GOOGLE_CLIENT_SECRET,
          }),
        ]
      : []),

    // Dev bypass — registered only when flag is set
    ...(process.env.ALLOW_DEV_AUTH_BYPASS === "true"
      ? [
          CredentialsProvider({
            name: "Dev Login",
            credentials: {},
            async authorize() {
              return {
                id: process.env.DEV_AUTH_USER_ID ?? "dev",
                name: "Dev User",
                email: "dev@localhost",
              };
            },
          }),
        ]
      : []),
  ],
  // デフォルトの JWE（暗号化）ではなく HS256 署名の平文 JWT を発行する。
  // これにより Go API が golang-jwt で直接検証できる。
  jwt: {
    async encode({ secret, token, maxAge }) {
      const key = new TextEncoder().encode(secret as string);
      return new SignJWT(token as Record<string, unknown>)
        .setProtectedHeader({ alg: "HS256" })
        .setIssuedAt()
        .setExpirationTime(
          Math.floor(Date.now() / 1000) + (maxAge ?? 30 * 24 * 60 * 60)
        )
        .sign(key);
    },
    async decode({ secret, token }) {
      if (!token) return null;
      const key = new TextEncoder().encode(secret as string);
      const { payload } = await jwtVerify(token, key, { algorithms: ["HS256"] });
      return payload as import("next-auth/jwt").JWT;
    },
  },
  callbacks: {
    async jwt({ token, user }) {
      if (user?.id && isUUID(user.id)) {
        token.sub = user.id;
        return token;
      }
      // 初回ログイン時: Go API でユーザーを upsert して内部 UUID を sub にセット
      if (user?.email) {
        const apiUrl = resolveServerAPIURL();
        const res = await fetch(`${apiUrl}/api/internal/users/upsert`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Internal-Secret": process.env.NEXTAUTH_SECRET ?? "",
          },
          body: JSON.stringify({
            email: user.email,
            name: user.name ?? null,
          }),
        });
        if (!res.ok) {
          const errorText = await res.text();
          throw new Error(`upsert user failed: ${res.status} ${errorText}`.trim());
        }
        const data = (await res.json()) as { id?: string };
        if (!isUUID(data.id)) {
          throw new Error("upsert user returned invalid internal user id");
        }
        token.sub = data.id;
      }
      if (!isUUID(token.sub)) {
        throw new Error("session token does not contain a valid internal user id");
      }
      return token;
    },
    async session({ session, token }) {
      if (session.user && token.sub) {
        (session.user as typeof session.user & { id: string }).id = token.sub;
      }
      return session;
    },
  },
};
