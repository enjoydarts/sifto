import { Suspense } from "react";
import LoginForm from "./login-form";

export default function LoginPage() {
  const showClerk = !!(
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY && process.env.CLERK_SECRET_KEY
  );
  const showDev = process.env.ALLOW_DEV_AUTH_BYPASS === "true";
  const showGoogle = !!(
    process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET
  );

  return (
    <Suspense>
      <LoginForm showClerk={showClerk} showDev={showDev} showGoogle={showGoogle} />
    </Suspense>
  );
}
