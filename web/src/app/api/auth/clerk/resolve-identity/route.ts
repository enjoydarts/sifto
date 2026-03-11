import { NextResponse } from "next/server";
import { auth, currentUser } from "@clerk/nextjs/server";
import { resolveServerAPIURL } from "@/lib/server-api-url";

function resolveDisplayName(user: Awaited<ReturnType<typeof currentUser>>) {
  const fullName = user?.fullName?.trim();
  if (fullName) return fullName;
  const firstName = user?.firstName?.trim() ?? "";
  const lastName = user?.lastName?.trim() ?? "";
  const joined = `${firstName} ${lastName}`.trim();
  return joined || null;
}

export async function POST() {
  const clerkAuth = await auth();
  if (!clerkAuth.userId) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const user = await currentUser();
  const email =
    user?.emailAddresses.find((entry) => entry.id === user.primaryEmailAddressId)?.emailAddress ??
    user?.emailAddresses[0]?.emailAddress ??
    "";
  if (!email) {
    return NextResponse.json({ error: "email missing" }, { status: 400 });
  }

  const secret = process.env.NEXTAUTH_SECRET ?? "";
  if (!secret) {
    return NextResponse.json({ error: "NEXTAUTH_SECRET is not set" }, { status: 500 });
  }

  const apiUrl = resolveServerAPIURL();
  const res = await fetch(`${apiUrl}/api/internal/users/resolve-identity`, {
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

  const text = await res.text();
  return new NextResponse(text, {
    status: res.status,
    headers: {
      "Content-Type": res.headers.get("Content-Type") ?? "application/json",
    },
  });
}
