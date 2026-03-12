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
          <div className="grid gap-4 xl:grid-cols-2">
            {rows.map((cluster) => (
              <section key={cluster.id} className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h2 className="truncate text-lg font-semibold text-zinc-900">
                      {cluster.label || t("briefing.clusterFallback")}
                    </h2>
                    {cluster.summary ? <p className="mt-1 text-sm text-zinc-600">{cluster.summary}</p> : null}
                  </div>
                  <span className="rounded-full border border-zinc-300 bg-zinc-50 px-2 py-1 text-xs text-zinc-600">
                    {cluster.items.length}
                  </span>
                </div>

                <div className="mt-4 flex flex-wrap gap-2">
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

                <ul className="mt-4 space-y-2">
                  {cluster.items.slice(0, 6).map((item) => (
                    <li key={item.id}>
                      <Link
                        href={`/items/${item.id}?from=${encodeURIComponent("/clusters")}`}
                        className="block rounded-xl border border-zinc-200 bg-zinc-50 px-3 py-3 hover:border-zinc-300 hover:bg-zinc-100"
                      >
                        <div className="line-clamp-2 text-sm font-medium text-zinc-900">
                          {item.translated_title || item.title || item.url}
                        </div>
                        <div className="mt-1 text-xs text-zinc-500">{fmtDate(item, locale)}</div>
                      </Link>
                    </li>
                  ))}
                </ul>
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
