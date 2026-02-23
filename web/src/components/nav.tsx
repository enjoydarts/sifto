"use client";

import { useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSession, signOut } from "next-auth/react";
import { useI18n } from "@/components/i18n-provider";

const links = [
  { href: "/", labelKey: "dashboard.title" },
  { href: "/items", labelKey: "nav.items" },
  { href: "/sources", labelKey: "nav.sources" },
  { href: "/digests", labelKey: "nav.digests" },
  { href: "/llm-usage", labelKey: "nav.llmUsage" },
  { href: "/settings", labelKey: "nav.settings" },
  { href: "/debug/digests", labelKey: "nav.debug" },
];

export default function Nav() {
  const pathname = usePathname();
  const { data: session } = useSession();
  const { locale, setLocale, t } = useI18n();
  const [menuOpen, setMenuOpen] = useState(false);

  const isActive = (href: string) => pathname === href || pathname.startsWith(`${href}/`);

  return (
    <header className="sticky top-0 z-20 border-b border-zinc-200/80 bg-white/90 backdrop-blur">
      <div className="mx-auto max-w-6xl px-4 py-2">
        <div className="flex min-h-12 items-center gap-3">
          <Link href="/" className="flex items-center gap-2">
            <Image src="/logo.png" alt="Sifto" width={28} height={28} priority />
            <span className="text-lg font-bold tracking-tight text-zinc-900">Sifto</span>
          </Link>

          <div className="ml-auto flex items-center gap-2">
            <label className="sr-only">{t("nav.language")}</label>
            <select
              value={locale}
              onChange={(e) => setLocale(e.target.value as "ja" | "en")}
              className="rounded border border-zinc-200 bg-white px-2 py-1 text-xs text-zinc-600"
              aria-label={t("nav.language")}
            >
              <option value="ja">日本語</option>
              <option value="en">English</option>
            </select>
            <button
              type="button"
              onClick={() => setMenuOpen((v) => !v)}
              className="rounded border border-zinc-200 px-3 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50 md:hidden"
              aria-expanded={menuOpen}
              aria-label={menuOpen ? "Close menu" : "Open menu"}
            >
              {menuOpen ? "Close" : "Menu"}
            </button>
          </div>
        </div>

        <div className="mt-2 hidden items-center gap-2 md:flex">
          <nav className="flex min-w-0 flex-1 flex-wrap gap-1">
            {links.map(({ href, labelKey }) => (
              <Link
                key={href}
                href={href}
                className={`rounded px-3 py-1.5 text-sm font-medium transition-colors ${
                  isActive(href)
                    ? "bg-zinc-900 text-white"
                    : "text-zinc-500 hover:bg-zinc-50 hover:text-zinc-900"
                }`}
              >
                {t(labelKey)}
              </Link>
            ))}
          </nav>
          {session?.user && (
            <>
              <span className="max-w-44 truncate text-sm text-zinc-500">
                {session.user.name ?? session.user.email}
              </span>
              <button
                onClick={() => signOut({ callbackUrl: "/login" })}
                className="rounded border border-zinc-200 px-3 py-1 text-xs font-medium text-zinc-600 hover:bg-zinc-50 hover:text-zinc-900"
              >
                {t("nav.signOut")}
              </button>
            </>
          )}
        </div>

        {menuOpen && (
          <div className="mt-2 rounded-xl border border-zinc-200 bg-white p-2 shadow-sm md:hidden">
            <nav className="grid gap-1">
              {links.map(({ href, labelKey }) => (
                <Link
                  key={href}
                  href={href}
                  onClick={() => setMenuOpen(false)}
                  className={`rounded px-3 py-2 text-sm font-medium ${
                    isActive(href)
                      ? "bg-zinc-900 text-white"
                      : "text-zinc-700 hover:bg-zinc-50"
                  }`}
                >
                  {t(labelKey)}
                </Link>
              ))}
            </nav>
            {session?.user && (
              <div className="mt-2 border-t border-zinc-100 px-2 pt-2">
                <div className="truncate text-xs text-zinc-500">
                  {session.user.name ?? session.user.email}
                </div>
                <button
                  onClick={() => signOut({ callbackUrl: "/login" })}
                  className="mt-2 w-full rounded border border-zinc-200 px-3 py-2 text-left text-xs font-medium text-zinc-700 hover:bg-zinc-50"
                >
                  {t("nav.signOut")}
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </header>
  );
}
