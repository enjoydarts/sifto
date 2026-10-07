import { auth, currentUser } from "@clerk/nextjs/server";
import { NextResponse } from "next/server";
import { pushIdentity } from "@/lib/push-identity";
import { getInternalAPISecret } from "@/lib/internal-secret";
import { verifiedPrimaryEmail } from "@/lib/verified-primary-email";

export async function GET() {
  const { userId } = await auth();
  if (!userId) return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  const user = await currentUser();
  const email = user?.id === userId ? verifiedPrimaryEmail(user) : null;
  if (!email) return NextResponse.json({ error: "verified email required" }, { status: 403 });
  try {
    return NextResponse.json({ externalId: pushIdentity(email, getInternalAPISecret()), userId }, { headers: { "Cache-Control": "no-store" } });
  } catch {
    return NextResponse.json({ error: "push identity is not configured" }, { status: 503 });
  }
}
