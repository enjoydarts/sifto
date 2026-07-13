import { getServerAuthUser, type ServerAuthUser } from "@/lib/server-auth";

export type DebugAdminAuthorization =
  | { authorized: true; user: ServerAuthUser }
  | { authorized: false; status: 401 | 403; error: "unauthorized" | "forbidden" };

export async function authorizeDebugAdmin(): Promise<DebugAdminAuthorization> {
  const user = await getServerAuthUser();
  if (!user) {
    return { authorized: false, status: 401, error: "unauthorized" };
  }

  const email = user.email?.trim().toLowerCase() ?? "";
  const allowed = new Set(
    (process.env.PROMPT_ADMIN_EMAILS ?? "")
      .split(",")
      .map((value) => value.trim().toLowerCase())
      .filter(Boolean)
  );
  if (!email || !allowed.has(email)) {
    return { authorized: false, status: 403, error: "forbidden" };
  }
  return { authorized: true, user };
}

export function internalAdminEmailHeader(user: ServerAuthUser): Record<string, string> {
  return { "X-Internal-User-Email": user.email?.trim().toLowerCase() ?? "" };
}
