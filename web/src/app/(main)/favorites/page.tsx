"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, Download, ExternalLink, Star } from "lucide-react";
import { api, Item } from "@/lib/api";
import { ItemCard } from "@/components/items/item-card";
import { InlineReader } from "@/components/inline-reader";
import { EmptyState } from "@/components/empty-state";
import { PageTransition } from "@/components/page-transition";
import { SkeletonItemRow } from "@/components/skeleton";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

const EXPORT_RANGES = [
  { days: 7, key: "favorites.export.range.7" },
  { days: 30, key: "favorites.export.range.30" },
  { days: 90, key: "favorites.export.range.90" },
  { days: 0, key: "favorites.export.range.all" },
] as const;

function downloadTextFile(text: string, filename: string) {
  const blob = new Blob([text], { type: "text/markdown;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export default function FavoritesPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [exportDays, setExportDays] = useState<number>(30);
  const [exporting, setExporting] = useState<"copy" | "download" | null>(null);
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const [favoriteUpdatingIds, setFavoriteUpdatingIds] = useState<Record<string, boolean>>({});

  const favoritesQuery = useQuery({
    queryKey: ["favorites-page", 50] as const,
    queryFn: () => api.getItems({ favorite_only: true, sort: "score", page_size: 50 }),
    placeholderData: (prev) => prev,
  });

  const items = favoritesQuery.data?.items ?? [];
  const total = favoritesQuery.data?.total ?? 0;
  const queueItemIds = useMemo(() => items.map((item) => item.id), [items]);
  const loading = !favoritesQuery.data && (favoritesQuery.isLoading || favoritesQuery.isFetching);
  const error = favoritesQuery.error ? String(favoritesQuery.error) : null;

  const invalidateFavoriteRelatedQueries = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["favorites-page"] }),
      queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
      queryClient.invalidateQueries({ queryKey: ["focus-queue"] }),
      queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
    ]);
  };

  const handleExport = async (mode: "copy" | "download") => {
    setExporting(mode);
    try {
      const markdown = await api.exportFavoritesMarkdown({ days: exportDays, limit: 80 });
      const filename = `sifto-favorites-${new Date().toISOString().slice(0, 10)}.md`;
      if (mode === "copy") {
        if (typeof navigator === "undefined" || !navigator.clipboard) {
          downloadTextFile(markdown, filename);
          showToast(t("favorites.export.downloaded"), "success");
          return;
        }
        await navigator.clipboard.writeText(markdown);
        showToast(t("favorites.export.copied"), "success");
      } else {
        downloadTextFile(markdown, filename);
        showToast(t("favorites.export.downloaded"), "success");
      }
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setExporting(null);
    }
  };

  const toggleRead = async (item: Item) => {
    setReadUpdatingIds((prev) => ({ ...prev, [item.id]: true }));
    try {
      if (item.is_read) {
        await api.markItemUnread(item.id);
      } else {
        await api.markItemRead(item.id);
      }
      await invalidateFavoriteRelatedQueries();
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setReadUpdatingIds((prev) => ({ ...prev, [item.id]: false }));
    }
  };

  const removeFavorite = async (item: Item) => {
    setFavoriteUpdatingIds((prev) => ({ ...prev, [item.id]: true }));
    try {
      await api.setItemFeedback(item.id, {
        rating: item.feedback_rating ?? 0,
        is_favorite: false,
      });
      showToast(t("favorites.toast.removed"), "success");
      await invalidateFavoriteRelatedQueries();
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setFavoriteUpdatingIds((prev) => ({ ...prev, [item.id]: false }));
    }
  };

  return (
    <PageTransition>
      <div className="space-y-6">
        <section className="rounded-3xl border border-amber-200 bg-[linear-gradient(135deg,rgba(255,251,235,1),rgba(255,255,255,0.96))] p-5 shadow-sm md:p-6">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div className="max-w-2xl">
              <div className="inline-flex items-center gap-2 rounded-full border border-amber-200 bg-white/80 px-3 py-1 text-xs font-medium text-amber-700">
                <Star className="size-3.5 fill-current" aria-hidden="true" />
                <span>{t("favorites.eyebrow")}</span>
              </div>
              <h1 className="mt-3 text-2xl font-bold tracking-tight text-zinc-950">{t("favorites.title")}</h1>
              <p className="mt-2 text-sm leading-6 text-zinc-600">{t("favorites.subtitle")}</p>
              <p className="mt-3 text-sm font-medium text-zinc-700">
                {total}
                {locale === "ja" ? t("favorites.countSuffix") : ` ${t("favorites.countSuffix")}`}
              </p>
            </div>

            <div className="w-full max-w-xl rounded-2xl border border-zinc-200 bg-white/90 p-4 shadow-sm">
              <p className="text-sm font-semibold text-zinc-900">{t("favorites.export.title")}</p>
              <p className="mt-1 text-sm text-zinc-500">{t("favorites.export.subtitle")}</p>
              <div className="mt-3 flex flex-wrap gap-2">
                {EXPORT_RANGES.map((range) => (
                  <button
                    key={range.days}
                    type="button"
                    onClick={() => setExportDays(range.days)}
                    className={`rounded-full border px-3 py-1.5 text-xs font-medium press focus-ring ${
                      exportDays === range.days
                        ? "border-zinc-900 bg-zinc-900 text-white"
                        : "border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
                    }`}
                  >
                    {t(range.key)}
                  </button>
                ))}
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => void handleExport("copy")}
                  disabled={exporting != null}
                  className="inline-flex h-10 items-center gap-2 rounded-xl border border-zinc-300 bg-white px-4 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-60 press focus-ring"
                >
                  <Copy className="size-4" aria-hidden="true" />
                  <span>{exporting === "copy" ? t("common.loading") : t("favorites.export.copy")}</span>
                </button>
                <button
                  type="button"
                  onClick={() => void handleExport("download")}
                  disabled={exporting != null}
                  className="inline-flex h-10 items-center gap-2 rounded-xl border border-zinc-900 bg-zinc-900 px-4 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-60 press focus-ring"
                >
                  <Download className="size-4" aria-hidden="true" />
                  <span>{exporting === "download" ? t("common.loading") : t("favorites.export.download")}</span>
                </button>
                <Link
                  href="/items?favorite=1&sort=score"
                  className="inline-flex h-10 items-center gap-2 rounded-xl border border-zinc-300 bg-white px-4 text-sm font-medium text-zinc-700 hover:bg-zinc-50 press focus-ring"
                >
                  <ExternalLink className="size-4" aria-hidden="true" />
                  <span>{t("favorites.openItems")}</span>
                </Link>
              </div>
            </div>
          </div>
        </section>

        <section className="space-y-3">
          {loading && Array.from({ length: 5 }).map((_, idx) => <SkeletonItemRow key={idx} />)}
          {!loading && error && (
            <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {error}
            </div>
          )}
          {!loading && !error && items.length === 0 && (
            <EmptyState
              icon={Star}
              title={t("favorites.empty.title")}
              description={t("favorites.empty.desc")}
              action={{ label: t("favorites.openItems"), href: "/items?favorite=1&sort=score" }}
            />
          )}
          {!loading &&
            !error &&
            items.map((item, idx) => (
              <div key={item.id} className="rounded-2xl border border-zinc-200 bg-white p-2 shadow-sm">
                <ItemCard
                  item={item}
                  locale={locale}
                  featured={idx === 0}
                  rank={idx + 1}
                  readUpdating={Boolean(readUpdatingIds[item.id])}
                  retrying={false}
                  onOpen={() => setInlineItemId(item.id)}
                  onOpenDetail={() => router.push(`/items/${item.id}?from=${encodeURIComponent("/favorites")}`)}
                  onToggleRead={() => void toggleRead(item)}
                  onRetry={() => undefined}
                  onPrefetch={() => {
                    void queryClient.prefetchQuery({
                      queryKey: ["item-detail", item.id],
                      queryFn: () => api.getItem(item.id),
                    });
                  }}
                  t={t}
                />
                <div className="mt-2 flex justify-end border-t border-zinc-100 px-4 pb-4 pt-3">
                  <button
                    type="button"
                    onClick={() => void removeFavorite(item)}
                    disabled={Boolean(favoriteUpdatingIds[item.id])}
                    className="inline-flex h-9 items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 text-xs font-medium text-amber-800 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60 press focus-ring"
                  >
                    <Star className="size-3.5 fill-current" aria-hidden="true" />
                    <span>{favoriteUpdatingIds[item.id] ? t("common.loading") : t("favorites.remove")}</span>
                  </button>
                </div>
              </div>
            ))}
        </section>
      </div>

      <InlineReader
        itemId={inlineItemId}
        open={!!inlineItemId}
        locale={locale}
        onClose={() => setInlineItemId(null)}
        onOpenItem={(itemId) => setInlineItemId(itemId)}
        onOpenDetail={(itemId) => router.push(`/items/${itemId}?from=${encodeURIComponent("/favorites")}`)}
        queueItemIds={queueItemIds}
        onReadToggled={() => {
          void invalidateFavoriteRelatedQueries();
        }}
        onFeedbackUpdated={() => {
          void invalidateFavoriteRelatedQueries();
        }}
      />
    </PageTransition>
  );
}
