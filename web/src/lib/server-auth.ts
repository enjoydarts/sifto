import { auth, currentUser } from "@clerk/nextjs/server";
import { verifiedPrimaryEmail } from "@/lib/verified-primary-email";

export interface ServerAuthUser {
  provider: "clerk";
  email: string | null;
}

export async function getServerAuthUser(): Promise<ServerAuthUser | null> {
  const clerkEnabled = Boolean(
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY && process.env.CLERK_SECRET_KEY
  );
  if (!clerkEnabled) {
    return null;
  }

  const clerkAuth = await auth();
  if (!clerkAuth.userId) return null;

  const user = await currentUser();
  const primaryEmail = user?.id === clerkAuth.userId ? verifiedPrimaryEmail(user) : null;

  return {
    provider: "clerk",
    email: primaryEmail,
  };
}
