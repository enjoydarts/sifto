"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Layers3 } from "lucide-react";
import { api, BriefingCluster, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { useToast } from "@/components/toast-provider";

const EMPTY_CLUSTERS: BriefingCluster[] = [];

export default function ClustersPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [savingClusterId, setSavingClusterId] = useState<string | null>(null);

  const briefingQuery = useQuery({
    queryKey: ["briefing-clusters", 24] as const,
    queryFn: () => api.getBriefingToday({ size: 24 }),
    staleTime: 15_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
    placeholderData: (prev) => prev,
  });

  const clusters = briefingQuery.data?.clusters ?? EMPTY_CLUSTERS;
  const rows = useMemo(() => clusters.filter((cluster) => (cluster.items ?? []).length > 0), [clusters]);

  const saveClusterForLater = async (cluster: BriefingCluster) => {
    const itemIds = Array.from(new Set((cluster.items ?? []).map((item) => item.id).filter(Boolean)));
    if (itemIds.length === 0) {
      showToast(t("briefing.clusterLaterEmpty"), "info");
      return;
    }
    setSavingClusterId(cluster.id);
    try {
      const res = await api.markItemsLaterBulk({ item_ids: itemIds });
      if (res.updated_count <= 0) {
        showToast(t("briefing.clusterLaterEmpty"), "info");
        return;
      }
      showToast(`${res.updated_count}${locale === "ja" ? "" : " "}${t("briefing.clusterLaterDone")}`, "success");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
        queryClient.invalidateQueries({ queryKey: ["briefing-clusters"] }),
        queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
      ]);
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setSavingClusterId(null);
    }
  };

  const markClusterRead = async (cluster: BriefingCluster) => {
    const itemIds = Array.from(
      new Set((cluster.items ?? []).filter((item) => !item.is_read).map((item) => item.id).filter(Boolean))
    );
    if (itemIds.length === 0) {
      showToast(t("briefing.clusterReadEmpty"), "info");
      return;
    }
    setSavingClusterId(`read:${cluster.id}`);
    try {
      const res = await api.markItemsReadByIDs(itemIds);
      if (res.updated_count <= 0) {
        showToast(t("briefing.clusterReadEmpty"), "info");
        return;
      }
      showToast(`${res.updated_count}${locale === "ja" ? "" : " "}${t("briefing.clusterReadDone")}`, "success");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
        queryClient.invalidateQueries({ queryKey: ["briefing-clusters"] }),
        queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
      ]);
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setSavingClusterId(null);
    }
  };

  return (
    <PageTransition>
      <div className="space-y-6">
        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm md:p-6">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h1 className="inline-flex items-center gap-2 text-2xl font-bold tracking-tight text-zinc-900">
                <Layers3 className="size-6 text-blue-600" aria-hidden="true" />
                <span>{t("clusters.title")}</span>
              </h1>
              <p className="mt-1 text-sm text-zinc-600">{t("clusters.subtitle")}</p>
            </div>
            <Link
              href="/"
              className="inline-flex items-center rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50 press focus-ring"
            >
              {t("clusters.backBriefing")}
            </Link>
          </div>
        </section>

        {briefingQuery.isLoading && !briefingQuery.data ? (
          <section className="rounded-2xl border border-zinc-200 bg-white p-5 text-sm text-zinc-500 shadow-sm md:p-6">
            {t("common.loading")}
          </section>
        ) : rows.length === 0 ? (
          <section className="rounded-2xl border border-zinc-200 bg-white p-5 text-sm text-zinc-500 shadow-sm md:p-6">
            {t("clusters.empty")}
          </section>
        ) : (
          <div className="grid gap-5 xl:grid-cols-2">
            {rows.map((cluster) => (
              <section
                key={cluster.id}
                className="overflow-hidden rounded-[28px] border border-zinc-200 bg-white shadow-sm transition-shadow hover:shadow-md"
              >
                <div className="border-b border-zinc-100 bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.08),_transparent_30%),linear-gradient(180deg,_#ffffff_0%,_#f8fafc_100%)] p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
                        {t("clusters.eyebrow")}
                      </div>
                      <h2 className="mt-2 truncate text-lg font-semibold text-zinc-900">
                        {cluster.label || t("briefing.clusterFallback")}
                      </h2>
                      {cluster.summary ? <p className="mt-2 line-clamp-3 text-sm leading-6 text-zinc-600">{cluster.summary}</p> : null}
                    </div>
                    <span className="shrink-0 rounded-full border border-zinc-300 bg-white px-2.5 py-1 text-xs text-zinc-600 shadow-sm">
                      {cluster.items.length}
                    </span>
                  </div>

                  {cluster.items[0] ? (
                    <Link
                      href={`/items/${cluster.items[0].id}?from=${encodeURIComponent("/clusters")}`}
                      className="mt-4 block overflow-hidden rounded-[22px] border border-zinc-200 bg-white hover:border-zinc-300"
                    >
                      <div className="grid gap-0 md:grid-cols-[1.15fr_0.85fr]">
                        <ThumbnailArtwork item={cluster.items[0]} className="h-52 w-full md:h-full" />
                        <div className="flex flex-col justify-between p-4">
                          <div>
                            <div className="text-xs font-medium uppercase tracking-[0.12em] text-zinc-500">
                              {t("clusters.lead")}
                            </div>
                            <div className="mt-2 line-clamp-3 text-base font-semibold leading-6 text-zinc-900">
                              {cluster.items[0].translated_title || cluster.items[0].title || cluster.items[0].url}
                            </div>
                          </div>
                          <div className="mt-4 flex items-center justify-between gap-2 text-xs text-zinc-500">
                            <span>{fmtDate(cluster.items[0], locale)}</span>
                            {cluster.items[0].summary_score != null ? (
                              <span className="rounded-full bg-zinc-100 px-2 py-1 text-zinc-600">
                                {t("clusters.score")} {Math.round(cluster.items[0].summary_score * 100)}
                              </span>
                            ) : null}
                          </div>
                        </div>
                      </div>
                    </Link>
                  ) : null}
                </div>

                <div className="p-5">
                  <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => void saveClusterForLater(cluster)}
                    disabled={savingClusterId === cluster.id || savingClusterId === `read:${cluster.id}`}
                    className="inline-flex items-center rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50 press focus-ring disabled:cursor-wait disabled:opacity-60"
                  >
                    {savingClusterId === cluster.id ? t("briefing.clusterLaterSaving") : t("briefing.clusterLater")}
                  </button>
                  <button
                    type="button"
                    onClick={() => void markClusterRead(cluster)}
                    disabled={savingClusterId === cluster.id || savingClusterId === `read:${cluster.id}`}
                    className="inline-flex items-center rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50 press focus-ring disabled:cursor-wait disabled:opacity-60"
                  >
                    {savingClusterId === `read:${cluster.id}` ? t("briefing.clusterReadSaving") : t("briefing.clusterRead")}
                  </button>
                  </div>

                  <ul className="mt-4 space-y-3">
                  {cluster.items.slice(1, 7).map((item, idx) => (
                    <li key={item.id}>
                      <Link
                        href={`/items/${item.id}?from=${encodeURIComponent("/clusters")}`}
                        className="grid grid-cols-[92px_1fr] gap-3 rounded-2xl border border-zinc-200 bg-zinc-50/70 p-3 hover:border-zinc-300 hover:bg-zinc-100"
                      >
                        <ThumbnailArtwork item={item} className="h-20 w-full rounded-xl" />
                        <div className="min-w-0">
                          <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-[0.12em] text-zinc-500">
                            <span>{idx + 2}</span>
                            <span>{t("clusters.story")}</span>
                          </div>
                          <div className="mt-1 line-clamp-2 text-sm font-medium leading-5 text-zinc-900">
                            {item.translated_title || item.title || item.url}
                          </div>
                          <div className="mt-2 text-xs text-zinc-500">{fmtDate(item, locale)}</div>
                        </div>
                      </Link>
                    </li>
                  ))}
                  </ul>
                </div>
              </section>
            ))}
          </div>
        )}
      </div>
    </PageTransition>
  );
}

function fmtDate(item: Item, locale: "ja" | "en") {
  const value = item.published_at || item.created_at;
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US");
}

function ThumbnailArtwork({ item, className }: { item: Item; className?: string }) {
  if (item.thumbnail_url) {
    return (
      <div className={`relative overflow-hidden bg-zinc-100 ${className ?? ""}`}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={item.thumbnail_url}
          alt={item.translated_title || item.title || item.url}
          className="h-full w-full object-cover"
          loading="lazy"
        />
      </div>
    );
  }
  return (
    <div
      className={`relative overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.24),_transparent_35%),linear-gradient(135deg,_#18181b_0%,_#27272a_42%,_#d4d4d8_100%)] ${className ?? ""}`}
    >
      <div className="absolute inset-0 bg-[linear-gradient(135deg,transparent_0%,rgba(255,255,255,0.12)_45%,transparent_100%)]" />
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/65 via-black/10 to-transparent p-3">
        <div className="line-clamp-3 text-xs font-medium leading-5 text-white/90">
          {item.translated_title || item.title || item.url}
        </div>
      </div>
    </div>
  );
}
