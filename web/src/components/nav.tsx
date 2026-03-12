"use client";

import { useEffect, useRef, useState } from "react";
import { useAuth, useClerk, useUser } from "@clerk/nextjs";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Activity,
  Brain,
  Bug,
  Layers3,
  Mail,
  Newspaper,
  Rss,
  Search,
  Settings as SettingsIcon,
  Sparkles,
  Star,
  type LucideIcon,
} from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import PWAInstallButton from "@/components/pwa-install";

const primaryLinks = [
  { href: "/", labelKey: "nav.briefing", icon: Sparkles },
  { href: "/clusters", labelKey: "nav.clusters", icon: Layers3 },
  { href: "/items", labelKey: "nav.items", icon: Newspaper },
  { href: "/pulse", labelKey: "nav.pulse", icon: Activity },
];

const secondaryLinks = [
  { href: "/favorites", labelKey: "nav.favorites", icon: Star },
  { href: "/sources", labelKey: "nav.sources", icon: Rss },
  { href: "/digests", labelKey: "nav.digests", icon: Mail },
  { href: "/ask", labelKey: "nav.ask", icon: Search },
  { href: "/llm-usage", labelKey: "nav.llmUsage", icon: Brain },
  { href: "/settings", labelKey: "nav.settings", icon: SettingsIcon },
  { href: "/debug/digests", labelKey: "nav.debug", icon: Bug },
];

type SharedNavProps = {
  displayName: string | null;
  hasSignedInUser: boolean;
  onSignOut: () => void;
};

export default function Nav() {
  const { isSignedIn, userId } = useAuth();
  const { user } = useUser();
  const clerk = useClerk();

  return (
    <NavShell
      displayName={
        user?.fullName ??
        user?.primaryEmailAddress?.emailAddress ??
        userId ??
        null
      }
      hasSignedInUser={Boolean(isSignedIn || userId)}
      onSignOut={() => clerk.signOut({ redirectUrl: "/login" })}
    />
  );
}

function NavShell({ displayName, hasSignedInUser, onSignOut }: SharedNavProps) {
  const pathname = usePathname();
  const { locale, setLocale, t } = useI18n();
  const [menuOpen, setMenuOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const moreMenuRef = useRef<HTMLDivElement | null>(null);

  const isActive = (href: string) => pathname === href || pathname?.startsWith(`${href}/`);
  const isMoreActive = secondaryLinks.some((v) => isActive(v.href));
  const showBottomNav = !/^\/items\/[^/]+/.test(pathname ?? "") && pathname !== "/triage";

  useEffect(() => {
    if (!moreOpen) return;
    const onDocClick = (e: MouseEvent) => {
      const root = moreMenuRef.current;
      if (!root) return;
      if (root.contains(e.target as Node)) return;
      setMoreOpen(false);
    };
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, [moreOpen]);

  return (
    <>
      <header className="sticky top-0 z-20 border-b border-zinc-200/80 bg-white/90 backdrop-blur">
        <div className="mx-auto max-w-6xl px-4 py-2">
          <div className="flex min-h-12 items-center gap-3">
            <Link href="/" className="flex items-center gap-2 press focus-ring rounded">
              <Image src="/logo.png" alt="Sifto" width={28} height={28} priority />
              <span className="text-lg font-bold tracking-tight text-zinc-900">Sifto</span>
            </Link>

            <div className="ml-auto flex items-center gap-2">
              <PWAInstallButton />
              <label className="sr-only">{t("nav.language")}</label>
              <select
                value={locale}
                onChange={(e) => setLocale(e.target.value as "ja" | "en")}
                className="rounded border border-zinc-200 bg-white px-2 py-1 text-xs text-zinc-600 focus-ring"
                aria-label={t("nav.language")}
              >
                <option value="ja">{t("nav.locale.ja")}</option>
                <option value="en">{t("nav.locale.en")}</option>
              </select>
              <button
                type="button"
                onClick={() => setMenuOpen((v) => !v)}
                className="rounded border border-zinc-200 px-3 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50 md:hidden press focus-ring"
                aria-expanded={menuOpen}
                aria-label={menuOpen ? t("nav.menu.close") : t("nav.menu.open")}
              >
                {menuOpen ? t("nav.menu.closeShort") : t("nav.menu.short")}
              </button>
            </div>
          </div>

          <div className="mt-2 hidden items-center gap-2 md:flex">
            <nav className="flex min-w-0 flex-1 flex-wrap gap-1">
              {primaryLinks.map(({ href, labelKey, icon: Icon }) => {
                const active = isActive(href);
                return (
                  <Link
                    key={href}
                    href={href}
                    onClick={() => setMoreOpen(false)}
                    className={`group inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors duration-150 press focus-ring ${
                      active
                        ? "bg-zinc-900 text-white"
                        : "text-zinc-500 hover:bg-zinc-50 hover:text-zinc-900"
                    }`}
                  >
                    <NavIcon icon={Icon} active={active} />
                    <span>{t(labelKey)}</span>
                  </Link>
                );
              })}
              <div className="relative" ref={moreMenuRef}>
                <button
                  type="button"
                  onClick={() => setMoreOpen((v) => !v)}
                  className={`group inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors duration-150 press focus-ring ${
                    secondaryLinks.some((v) => isActive(v.href))
                      ? "bg-zinc-900 text-white"
                      : "text-zinc-500 hover:bg-zinc-50 hover:text-zinc-900"
                  }`}
                  aria-expanded={moreOpen}
                >
                  <NavIcon icon={SettingsIcon} active={isMoreActive} />
                  <span>{t("nav.more")}</span>
                </button>
                {moreOpen && (
                  <div className="absolute left-0 top-10 z-30 w-52 rounded-xl border border-zinc-200 bg-white p-1 shadow-lg motion-safe:animate-scale-in">
                    {secondaryLinks.map(({ href, labelKey, icon: Icon }) => {
                      const active = isActive(href);
                      return (
                        <Link
                          key={href}
                          href={href}
                          onClick={() => setMoreOpen(false)}
                          className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors duration-150 press focus-ring ${
                            active ? "bg-zinc-900 text-white" : "text-zinc-700 hover:bg-zinc-50"
                          }`}
                        >
                          <NavIcon icon={Icon} active={active} />
                          <span>{t(labelKey)}</span>
                        </Link>
                      );
                    })}
                  </div>
                )}
              </div>
            </nav>
            {hasSignedInUser && (
              <>
                <span className="max-w-44 truncate text-sm text-zinc-500">
                  {displayName}
                </span>
                <button
                  onClick={onSignOut}
                  className="rounded-lg border border-zinc-200 px-3 py-1 text-xs font-medium text-zinc-600 hover:bg-zinc-50 hover:text-zinc-900 press focus-ring"
                >
                  {t("nav.signOut")}
                </button>
              </>
            )}
          </div>

          <div className="mt-2 flex items-center gap-1 overflow-x-auto md:hidden">
            <nav className="flex min-w-full items-center gap-1">
              {primaryLinks.map(({ href, labelKey, icon: Icon }) => {
                const active = isActive(href);
                return (
                  <Link
                    key={href}
                    href={href}
                    className={`group inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors duration-150 press focus-ring ${
                      active ? "bg-zinc-900 text-white" : "text-zinc-600 hover:bg-zinc-50"
                    }`}
                  >
                    <NavIcon icon={Icon} active={active} />
                    <span>{t(labelKey)}</span>
                  </Link>
                );
              })}
            </nav>
          </div>

          {menuOpen && (
            <div className="mt-2 rounded-xl border border-zinc-200 bg-white p-2 shadow-sm md:hidden motion-safe:animate-scale-in">
              <nav className="grid gap-1">
                {primaryLinks.map(({ href, labelKey, icon: Icon }) => {
                  const active = isActive(href);
                  return (
                    <Link
                      key={href}
                      href={href}
                      onClick={() => setMenuOpen(false)}
                      className={`inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors duration-150 press focus-ring ${
                        active
                          ? "bg-zinc-900 text-white"
                          : "text-zinc-700 hover:bg-zinc-50"
                      }`}
                    >
                      <NavIcon icon={Icon} active={active} />
                      <span>{t(labelKey)}</span>
                    </Link>
                  );
                })}
                <div className="my-1 h-px bg-zinc-100" />
                {secondaryLinks.map(({ href, labelKey, icon: Icon }) => {
                  const active = isActive(href);
                  return (
                    <Link
                      key={href}
                      href={href}
                      onClick={() => setMenuOpen(false)}
                      className={`inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors duration-150 press focus-ring ${
                        active
                          ? "bg-zinc-900 text-white"
                          : "text-zinc-700 hover:bg-zinc-50"
                      }`}
                    >
                      <NavIcon icon={Icon} active={active} />
                      <span>{t(labelKey)}</span>
                    </Link>
                  );
                })}
              </nav>
              {hasSignedInUser && (
                <div className="mt-2 border-t border-zinc-100 px-2 pt-2">
                  <div className="truncate text-xs text-zinc-500">
                    {displayName}
                  </div>
                  <button
                    onClick={onSignOut}
                    className="mt-2 w-full rounded-lg border border-zinc-200 px-3 py-2 text-left text-xs font-medium text-zinc-700 hover:bg-zinc-50 press focus-ring"
                  >
                    {t("nav.signOut")}
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </header>
      <nav
        className={`fixed inset-x-0 bottom-0 z-30 border-t border-zinc-200 bg-white/95 px-2 pb-[calc(env(safe-area-inset-bottom)+0.4rem)] pt-2 backdrop-blur md:hidden ${
          showBottomNav ? "" : "hidden"
        }`}
      >
        <div className="mx-auto grid max-w-md grid-cols-5 gap-1">
          {primaryLinks.map(({ href, labelKey, icon: Icon }) => {
            const active = isActive(href);
            return (
              <Link
                key={href}
                href={href}
                className={`relative flex min-h-12 flex-col items-center justify-center rounded-xl px-1 py-1 text-[11px] font-medium transition-colors duration-150 press focus-ring ${
                  active ? "text-zinc-900" : "text-zinc-500 hover:bg-zinc-50"
                }`}
              >
                <NavIcon icon={Icon} active={active} />
                <span className="mt-0.5 truncate">{t(labelKey)}</span>
                {active && (
                  <span className="absolute bottom-1 left-1/2 -translate-x-1/2 h-1 w-1 rounded-full bg-zinc-900" />
                )}
              </Link>
            );
          })}
          <Link
            href="/settings"
            className={`relative flex min-h-12 flex-col items-center justify-center rounded-xl px-1 py-1 text-[11px] font-medium transition-colors duration-150 press focus-ring ${
              isMoreActive ? "text-zinc-900" : "text-zinc-500 hover:bg-zinc-50"
            }`}
          >
            <NavIcon icon={SettingsIcon} active={isMoreActive} />
            <span className="mt-0.5 truncate">{t("nav.more")}</span>
            {isMoreActive && (
              <span className="absolute bottom-1 left-1/2 -translate-x-1/2 h-1 w-1 rounded-full bg-zinc-900" />
            )}
          </Link>
        </div>
      </nav>
    </>
  );
}

function NavIcon({ icon: Icon, active }: { icon: LucideIcon; active?: boolean }) {
  return (
    <Icon
      className={`size-4 shrink-0 transition-transform duration-150 group-hover:scale-110 ${active ? "scale-110" : ""}`}
      strokeWidth={2}
      aria-hidden="true"
    />
  );
}
