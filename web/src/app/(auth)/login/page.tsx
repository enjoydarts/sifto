import { Suspense } from "react";
import LoginForm from "./login-form";

export default function LoginPage() {
  const showClerk = Boolean(process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY);

  return (
    <Suspense>
      <LoginForm showClerk={showClerk} />
    </Suspense>
  );
}
