import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    const explicit = process.env.NEXT_PUBLIC_API_URL ?? process.env.API_URL;
    const apiUrl = explicit && explicit.trim()
      ? explicit.trim().replace(/\/+$/, "")
      : (process.env.VERCEL === "1" || process.env.NODE_ENV === "production")
        ? "https://sifto-api.fly.dev"
        : "http://localhost:8080";
    // /api/auth/* は NextAuth が処理するため除外し、それ以外を Go API にプロキシする
    return [
      {
        source: "/api/:path((?!auth(?:/|$)).+)",
        destination: `${apiUrl}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
