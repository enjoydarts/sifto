"use client";

import { useEffect, useRef, useState } from "react";
import { useAuth, useClerk, useUser } from "@clerk/nextjs";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Brain,
  Bug,
  CheckCheck,
  ChevronDown,
  ChevronRight,
  History,
  RefreshCw,
  Menu,
  X,
  Layers3,
  Link2,
  Mail,
  Newspaper,
  Radio,
  Rss,
  Search,
  Target,
  TableProperties,
  Settings as SettingsIcon,
  Sparkles,
  Star,
  type LucideIcon,
} from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import PWAInstallButton from "@/components/pwa-install";
import { api, enableForceFreshReload, OpenRouterSyncRun } from "@/lib/api";

const primaryLinks = [
  { href: "/", labelKey: "nav.briefing", icon: Sparkles },
  { href: "/items?feed=unread&sort=newest", activeHref: "/items", labelKey: "nav.inbox", icon: Newspaper },
  { href: "/triage", labelKey: "nav.triage", icon: CheckCheck },
  { href: "/ask", labelKey: "nav.ask", icon: Search },
];

const secondaryLinks = [
  { href: "/clusters", labelKey: "nav.clusters", icon: Layers3 },
  { href: "/pulse", labelKey: "nav.pulse", icon: Sparkles },
  { href: "/goals", labelKey: "nav.goals", icon: Target },
  { href: "/favorites", labelKey: "nav.favorites", icon: Star },
  { href: "/sources", labelKey: "nav.sources", icon: Rss },
  { href: "/digests", labelKey: "nav.digests", icon: Mail },
  { href: "/audio-briefings", labelKey: "nav.audioBriefings", icon: Radio },
  { href: "/ai-navigator-briefs", labelKey: "nav.aiNavigatorBriefs", icon: Brain },
  { href: "/playback-history", labelKey: "nav.playbackHistory", icon: History },
  { href: "/llm-usage", labelKey: "nav.llmUsage", icon: Brain },
  { href: "/llm-analysis", labelKey: "nav.llmAnalysis", icon: TableProperties },
  { href: "/provider-model-snapshots", labelKey: "nav.providerModelSnapshots", icon: Link2 },
  { href: "/poe-models", labelKey: "nav.poeModels", icon: Link2 },
  { href: "/openrouter-models", labelKey: "nav.openrouterModels", icon: Link2 },
  { href: "/aivis-models", labelKey: "nav.aivisModels", icon: Link2 },
  { href: "/settings", labelKey: "nav.settings", icon: SettingsIcon },
  { href: "/debug/digests", labelKey: "nav.debug", icon: Bug },
];

const moreStandaloneLinks = secondaryLinks.filter((link) => ["/settings", "/debug/digests"].includes(link.href));

const moreSubmenuGroups = [
  {
    labelKey: "nav.group.insights",
    items: secondaryLinks.filter((link) => ["/clusters", "/pulse", "/goals", "/favorites"].includes(link.href)),
  },
  {
    labelKey: "nav.group.library",
    items: secondaryLinks.filter((link) =>
      ["/sources", "/digests", "/audio-briefings", "/ai-navigator-briefs", "/playback-history"].includes(link.href),
    ),
  },
  {
    labelKey: "nav.group.llm",
    items: secondaryLinks.filter((link) =>
      ["/llm-usage", "/llm-analysis", "/provider-model-snapshots", "/poe-models", "/openrouter-models", "/aivis-models"].includes(link.href),
    ),
  },
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
  const [openMoreSubmenu, setOpenMoreSubmenu] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [openRouterSyncRun, setOpenRouterSyncRun] = useState<OpenRouterSyncRun | null>(null);
  const [watchOpenRouterSync, setWatchOpenRouterSync] = useState(false);
  const moreMenuRef = useRef<HTMLDivElement | null>(null);

  const isActive = (href: string) => pathname === href || pathname?.startsWith(`${href}/`);
  const isLinkActive = (href: string, activeHref?: string) => isActive(activeHref ?? href);
  const isMoreActive = secondaryLinks.some((v) => isActive(v.href));
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

  useEffect(() => {
    let cancelled = false;
    let interval: number | null = null;

    const loadStatus = async () => {
      try {
        const next = await api.getOpenRouterSyncStatus();
        if (!cancelled) {
          setOpenRouterSyncRun(next.run);
        }
      } catch {
        if (!cancelled) {
          setOpenRouterSyncRun(null);
        }
      }
    };

    const onStarted = () => {
      setWatchOpenRouterSync(true);
      loadStatus();
    };

    loadStatus();
    if (watchOpenRouterSync) {
      interval = window.setInterval(loadStatus, 3000);
    }
    window.addEventListener("openrouter-sync-started", onStarted);
    return () => {
      cancelled = true;
      if (interval != null) window.clearInterval(interval);
      window.removeEventListener("openrouter-sync-started", onStarted);
    };
  }, [watchOpenRouterSync]);

  useEffect(() => {
    if (!watchOpenRouterSync) return;
    if (openRouterSyncRun?.status === "running") return;
    setWatchOpenRouterSync(false);
  }, [openRouterSyncRun, watchOpenRouterSync]);

  const handleForceRefresh = () => {
    if (refreshing) return;
    setRefreshing(true);
    enableForceFreshReload();
    window.location.reload();
  };

  return (
    <>
      <header className="sticky top-0 z-20 border-b border-[color:rgba(190,179,160,0.55)] bg-[color:rgba(252,251,248,0.84)] shadow-[0_8px_28px_rgba(35,24,12,0.06)] backdrop-blur">
        <div className="mx-auto max-w-[1360px] px-4 py-3 md:px-6 md:py-[18px]">
          <div className="flex min-h-11 items-center gap-3 lg:hidden">
            <Link href="/" className="flex items-center gap-2 rounded-full press focus-ring">
              <Image src="/logo-transparent.png" alt="Sifto" width={32} height={32} priority />
              <span className="font-serif text-[20px] tracking-[-0.03em] text-[var(--color-editorial-ink)]">Sifto</span>
            </Link>

            <div className="ml-auto flex items-center gap-2">
              <PWAInstallButton />
              <button
                type="button"
                onClick={handleForceRefresh}
                className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
                aria-label={t("nav.refreshLatest")}
                title={t("nav.refreshLatest")}
              >
                <RefreshCw className={`size-4 ${refreshing ? "animate-spin" : ""}`} aria-hidden="true" />
              </button>
              <label className="sr-only">{t("nav.language")}</label>
              <select
                value={locale}
                onChange={(e) => setLocale(e.target.value as "ja" | "en")}
                className="h-9 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 text-xs text-[var(--color-editorial-ink-soft)] focus-ring"
                aria-label={t("nav.language")}
              >
                <option value="ja">{t("nav.locale.ja")}</option>
                <option value="en">{t("nav.locale.en")}</option>
              </select>
              <button
                type="button"
                onClick={() => setMenuOpen((v) => !v)}
                className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] lg:hidden press focus-ring"
                aria-expanded={menuOpen}
                aria-label={menuOpen ? t("nav.menu.close") : t("nav.menu.open")}
              >
                {menuOpen ? (
                  <X className="size-4" aria-hidden="true" />
                ) : (
                  <Menu className="size-4" aria-hidden="true" />
                )}
              </button>
            </div>
          </div>

          <div className="hidden items-center gap-5 lg:flex">
            <Link href="/" className="flex min-w-[220px] items-center gap-3 rounded-full press focus-ring">
              <Image src="/logo-transparent.png" alt="Sifto" width={38} height={38} priority />
              <div className="flex flex-col gap-0.5">
                <span className="font-serif text-[22px] leading-none tracking-[0.03em] text-[var(--color-editorial-ink)]">Sifto</span>
                <span className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  Intelligence Desk
                </span>
              </div>
            </Link>

            <nav className="flex min-w-0 flex-1 flex-wrap gap-2">
              {primaryLinks.map(({ href, activeHref, labelKey }) => {
                const active = isLinkActive(href, activeHref);
                return (
                  <Link
                    key={href}
                    href={href}
                    onClick={() => setMoreOpen(false)}
                    className={`group inline-flex items-center rounded-full border px-[14px] py-[10px] text-[13px] leading-none transition-colors duration-150 press focus-ring ${
                      active
                        ? "border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]"
                        : "border-transparent text-[var(--color-editorial-ink-soft)] hover:border-[var(--color-editorial-line)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                    }`}
                  >
                    <span>{t(labelKey)}</span>
                  </Link>
                );
              })}
              <div className="relative" ref={moreMenuRef}>
                <button
                  type="button"
                  onClick={() => setMoreOpen((v) => !v)}
                  className={`group inline-flex items-center rounded-full border px-[14px] py-[10px] text-[13px] leading-none transition-colors duration-150 press focus-ring ${
                    secondaryLinks.some((v) => isActive(v.href))
                      ? "border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]"
                      : "border-transparent text-[var(--color-editorial-ink-soft)] hover:border-[var(--color-editorial-line)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                  }`}
                  aria-expanded={moreOpen}
                >
                  <span>{t("nav.more")}</span>
                </button>
                {moreOpen && (
                  <div className="absolute left-0 top-11 z-30 w-64 rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3 shadow-[var(--shadow-dropdown)] motion-safe:animate-scale-in">
                    {moreSubmenuGroups.map((group) => {
                      const active = group.items.some((item) => isActive(item.href));
                      const expanded = openMoreSubmenu === group.labelKey;
                      return (
                        <div
                          key={group.labelKey}
                          className="relative"
                          onMouseEnter={() => setOpenMoreSubmenu(group.labelKey)}
                          onMouseLeave={() => setOpenMoreSubmenu((current) => (current === group.labelKey ? null : current))}
                        >
                          <button
                            type="button"
                            onFocus={() => setOpenMoreSubmenu(group.labelKey)}
                            className={`flex w-full items-center justify-between rounded-[14px] px-4 py-3 text-left text-[14px] transition-colors duration-150 press focus-ring ${
                              active || expanded
                                ? "bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]"
                                : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                            }`}
                          >
                            <span>{t(group.labelKey)}</span>
                            <ChevronRight className="size-4 shrink-0" aria-hidden="true" />
                          </button>
                          {expanded ? (
                            <div className="absolute left-full top-0 z-40 w-64 rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3 shadow-[var(--shadow-dropdown)]">
                              <div className="px-2 pb-1 pt-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                {t(group.labelKey)}
                              </div>
                              {group.items.map(({ href, labelKey, icon: Icon }) => {
                                const childActive = isActive(href);
                                return (
                                  <Link
                                    key={href}
                                    href={href}
                                    onClick={() => setMoreOpen(false)}
                                    className={`flex items-center gap-2 rounded-[14px] px-4 py-3 text-[14px] transition-colors duration-150 press focus-ring ${
                                      childActive
                                        ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                                        : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                                    }`}
                                  >
                                    <NavIcon icon={Icon} active={childActive} />
                                    <span>{t(labelKey)}</span>
                                  </Link>
                                );
                              })}
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                    <div className="my-2 h-px bg-[var(--color-editorial-line)]" />
                    {moreStandaloneLinks.map(({ href, labelKey, icon: Icon }) => {
                      const active = isActive(href);
                      return (
                        <Link
                          key={href}
                          href={href}
                          onClick={() => setMoreOpen(false)}
                          onMouseEnter={() => setOpenMoreSubmenu(null)}
                          onFocus={() => setOpenMoreSubmenu(null)}
                          className={`flex items-center gap-2 rounded-[14px] px-4 py-3 text-[14px] transition-colors duration-150 press focus-ring ${
                            active
                              ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                              : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                          }`}
                        >
                          <NavIcon icon={Icon} active={active} />
                          <span>{t(labelKey)}</span>
                        </Link>
                      );
                    })}
                    {hasSignedInUser && (
                      <div className="mt-2 border-t border-[var(--color-editorial-line)] px-2 pt-2">
                        <div className="truncate text-xs text-[var(--color-editorial-ink-faint)]">
                          {displayName}
                        </div>
                        <button
                          onClick={onSignOut}
                          className="mt-2 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-left text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
                        >
                          {t("nav.signOut")}
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </nav>
            <div className="ml-auto flex items-center gap-2">
              <PWAInstallButton />
              <button
                type="button"
                onClick={handleForceRefresh}
                className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] press focus-ring"
                aria-label={t("nav.refreshLatest")}
                title={t("nav.refreshLatest")}
              >
                <RefreshCw className={`size-4 ${refreshing ? "animate-spin" : ""}`} aria-hidden="true" />
              </button>
              <label className="sr-only">{t("nav.language")}</label>
              <select
                value={locale}
                onChange={(e) => setLocale(e.target.value as "ja" | "en")}
                className="h-9 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 text-xs text-[var(--color-editorial-ink-soft)] focus-ring"
                aria-label={t("nav.language")}
              >
                <option value="ja">{t("nav.locale.ja")}</option>
                <option value="en">{t("nav.locale.en")}</option>
              </select>
            </div>
          </div>

          {menuOpen && (
            <div className="mt-2 rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3 shadow-[var(--shadow-dropdown)] lg:hidden motion-safe:animate-scale-in">
              <nav className="grid gap-1">
                {primaryLinks.map(({ href, activeHref, labelKey, icon: Icon }) => {
                  const active = isLinkActive(href, activeHref);
                  return (
                    <Link
                      key={href}
                      href={href}
                      onClick={() => setMenuOpen(false)}
                      className={`inline-flex items-center gap-2 rounded-[14px] px-4 py-3 text-sm font-medium transition-colors duration-150 press focus-ring ${
                        active
                          ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                          : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                      }`}
                    >
                      <NavIcon icon={Icon} active={active} />
                      <span>{t(labelKey)}</span>
                    </Link>
                  );
                })}
                <div className="my-1 h-px bg-[var(--color-editorial-line)]" />
                {moreSubmenuGroups.map((group) => {
                  const expanded = openMoreSubmenu === group.labelKey;
                  const active = group.items.some((item) => isActive(item.href));
                  return (
                    <div key={group.labelKey}>
                      <button
                        type="button"
                        onClick={() => setOpenMoreSubmenu((current) => (current === group.labelKey ? null : group.labelKey))}
                        className={`flex w-full items-center justify-between rounded-[14px] px-4 py-3 text-left text-sm font-medium transition-colors duration-150 press focus-ring ${
                          active || expanded
                            ? "bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]"
                            : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                        }`}
                      >
                        <span>{t(group.labelKey)}</span>
                        <ChevronDown className={`size-4 transition-transform ${expanded ? "rotate-180" : ""}`} aria-hidden="true" />
                      </button>
                      {expanded ? group.items.map(({ href, labelKey, icon: Icon }) => {
                        const childActive = isActive(href);
                        return (
                          <Link
                            key={href}
                            href={href}
                            onClick={() => setMenuOpen(false)}
                            className={`ml-3 inline-flex items-center gap-2 rounded-[14px] px-4 py-3 text-sm font-medium transition-colors duration-150 press focus-ring ${
                              childActive
                                ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                                : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                            }`}
                          >
                            <NavIcon icon={Icon} active={childActive} />
                            <span>{t(labelKey)}</span>
                          </Link>
                        );
                      }) : null}
                    </div>
                  );
                })}
                <div className="my-1 h-px bg-[var(--color-editorial-line)]" />
                {moreStandaloneLinks.map(({ href, labelKey, icon: Icon }) => {
                  const active = isActive(href);
                  return (
                    <Link
                      key={href}
                      href={href}
                      onClick={() => setMenuOpen(false)}
                      className={`inline-flex items-center gap-2 rounded-[14px] px-4 py-3 text-sm font-medium transition-colors duration-150 press focus-ring ${
                        active
                          ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                          : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                      }`}
                    >
                      <NavIcon icon={Icon} active={active} />
                      <span>{t(labelKey)}</span>
                    </Link>
                  );
                })}
              </nav>
              {hasSignedInUser && (
                <div className="mt-2 border-t border-[var(--color-editorial-line)] px-2 pt-2">
                  <div className="truncate text-xs text-[var(--color-editorial-ink-faint)]">
                    {displayName}
                  </div>
                  <button
                    onClick={onSignOut}
                    className="mt-2 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-left text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
                  >
                    {t("nav.signOut")}
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
        {openRouterSyncRun ? (
          <div className="border-t border-[#e1cb9e] bg-[var(--warning-soft)]/95">
            <div className="mx-auto flex max-w-[1360px] items-center justify-between gap-3 px-4 py-2 text-xs text-[var(--warning)] md:px-6">
              <Link href="/openrouter-models" className="inline-flex min-w-0 items-center gap-2 rounded hover:underline">
                <RefreshCw className="size-3.5 animate-spin" aria-hidden="true" />
                <span className="truncate">
                  {openRouterSyncRun.translation_target_count > 0
                    ? t("openrouterModels.progressGlobal")
                        .replace("{{completed}}", String(openRouterSyncRun.translation_completed_count))
                        .replace("{{total}}", String(openRouterSyncRun.translation_target_count))
                    : t("openrouterModels.progressPreparing")}
                </span>
              </Link>
            </div>
          </div>
        ) : null}
      </header>
      <nav
        className="fixed inset-x-0 bottom-0 z-30 border-t border-[var(--color-editorial-line)] bg-[color-mix(in_srgb,var(--color-editorial-panel-strong)_92%,transparent)] px-2 pb-[calc(env(safe-area-inset-bottom)+0.4rem)] pt-2 backdrop-blur lg:hidden"
      >
        <div className="mx-auto grid max-w-md grid-cols-5 gap-1">
          {primaryLinks.map(({ href, activeHref, labelKey, icon: Icon }) => {
            const active = isLinkActive(href, activeHref);
            return (
              <Link
                key={href}
                href={href}
                aria-label={t(labelKey)}
                className={`relative flex min-h-12 flex-col items-center justify-center rounded-xl px-1 py-1 text-[11px] font-medium transition-colors duration-150 press focus-ring ${
                  active
                    ? "bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]"
                    : "text-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
                }`}
              >
                <NavIcon icon={Icon} active={active} mobile />
                <span className="sr-only">{t(labelKey)}</span>
                {active && (
                  <span className="absolute bottom-1.5 left-1/2 h-1 w-6 -translate-x-1/2 rounded-full bg-[var(--color-editorial-accent)]" />
                )}
              </Link>
            );
          })}
          <Link
            href="/settings"
            aria-label={t("nav.more")}
            className={`relative flex min-h-12 flex-col items-center justify-center rounded-xl px-1 py-1 text-[11px] font-medium transition-colors duration-150 press focus-ring ${
              isMoreActive
                ? "bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]"
                : "text-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel)] hover:text-[var(--color-editorial-ink)]"
            }`}
          >
            <NavIcon icon={SettingsIcon} active={isMoreActive} mobile />
            <span className="sr-only">{t("nav.more")}</span>
            {isMoreActive && (
              <span className="absolute bottom-1.5 left-1/2 h-1 w-6 -translate-x-1/2 rounded-full bg-[var(--color-editorial-accent)]" />
            )}
          </Link>
        </div>
      </nav>
    </>
  );
}

function NavIcon({ icon: Icon, active, mobile = false }: { icon: LucideIcon; active?: boolean; mobile?: boolean }) {
  return (
    <Icon
      className={`${mobile ? "size-[18px]" : "size-4"} shrink-0 transition-transform duration-150 group-hover:scale-110 ${active ? "scale-105" : ""}`}
      strokeWidth={2}
      aria-hidden="true"
    />
  );
}
