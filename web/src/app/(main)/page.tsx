"use client";

import Link from "next/link";
import { type ReactNode, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Bell, Flame, ListTree, Sparkles, Target } from "lucide-react";
import { api, BriefingCluster, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

const EMPTY_ITEMS: Item[] = [];
const EMPTY_CLUSTERS: BriefingCluster[] = [];

export default function BriefingPage() {
  const { t, locale } = useI18n();
  const briefingQuery = useQuery({
    queryKey: ["briefing-today", 18] as const,
    queryFn: () => api.getBriefingToday({ size: 18 }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const loading = !briefingQuery.data && (briefingQuery.isLoading || briefingQuery.isFetching);
  const error = briefingQuery.error ? String(briefingQuery.error) : null;
  const data = briefingQuery.data;
  const highlights = data?.highlight_items ?? EMPTY_ITEMS;
  const clusters = data?.clusters ?? EMPTY_CLUSTERS;

  const clusterRows = useMemo(
    () =>
      clusters
        .map((cluster) => ({
          ...cluster,
          topItems: (cluster.items ?? EMPTY_ITEMS).slice(0, 4),
        }))
        .filter((cluster) => cluster.topItems.length > 0),
    [clusters]
  );

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm md:p-6">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 className="inline-flex items-center gap-2 text-2xl font-bold tracking-tight text-zinc-900">
              <Sparkles className="size-6 text-blue-600" aria-hidden="true" />
              <span>{t("briefing.title")}</span>
            </h1>
            <p className="mt-1 text-sm text-zinc-600">
              {(data?.greeting ?? t("briefing.greetingFallback")) + " · " + (data?.date ?? "")}
            </p>
          </div>
          <button
            type="button"
            onClick={() => void briefingQuery.refetch()}
            className="rounded border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50"
          >
            {t("common.refresh")}
          </button>
        </div>

        <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <Kpi
            icon={<Target className="size-4 text-zinc-500" aria-hidden="true" />}
            label={t("briefing.kpi.today")}
            value={String(data?.stats.today_highlight_count ?? 0)}
          />
          <Kpi
            icon={<Bell className="size-4 text-zinc-500" aria-hidden="true" />}
            label={t("briefing.kpi.unread")}
            value={String(data?.stats.total_unread ?? 0)}
          />
          <Kpi
            icon={<ListTree className="size-4 text-zinc-500" aria-hidden="true" />}
            label={t("briefing.kpi.yesterdayRead")}
            value={String(data?.stats.yesterday_read ?? 0)}
          />
          <Kpi
            icon={<Flame className="size-4 text-zinc-500" aria-hidden="true" />}
            label={t("briefing.kpi.streak")}
            value={`${data?.stats.streak_days ?? 0}${t("briefing.kpi.streakUnit")}`}
          />
        </div>
        {data?.stats.streak_at_risk ? (
          <div className="mt-4 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-900">
            <p className="font-medium">{t("briefing.streakRisk.title")}</p>
            <p className="mt-0.5 text-xs text-rose-700">
              {t("briefing.streakRisk.prefix")}
              {data?.stats.streak_remaining ?? 0}
              {t("briefing.streakRisk.unit")}
              {t("briefing.streakRisk.suffix")}
            </p>
          </div>
        ) : null}
        <div className="mt-4">
          <Link
            href="/triage"
            className="inline-flex items-center rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800"
          >
            {t("briefing.openTriage")}
          </Link>
        </div>
      </section>

      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm md:p-6">
        <div className="mb-3 flex items-center justify-between gap-2">
          <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.highlights")}</h2>
          <Link href="/items?feed=recommended" className="text-xs text-zinc-500 hover:text-zinc-900">
            {t("briefing.openRecommended")}
          </Link>
        </div>
        {highlights.length === 0 ? (
          <p className="text-sm text-zinc-500">{t("briefing.emptyHighlights")}</p>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {highlights.slice(0, 6).map((item, idx) => (
              <article key={item.id} className="min-w-0 rounded-xl border border-zinc-200 p-4">
                <p className="text-xs font-semibold text-blue-600">{`PICK ${idx + 1}`}</p>
                <Link
                  href={`/items/${item.id}?from=${encodeURIComponent("/items?feed=recommended")}`}
                  className="mt-2 line-clamp-3 block break-words [overflow-wrap:anywhere] text-base font-semibold text-zinc-900 hover:underline"
                >
                  {item.translated_title || item.title || item.url}
                </Link>
                <p className="mt-2 truncate text-xs text-zinc-500" title={item.url}>
                  {item.url}
                </p>
                <div className="mt-3 flex items-center justify-between text-xs text-zinc-500">
                  <span>{item.is_read ? t("items.read.read") : t("items.read.unread")}</span>
                  <span>{fmtDate(item.published_at || item.created_at, locale)}</span>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm md:p-6">
        <div className="mb-3 flex items-center justify-between gap-2">
          <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.clusters")}</h2>
          <Link href="/items?feed=recommended" className="text-xs text-zinc-500 hover:text-zinc-900">
            {t("briefing.openRecommended")}
          </Link>
        </div>
        {clusterRows.length === 0 ? (
          <p className="text-sm text-zinc-500">{t("briefing.emptyClusters")}</p>
        ) : (
          <div className="space-y-3">
            {clusterRows.map((cluster) => (
              <article key={cluster.id} className="rounded-xl border border-zinc-200 p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-sm font-semibold text-zinc-900">{cluster.label || t("briefing.clusterFallback")}</h3>
                  <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-600">{`${cluster.topItems.length}${t("common.rows")}`}</span>
                </div>
                {cluster.summary ? (
                  <p className="mt-1 line-clamp-2 text-xs text-zinc-500">{cluster.summary}</p>
                ) : null}
                <ul className="mt-2 space-y-1.5">
                  {cluster.topItems.map((item) => (
                    <li key={item.id}>
                      <Link
                        href={`/items/${item.id}?from=${encodeURIComponent("/items?feed=recommended")}`}
                        className="block rounded px-2 py-1.5 text-sm text-zinc-700 break-words [overflow-wrap:anywhere] hover:bg-zinc-50"
                        title={item.translated_title || item.title || item.url}
                      >
                        {item.translated_title || item.title || item.url}
                      </Link>
                    </li>
                  ))}
                </ul>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function Kpi({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-xl border border-zinc-200 bg-zinc-50/60 p-3">
      <div className="flex items-center gap-2 text-xs text-zinc-600">
        {icon}
        <span>{label}</span>
      </div>
      <div className="mt-1 text-2xl font-semibold tracking-tight text-zinc-900">{value}</div>
    </div>
  );
}

function fmtDate(value: string | null | undefined, locale: "ja" | "en") {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US");
}
