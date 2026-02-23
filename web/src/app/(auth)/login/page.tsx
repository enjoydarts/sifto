import { Suspense } from "react";
import LoginForm from "./login-form";

export default function LoginPage() {
  const showDev = process.env.ALLOW_DEV_AUTH_BYPASS === "true";
  const showGoogle = !!(
    process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET
  );

  return (
    <Suspense>
      <LoginForm showDev={showDev} showGoogle={showGoogle} />
    </Suspense>
  );
}
