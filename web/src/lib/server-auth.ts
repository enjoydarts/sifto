import { auth, currentUser } from "@clerk/nextjs/server";
import { getServerSession } from "next-auth";
import { authOptions } from "@/lib/auth";

export interface ServerAuthUser {
  provider: "clerk" | "nextauth";
  email: string | null;
}

export async function getServerAuthUser(): Promise<ServerAuthUser | null> {
  const session = await getServerSession(authOptions);
  if (session?.user) {
    return {
      provider: "nextauth",
      email: session.user.email ?? null,
    };
  }

  const clerkEnabled = Boolean(
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY && process.env.CLERK_SECRET_KEY
  );
  if (!clerkEnabled) {
    return null;
  }

  const clerkAuth = await auth();
  if (!clerkAuth.userId) return null;

  const user = await currentUser();
  const primaryEmail =
    user?.emailAddresses.find((entry) => entry.id === user.primaryEmailAddressId)?.emailAddress ??
    user?.emailAddresses[0]?.emailAddress ??
    null;

  return {
    provider: "clerk",
    email: primaryEmail,
  };
}
