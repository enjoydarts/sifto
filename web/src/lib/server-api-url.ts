export function resolveServerAPIURL(): string {
  const explicit =
    process.env.NEXT_PUBLIC_API_URL?.trim() || process.env.API_URL?.trim();
  if (explicit) return explicit.replace(/\/+$/, "");
  if (process.env.VERCEL === "1" || process.env.NODE_ENV === "production") {
    return "https://sifto-api.fly.dev";
  }
  return "http://localhost:8080";
}

