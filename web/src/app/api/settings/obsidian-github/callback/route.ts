import { NextRequest, NextResponse } from "next/server";
import { auth, currentUser } from "@clerk/nextjs/server";
import { getInternalAPISecret, getInternalAPISecretError } from "@/lib/internal-secret";
import { resolveServerAPIURL } from "@/lib/server-api-url";
import { createInstallationState, verifyInstallationState, authorizedInstallationRepositories, GITHUB_FLOW_COOKIE } from "@/lib/github-install-flow";
import { verifiedPrimaryEmail } from "@/lib/verified-primary-email";

function appBaseURL(req: NextRequest): string {
  return new URL("/", req.url).toString();
}

function redirectWithStatus(req: NextRequest, status: string) {
  return NextResponse.redirect(new URL(`/settings?obsidian_github=${status}`, appBaseURL(req)));
}

function resolveDisplayName(user: Awaited<ReturnType<typeof currentUser>>) {
  const fullName = user?.fullName?.trim();
  if (fullName) return fullName;
  const firstName = user?.firstName?.trim() ?? "";
  const lastName = user?.lastName?.trim() ?? "";
  const joined = `${firstName} ${lastName}`.trim();
  return joined || null;
}

export async function GET(req: NextRequest) {
  const clerkAuth = await auth();
  if (!clerkAuth.userId) {
    return NextResponse.redirect(new URL(`/login?callbackUrl=${encodeURIComponent("/settings")}`, appBaseURL(req)));
  }

  const secret = getInternalAPISecret();
  const flow = verifyInstallationState(req.nextUrl.searchParams.get("state") ?? "", req.cookies.get(GITHUB_FLOW_COOKIE)?.value ?? "", clerkAuth.userId, secret);
  if (!flow) return redirectWithStatus(req, "error&reason=invalid_state");
  if (!process.env.GITHUB_APP_CLIENT_ID || !process.env.GITHUB_APP_CLIENT_SECRET) return redirectWithStatus(req, "error&reason=disabled");
  const installationID = flow.installationId ?? Number(req.nextUrl.searchParams.get("installation_id"));
  if (!Number.isSafeInteger(installationID) || installationID <= 0) {
    return redirectWithStatus(req, "error&reason=invalid_installation");
  }

  const redirectURI = new URL("/api/settings/obsidian-github/callback", process.env.NEXTAUTH_URL ?? req.url).toString();
  const code = req.nextUrl.searchParams.get("code");
  if (!code) {
    const state = createInstallationState(clerkAuth.userId, secret, installationID);
    const authorize = new URL("https://github.com/login/oauth/authorize");
    authorize.searchParams.set("client_id", process.env.GITHUB_APP_CLIENT_ID);
    authorize.searchParams.set("redirect_uri", redirectURI);
    authorize.searchParams.set("state", state);
    const response = NextResponse.redirect(authorize);
    response.cookies.set(GITHUB_FLOW_COOKIE, state, { httpOnly: true, secure: req.nextUrl.protocol === "https:", sameSite: "lax", path: "/api/settings/obsidian-github", maxAge: 600 });
    return response;
  }
  let allowedRepositories: string[];
  try { allowedRepositories = await authorizedInstallationRepositories(code, redirectURI, installationID); }
  catch { return redirectWithStatus(req, "error&reason=unauthorized_installation"); }

  const user = await currentUser();
  const email = user?.id === clerkAuth.userId ? verifiedPrimaryEmail(user) : null;
  if (!email) {
    return redirectWithStatus(req, "error&reason=email_missing");
  }

  if (!secret) {
    return NextResponse.json({ error: getInternalAPISecretError() }, { status: 500 });
  }

  const apiURL = resolveServerAPIURL();
  const resolveIdentity = await fetch(`${apiURL}/api/internal/users/resolve-identity`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Internal-Secret": secret,
    },
    body: JSON.stringify({
      provider: "clerk",
      provider_user_id: clerkAuth.userId,
      email,
      name: resolveDisplayName(user),
    }),
    cache: "no-store",
  });
  if (!resolveIdentity.ok) {
    return redirectWithStatus(req, "error&reason=identity");
  }
  const identityJSON = (await resolveIdentity.json().catch(() => null)) as { id?: string } | null;
  const internalUserID = identityJSON?.id?.trim();
  if (!internalUserID) {
    return redirectWithStatus(req, "error&reason=identity");
  }

  const saveRes = await fetch(`${apiURL}/api/internal/settings/obsidian-github/installation`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Internal-Secret": secret,
    },
    body: JSON.stringify({
      user_id: internalUserID,
      installation_id: installationID,
      authorized_repositories: allowedRepositories,
    }),
    cache: "no-store",
  });
  if (!saveRes.ok) {
    return redirectWithStatus(req, "error&reason=save_failed");
  }

  const response = redirectWithStatus(req, "connected");
  response.cookies.delete({ name: GITHUB_FLOW_COOKIE, path: "/api/settings/obsidian-github" });
  return response;
}
