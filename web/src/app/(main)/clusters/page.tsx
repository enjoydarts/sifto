"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, BriefingCluster, Item } from "@/lib/api";
import { PageTransition } from "@/components/page-transition";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

const EMPTY_CLUSTERS: BriefingCluster[] = [];

export default function ClustersPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [savingClusterId, setSavingClusterId] = useState<string | null>(null);
  const [hiddenClusterIds, setHiddenClusterIds] = useState<Record<string, true>>({});

  const briefingQuery = useQuery({
    queryKey: ["briefing-clusters", 24] as const,
    queryFn: () => api.getBriefingToday({ size: 24 }),
    staleTime: 15_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
    placeholderData: (prev) => prev,
  });

  const clusters = briefingQuery.data?.clusters ?? EMPTY_CLUSTERS;
  const rows = useMemo(
    () => clusters.filter((cluster) => (cluster.items ?? []).length > 0 && !hiddenClusterIds[cluster.id]),
    [clusters, hiddenClusterIds]
  );

  const totalUnread = useMemo(
    () => rows.reduce((sum, cluster) => sum + cluster.items.filter((item) => !item.is_read).length, 0),
    [rows]
  );

  const topRepresentativeScore = useMemo(() => {
    const scores = rows
      .map((cluster) => cluster.items[0]?.summary_score)
      .filter((value): value is number => typeof value === "number");
    if (scores.length === 0) return null;
    return Math.max(...scores);
  }, [rows]);

  const latestClusterUpdate = useMemo(() => {
    const timestamps = rows
      .flatMap((cluster) => cluster.items)
      .map((item) => item.published_at || item.created_at)
      .filter((value): value is string => Boolean(value))
      .map((value) => new Date(value).getTime())
      .filter((value) => !Number.isNaN(value));
    if (timestamps.length === 0) return null;
    return new Date(Math.max(...timestamps)).toISOString();
  }, [rows]);

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
      setHiddenClusterIds((prev) => ({ ...prev, [cluster.id]: true }));
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
      setHiddenClusterIds((prev) => ({ ...prev, [cluster.id]: true }));
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setSavingClusterId(null);
    }
  };

  return (
    <PageTransition>
      <div className="space-y-6">
        <section className="space-y-4">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                {t("clusters.eyebrow")}
              </div>
              <h1 className="mt-2 font-serif text-[40px] leading-[1.08] tracking-[-0.04em] text-[var(--color-editorial-ink)] md:text-[44px]">
                {t("clusters.title")}
              </h1>
              <p className="mt-3 max-w-[70ch] text-[15px] leading-8 text-[var(--color-editorial-ink-soft)]">
                {t("clusters.subtitle")}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-xs text-[var(--color-editorial-ink-soft)]">
                {rows.length} {t("clusters.summary.clusterCountSuffix")} / {totalUnread} {t("clusters.summary.unreadCountSuffix")}
              </span>
            </div>
          </div>
        </section>

        {briefingQuery.isLoading && !briefingQuery.data ? (
          <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] px-5 py-5 text-sm text-[var(--color-editorial-ink-soft)] shadow-[var(--shadow-card)]">
            {t("common.loading")}
          </section>
        ) : rows.length === 0 ? (
          <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] px-5 py-5 text-sm text-[var(--color-editorial-ink-soft)] shadow-[var(--shadow-card)]">
            {t("clusters.empty")}
          </section>
        ) : (
          <>
            <section className="grid gap-4 xl:grid-cols-4">
              <MetricCard
                label={t("clusters.summary.activeThemes")}
                value={String(rows.length)}
                note={t("clusters.summary.activeThemesNote")}
              />
              <MetricCard
                label={t("clusters.summary.unreadAcrossClusters")}
                value={String(totalUnread)}
                note={t("clusters.summary.unreadAcrossClustersNote")}
              />
              <MetricCard
                label={t("clusters.summary.topRepresentativeScore")}
                value={topRepresentativeScore != null ? topRepresentativeScore.toFixed(2) : "-"}
                note={t("clusters.summary.topRepresentativeScoreNote")}
              />
              <MetricCard
                label={t("clusters.summary.newestClusterUpdate")}
                value={latestClusterUpdate ? formatTimeOnly(latestClusterUpdate) : "-"}
                note={t("clusters.summary.newestClusterUpdateNote")}
              />
            </section>

            <section className="grid gap-4 xl:grid-cols-2">
              {rows.map((cluster) => {
                const representative = cluster.items[0];
                const restItems = cluster.items.slice(1, 4);
                const unreadCount = cluster.items.filter((item) => !item.is_read).length;
                const averageScore = averageClusterScore(cluster.items);
                const latestItem = cluster.items.reduce<Item | null>((latest, item) => {
                  const currentTime = new Date(item.published_at || item.created_at || 0).getTime();
                  if (!latest) return item;
                  const latestTime = new Date(latest.published_at || latest.created_at || 0).getTime();
                  return currentTime > latestTime ? item : latest;
                }, null);
                const disabled = savingClusterId === cluster.id || savingClusterId === `read:${cluster.id}`;

                return (
                  <article
                    key={cluster.id}
                    className="overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.78)] shadow-[var(--shadow-card)]"
                  >
                    <div className="p-5 md:p-6">
                      <div className="mb-4 flex flex-wrap gap-2">
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                          {unreadCount} {t("clusters.card.unreadSuffix")}
                        </span>
                        {averageScore != null ? (
                          <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                            {t("clusters.card.avgScore")} {averageScore.toFixed(2)}
                          </span>
                        ) : null}
                        {latestItem ? (
                          <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                            {t("clusters.card.updated")} {formatTimeOnly(latestItem.published_at || latestItem.created_at)}
                          </span>
                        ) : null}
                      </div>

                      <h2 className="font-serif text-[30px] leading-[1.14] tracking-[-0.04em] text-[var(--color-editorial-ink)]">
                        {cluster.label || t("briefing.clusterFallback")}
                      </h2>
                      {cluster.summary ? (
                        <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{cluster.summary}</p>
                      ) : null}

                      {representative ? (
                        <Link
                          href={`/items/${representative.id}?from=${encodeURIComponent("/clusters")}`}
                          className="mt-5 block overflow-hidden rounded-[18px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,#faf6ef,#fffdfa)]"
                        >
                          <ThumbnailArtwork item={representative} className="h-44 w-full" />
                          <div className="p-4">
                            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                              {t("clusters.lead")}
                            </div>
                            <h3 className="mt-2 text-[16px] font-semibold leading-7 text-[var(--color-editorial-ink)]">
                              {representative.translated_title || representative.title || representative.url}
                            </h3>
                            <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                              {cluster.summary ||
                                representative.recommendation_reason ||
                                representative.content_text ||
                                t("clusters.card.noRepresentativeSummary")}
                            </p>
                          </div>
                        </Link>
                      ) : null}

                      {cluster.topics && cluster.topics.length > 0 ? (
                        <div className="mt-4 flex flex-wrap gap-2">
                          {cluster.topics.slice(0, 4).map((topic) => (
                            <span
                              key={`${cluster.id}-${topic}`}
                              className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                            >
                              {topic}
                            </span>
                          ))}
                        </div>
                      ) : null}

                      {restItems.length > 0 ? (
                        <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.58)] px-4 py-3">
                          <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                            {t("clusters.moreInCluster")}
                          </div>
                          <div className="mt-3 grid gap-3">
                            {restItems.map((item) => (
                              <Link
                                key={item.id}
                                href={`/items/${item.id}?from=${encodeURIComponent("/clusters")}`}
                                className="block border-t border-[var(--color-editorial-line)] pt-3 first:border-t-0 first:pt-0"
                              >
                                <strong className="block text-sm leading-6 text-[var(--color-editorial-ink)]">
                                  {item.translated_title || item.title || item.url}
                                </strong>
                                <span className="mt-1 block text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                                  {item.recommendation_reason ||
                                    item.content_text ||
                                    fmtDateTime(item.published_at || item.created_at, locale)}
                                </span>
                              </Link>
                            ))}
                            {cluster.items.length > 4 ? (
                              <div className="border-t border-[var(--color-editorial-line)] pt-3">
                                <strong className="block text-sm leading-6 text-[var(--color-editorial-ink)]">
                                  +{cluster.items.length - 4} {t("clusters.moreSuffix")}
                                </strong>
                                <span className="mt-1 block text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                                  {t("clusters.moreNote")}
                                </span>
                              </div>
                            ) : null}
                          </div>
                        </div>
                      ) : null}
                    </div>

                    <div className="flex flex-wrap gap-2 px-5 pb-5 md:px-6 md:pb-6">
                      <button
                        type="button"
                        onClick={() => void saveClusterForLater(cluster)}
                        disabled={disabled}
                        className="inline-flex min-h-[42px] items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-semibold text-[var(--color-editorial-panel-strong)] disabled:cursor-wait disabled:opacity-60"
                      >
                        {savingClusterId === cluster.id ? t("briefing.clusterLaterSaving") : t("clusters.action.later")}
                      </button>
                      <button
                        type="button"
                        onClick={() => void markClusterRead(cluster)}
                        disabled={disabled}
                        className="inline-flex min-h-[42px] items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-semibold text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:cursor-wait disabled:opacity-60"
                      >
                        {savingClusterId === `read:${cluster.id}` ? t("briefing.clusterReadSaving") : t("clusters.action.read")}
                      </button>
                    </div>
                  </article>
                );
              })}
            </section>
          </>
        )}
      </div>
    </PageTransition>
  );
}

function MetricCard({
  label,
  value,
  note,
}: {
  label: string;
  value: string;
  note: string;
}) {
  return (
    <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] px-5 py-4 shadow-[var(--shadow-card)]">
      <div className="text-[11px] uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{label}</div>
      <div className="mt-2 font-serif text-[30px] text-[var(--color-editorial-ink)]">{value}</div>
      <div className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{note}</div>
    </section>
  );
}

function averageClusterScore(items: Item[]) {
  const scores = items
    .map((item) => item.summary_score)
    .filter((value): value is number => typeof value === "number");
  if (scores.length === 0) return null;
  return scores.reduce((sum, value) => sum + value, 0) / scores.length;
}

function fmtDateTime(value?: string | null, locale?: "ja" | "en") {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleString(locale === "ja" ? "ja-JP" : "en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatTimeOnly(value: string) {
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "-";
  const hours = String(d.getHours()).padStart(2, "0");
  const minutes = String(d.getMinutes()).padStart(2, "0");
  return `${hours}:${minutes}`;
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
      className={`relative overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(146,121,89,0.24),_transparent_35%),linear-gradient(135deg,_#2f2721_0%,_#6d5b4b_42%,_#d8cdbf_100%)] ${className ?? ""}`}
    >
      <div className="absolute inset-0 bg-[linear-gradient(135deg,transparent_0%,rgba(255,255,255,0.12)_45%,transparent_100%)]" />
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/55 via-black/10 to-transparent p-3">
        <div className="line-clamp-3 text-xs font-medium leading-5 text-white/90">
          {item.translated_title || item.title || item.url}
        </div>
      </div>
    </div>
  );
}
