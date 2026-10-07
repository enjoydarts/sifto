import { createHmac, randomBytes, timingSafeEqual } from "node:crypto";

export const GITHUB_FLOW_COOKIE = "sifto_github_install_flow";
type InstallationFlow = { userId: string; nonce: string; expiresAt: number; installationId?: number };

export function createInstallationState(userId: string, secret: string, installationId?: number): string {
  if (secret.length < 32) throw new Error("installation flow secret is not configured");
  const body = Buffer.from(JSON.stringify({ userId, installationId, nonce: randomBytes(32).toString("hex"), expiresAt: Date.now() + 600_000 })).toString("base64url");
  return body + "." + createHmac("sha256", secret).update(body).digest("base64url");
}

export function verifyInstallationState(state: string, cookie: string, userId: string, secret: string): InstallationFlow | null {
  if (!state || state !== cookie || secret.length < 32 || state.length > 2048) return null;
  const [body, signature, extra] = state.split(".");
  if (!body || !signature || extra) return null;
  const expected = createHmac("sha256", secret).update(body).digest();
  const supplied = Buffer.from(signature, "base64url");
  if (supplied.length !== expected.length || !timingSafeEqual(expected, supplied)) return null;
  try {
    const flow = JSON.parse(Buffer.from(body, "base64url").toString()) as InstallationFlow;
    if (flow.userId !== userId || !flow.nonce || !Number.isFinite(flow.expiresAt) || flow.expiresAt <= Date.now()) return null;
    return flow;
  } catch { return null; }
}

export async function authorizedInstallationRepositories(code: string, redirectURI: string, installationId: number): Promise<string[]> {
  const tokenResponse = await fetch("https://github.com/login/oauth/access_token", {
    method: "POST", redirect: "error", cache: "no-store", signal: AbortSignal.timeout(10_000),
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify({ client_id: process.env.GITHUB_APP_CLIENT_ID, client_secret: process.env.GITHUB_APP_CLIENT_SECRET, code, redirect_uri: redirectURI }),
  });
  const token = await tokenResponse.json() as { access_token?: string; error?: string };
  if (!tokenResponse.ok || token.error || !token.access_token) throw new Error("GitHub user authorization failed");
  const repositories: string[] = [];
  for (let page = 1; page <= 10; page++) {
    const response = await fetch(`https://api.github.com/user/installations/${installationId}/repositories?per_page=100&page=${page}`, {
      redirect: "error", cache: "no-store", signal: AbortSignal.timeout(10_000),
      headers: { Authorization: `Bearer ${token.access_token}`, Accept: "application/vnd.github+json" },
    });
    if (!response.ok) throw new Error("installation is not authorized for this user");
    const result = await response.json() as { total_count?: number; repositories?: { full_name?: string; permissions?: { push?: boolean } }[] };
    if (!Array.isArray(result.repositories) || (result.total_count ?? 0) > 1000) throw new Error("invalid repository access response");
    for (const repo of result.repositories) {
      if (repo.permissions?.push && repo.full_name && /^[a-z0-9_.-]+\/[a-z0-9_.-]+$/i.test(repo.full_name)) repositories.push(repo.full_name.toLowerCase());
    }
    if (result.repositories.length < 100) break;
  }
  if (!repositories.length) throw new Error("no writable repositories for this installation");
  return [...new Set(repositories)];
}
