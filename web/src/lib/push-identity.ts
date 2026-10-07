import { createHmac } from "node:crypto";

export function pushIdentity(email: string, secret: string): string {
  if (secret.length < 32 || !email.trim()) throw new Error("push identity is not configured");
  return "sifto_v2_" + createHmac("sha256", secret).update("sifto:onesignal:v2:" + email.trim().toLowerCase()).digest("hex");
}
