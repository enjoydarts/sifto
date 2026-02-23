import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
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
