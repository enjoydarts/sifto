import { NextRequest, NextResponse } from "next/server";
import { auth, currentUser } from "@clerk/nextjs/server";
import { getInternalAPISecret, getInternalAPISecretError } from "@/lib/internal-secret";
import { resolveServerAPIURL } from "@/lib/server-api-url";

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

  const installationID = Number(req.nextUrl.searchParams.get("installation_id"));
  if (!Number.isFinite(installationID) || installationID <= 0) {
    return redirectWithStatus(req, "error&reason=invalid_installation");
  }

  const user = await currentUser();
  const email =
    user?.emailAddresses.find((entry) => entry.id === user.primaryEmailAddressId)?.emailAddress ??
    user?.emailAddresses[0]?.emailAddress ??
    "";
  if (!email) {
    return redirectWithStatus(req, "error&reason=email_missing");
  }

  const secret = getInternalAPISecret();
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
    }),
    cache: "no-store",
  });
  if (!saveRes.ok) {
    return redirectWithStatus(req, "error&reason=save_failed");
  }

  return redirectWithStatus(req, "connected");
}
