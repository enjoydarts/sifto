"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { type ReactNode, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Bell, BookOpen, Flame, ListTree, Sparkles, Target } from "lucide-react";
import { api, BriefingCluster, Item, ProviderModelChangeEvent } from "@/lib/api";
import { InlineReader } from "@/components/inline-reader";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { EmptyState } from "@/components/empty-state";
import { SkeletonCard, SkeletonKpi } from "@/components/skeleton";

const EMPTY_ITEMS: Item[] = [];
const EMPTY_CLUSTERS: BriefingCluster[] = [];
const EMPTY_MODEL_UPDATES: ProviderModelChangeEvent[] = [];

export default function BriefingPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const briefingQuery = useQuery({
    queryKey: ["briefing-today", 18] as const,
    queryFn: () => api.getBriefingToday({ size: 18 }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const modelUpdatesQuery = useQuery({
    queryKey: ["provider-model-updates", 7] as const,
    queryFn: () => api.getProviderModelUpdates({ days: 7, limit: 6 }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const loading = !briefingQuery.data && (briefingQuery.isLoading || briefingQuery.isFetching);
  const error = briefingQuery.error ? String(briefingQuery.error) : null;
  const data = briefingQuery.data;
  const modelUpdates = modelUpdatesQuery.data ?? EMPTY_MODEL_UPDATES;
  const highlights = data?.highlight_items ?? EMPTY_ITEMS;
  const clusters = data?.clusters ?? EMPTY_CLUSTERS;
  const unreadHighlights = useMemo(() => highlights.filter((item) => !item.is_read), [highlights]);
  const nowReading = unreadHighlights[0] ?? highlights[0] ?? null;
  const nextReads = useMemo(() => {
    const src = unreadHighlights.length > 0 ? unreadHighlights : highlights;
    return src.slice(1, 7);
  }, [highlights, unreadHighlights]);

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
  const greetingLabel = (() => {
    const key = data?.greeting_key;
    if (key === "morning") return t("briefing.greeting.morning");
    if (key === "afternoon") return t("briefing.greeting.afternoon");
    if (key === "evening") return t("briefing.greeting.evening");
    if (data?.greeting) return data.greeting;
    return t("briefing.greetingFallback");
  })();
  const briefingQueueItemIds = useMemo(() => {
    const ids: string[] = [];
    const seen = new Set<string>();
    for (const item of highlights) {
      if (seen.has(item.id)) continue;
      seen.add(item.id);
      ids.push(item.id);
    }
    for (const cluster of clusterRows) {
      for (const item of cluster.topItems) {
        if (seen.has(item.id)) continue;
        seen.add(item.id);
        ids.push(item.id);
      }
    }
    return ids;
  }, [clusterRows, highlights]);
  const nowQueueIds = useMemo(() => {
    const ids: string[] = [];
    const seen = new Set<string>();
    if (nowReading) {
      ids.push(nowReading.id);
      seen.add(nowReading.id);
    }
    for (const item of nextReads) {
      if (seen.has(item.id)) continue;
      ids.push(item.id);
      seen.add(item.id);
    }
    return ids.length > 0 ? ids : briefingQueueItemIds;
  }, [briefingQueueItemIds, nextReads, nowReading]);

  return (
    <PageTransition>
      <div className="space-y-6">
        <section className="rounded-2xl border border-amber-200 bg-amber-50/70 p-4 shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
              <Bell className="size-4 shrink-0 text-amber-700" aria-hidden="true" />
              <div className="min-w-0">
                <h2 className="text-sm font-semibold text-amber-900">{t("briefing.providerModelUpdates")}</h2>
                {modelUpdates.length === 0 ? (
                  <p className="text-sm text-amber-800">{t("briefing.providerModelUpdatesEmpty")}</p>
                ) : (
                  <p className="text-sm text-amber-800">
                    {modelUpdates.slice(0, 3).map((event) => `${event.provider} ${event.change_type === "added" ? "+" : "-"} ${event.model_id}`).join(" / ")}
                  </p>
                )}
              </div>
            </div>
            <Link
              href="/settings"
              className="inline-flex items-center rounded-lg border border-amber-300 bg-white px-3 py-2 text-sm font-medium text-amber-900 hover:bg-amber-100 press focus-ring"
            >
              {t("briefing.providerModelUpdatesOpen")}
            </Link>
          </div>
        </section>

        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm md:p-6">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h1 className="inline-flex items-center gap-2 text-2xl font-bold tracking-tight text-zinc-900">
                <Sparkles className="size-6 text-blue-600" aria-hidden="true" />
                <span>{t("briefing.title")}</span>
              </h1>
              <p className="mt-1 text-sm text-zinc-600">
                {`${greetingLabel} · ${data?.date ?? ""}`}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => {
                  void api
                    .getBriefingToday({ size: 18, cache_bust: true })
                    .then((next) => {
                      queryClient.setQueryData(["briefing-today", 18], next);
                    })
                    .catch(() => {
                      // keep current snapshot on refresh failure
                    });
                }}
                className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50 press focus-ring"
              >
                {t("common.refresh")}
              </button>
              <Link
                href="/triage?mode=all"
                className="inline-flex items-center rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 press focus-ring"
              >
                {t("briefing.openTriage")}
              </Link>
            </div>
          </div>

          {error && <p className="mt-4 text-sm text-red-500">{error}</p>}

          <div className="mt-5">
            <p className="text-xs font-medium uppercase tracking-[0.12em] text-zinc-500">
              {t("briefing.nowReadingLabel", "NOW READING")}
            </p>
          </div>
          {loading ? (
            <div className="mt-3">
              <SkeletonCard />
            </div>
          ) : !nowReading ? (
            <EmptyState
              icon={Sparkles}
              title={t("emptyState.briefing.title")}
              description={t("emptyState.briefing.desc")}
            />
          ) : (
            <article className="mt-3 rounded-2xl border border-zinc-200 bg-zinc-50/60 p-4 md:p-5">
              <button
                type="button"
                onClick={() => setInlineItemId(nowReading.id)}
                className="block w-full text-left"
              >
                <h2 className="line-clamp-3 break-words [overflow-wrap:anywhere] text-xl font-semibold leading-tight text-zinc-900 hover:underline">
                  {nowReading.translated_title || nowReading.title || nowReading.url}
                </h2>
              </button>
              <p className="mt-2 truncate text-xs text-zinc-500" title={nowReading.url}>
                {nowReading.url}
              </p>
              <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                <span className="rounded-full border border-zinc-300 bg-white px-2 py-0.5">
                  {nowReading.is_read ? t("items.read.read") : t("items.read.unread")}
                </span>
                <span>{fmtDate(nowReading.published_at || nowReading.created_at, locale)}</span>
              </div>
              <div className="mt-4 flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => setInlineItemId(nowReading.id)}
                  className="inline-flex items-center gap-1 rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 press focus-ring"
                >
                  <BookOpen className="size-4" aria-hidden="true" />
                  {t("briefing.readNow", "今すぐ読む")}
                </button>
                <Link
                  href={`/items/${nowReading.id}?from=${encodeURIComponent("/items?feed=recommended")}`}
                  className="inline-flex items-center gap-1 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 press focus-ring"
                >
                  {t("items.action.openDetail")}
                  <ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </div>
            </article>
          )}

          <div className="mt-5 flex items-center justify-between gap-2">
            <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.nextReads", "Next Up")}</h2>
            <Link href="/items?feed=recommended" className="text-xs text-zinc-500 hover:text-zinc-900 transition-colors">
              {t("briefing.openRecommended")}
            </Link>
          </div>
          {loading ? (
            <div className="mt-3 grid gap-3 md:grid-cols-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <SkeletonCard key={i} />
              ))}
            </div>
          ) : nextReads.length === 0 ? (
            <p className="mt-2 text-sm text-zinc-500">{t("briefing.emptyHighlights", "次に読む記事はありません。")}</p>
          ) : (
            <ul className="mt-3 space-y-2">
              {nextReads.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    onClick={() => setInlineItemId(item.id)}
                    className="block w-full rounded-xl border border-zinc-200 bg-white px-4 py-3 text-left hover:border-zinc-300 hover:bg-zinc-50"
                  >
                    <div className="line-clamp-2 break-words [overflow-wrap:anywhere] text-sm font-medium text-zinc-900">
                      {item.translated_title || item.title || item.url}
                    </div>
                    <div className="mt-1 flex items-center gap-2 text-xs text-zinc-500">
                      <span>{item.is_read ? t("items.read.read") : t("items.read.unread")}</span>
                      <span>{fmtDate(item.published_at || item.created_at, locale)}</span>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm md:p-6">
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {loading ? (
              <>
                <SkeletonKpi />
                <SkeletonKpi />
                <SkeletonKpi />
                <SkeletonKpi />
              </>
            ) : (
              <>
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
              </>
            )}
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
        </section>

        {inlineItemId && (
          <InlineReader
            open={!!inlineItemId}
            itemId={inlineItemId}
            locale={locale}
            queueItemIds={nowQueueIds}
            onClose={() => setInlineItemId(null)}
            onOpenDetail={(itemId) => {
              router.push(`/items/${itemId}?from=${encodeURIComponent("/items?feed=recommended")}`);
            }}
            onOpenItem={(itemId) => setInlineItemId(itemId)}
          />
        )}
      </div>
    </PageTransition>
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
