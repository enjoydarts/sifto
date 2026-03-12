import { Suspense } from "react";
import LoginForm from "./login-form";

export default function LoginPage() {
  const showClerk = !!(
    process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY && process.env.CLERK_SECRET_KEY
  );

  return (
    <Suspense>
      <LoginForm showClerk={showClerk} />
    </Suspense>
  );
}
