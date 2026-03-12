"use client";

import { SignIn } from "@clerk/nextjs";
import Image from "next/image";
import { useSearchParams } from "next/navigation";
import { useI18n } from "@/components/i18n-provider";

interface Props {
  showClerk: boolean;
}

export default function LoginForm({ showClerk }: Props) {
  const { t } = useI18n();
  const searchParams = useSearchParams();
  const callbackUrl = searchParams.get("callbackUrl") ?? "/items";

  return (
    <div className="flex min-h-screen items-center justify-center bg-[radial-gradient(circle_at_top,_#ffffff,_#f4f4f5_55%,_#e4e4e7)] px-4 py-10">
      <div className="w-full max-w-lg">
        <div className="mb-8 flex flex-col items-center gap-3 text-center">
          <Image src="/logo.png" alt="Sifto" width={56} height={56} priority />
          <h1 className="text-2xl font-bold tracking-tight text-zinc-900">Sifto</h1>
          <p className="text-sm text-zinc-500">{t("login.subtitle")}</p>
        </div>

        <div className="flex flex-col gap-4">
          {showClerk ? (
            <SignIn
              routing="hash"
              fallbackRedirectUrl={callbackUrl}
              forceRedirectUrl={callbackUrl}
              appearance={{
                elements: {
                  rootBox: "mx-auto block w-full",
                  card: "mx-auto w-full rounded-3xl border border-zinc-200 bg-white p-6 shadow-[0_20px_60px_-30px_rgba(24,24,27,0.45)] md:p-8",
                  headerTitle: "hidden",
                  headerSubtitle: "hidden",
                  socialButtonsBlockButton:
                    "min-h-12 border-zinc-300 text-zinc-700 shadow-none hover:bg-zinc-50",
                  socialButtonsBlockButtonText: "font-medium",
                  dividerLine: "bg-zinc-200",
                  dividerText: "text-zinc-400",
                  formFieldLabel: "text-zinc-700",
                  formFieldInput:
                    "min-h-12 border-zinc-300 shadow-none focus:border-zinc-500 focus:ring-zinc-500",
                  formButtonPrimary:
                    "min-h-12 bg-zinc-900 hover:bg-zinc-700 text-white shadow-none",
                  footerActionText: "text-zinc-500",
                  footerActionLink: "text-zinc-900 hover:text-zinc-700",
                  identityPreviewText: "text-zinc-700",
                },
              }}
            />
          ) : null}

          {!showClerk && (
            <div className="rounded-2xl border border-dashed border-zinc-300 bg-white px-4 py-6 text-center text-sm text-zinc-400 shadow-sm">
              {t("login.noProvider")}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
