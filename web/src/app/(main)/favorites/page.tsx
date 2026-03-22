"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, Download, ExternalLink, Star } from "lucide-react";
import { api, AskInsight, Item, ItemDetail } from "@/lib/api";
import { CheckStatusBadges } from "@/components/items/check-status-badges";
import { InlineReader } from "@/components/inline-reader";
import { useConfirm } from "@/components/confirm-provider";
import { EmptyState } from "@/components/empty-state";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { Thumbnail } from "@/components/thumbnail";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { StatusPill } from "@/components/ui/status-pill";
import { Tag } from "@/components/ui/tag";

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
  const { confirm } = useConfirm();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [exportDays, setExportDays] = useState<number>(30);
  const [exporting, setExporting] = useState<"copy" | "download" | null>(null);
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const [favoriteUpdatingIds, setFavoriteUpdatingIds] = useState<Record<string, boolean>>({});
  const [detailNotes, setDetailNotes] = useState<Record<string, string>>({});
  const [deletingInsightId, setDeletingInsightId] = useState<string | null>(null);

  const favoritesQuery = useQuery({
    queryKey: ["favorites-page", 50] as const,
    queryFn: () => api.getItems({ favorite_only: true, sort: "score", page_size: 50 }),
    placeholderData: (prev) => prev,
  });
  const insightsQuery = useQuery({
    queryKey: ["ask-insights", 6] as const,
    queryFn: () => api.getAskInsights({ limit: 6 }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const items = useMemo(() => favoritesQuery.data?.items ?? [], [favoritesQuery.data?.items]);
  const insights = useMemo(() => insightsQuery.data?.insights ?? [], [insightsQuery.data?.insights]);
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

  const prefetchDetail = async (itemId: string) => {
    const detail = await queryClient.fetchQuery<ItemDetail>({
      queryKey: ["item-detail", itemId],
      queryFn: () => api.getItem(itemId),
      staleTime: 60_000,
    });
    setDetailNotes((prev) => {
      const nextNote = detail.note?.content?.trim() ?? "";
      if ((prev[itemId] ?? "") === nextNote) return prev;
      return { ...prev, [itemId]: nextNote };
    });
    return detail;
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

  const deleteInsight = async (insight: AskInsight) => {
    const ok = await confirm({
      title: t("ask.insight.delete.title"),
      message: t("ask.insight.delete.message"),
      confirmLabel: t("ask.insight.delete.confirm"),
      cancelLabel: t("common.cancel"),
      tone: "danger",
    });
    if (!ok) return;
    setDeletingInsightId(insight.id);
    try {
      await api.deleteAskInsight(insight.id);
      await queryClient.invalidateQueries({ queryKey: ["ask-insights"] });
      showToast(t("ask.insight.delete.done"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setDeletingInsightId(null);
    }
  };

  return (
    <PageTransition>
      <div className="space-y-6 overflow-x-hidden">
        <section className="grid gap-4 xl:grid-cols-[minmax(0,1.18fr)_minmax(340px,0.82fr)]">
          <PageHeader
            title={t("favorites.title")}
            titleIcon={Star}
            eyebrow={t("favorites.eyebrow")}
            description={t("favorites.subtitle")}
            compact
            actions={
              <div className="inline-flex items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-xs text-[var(--color-editorial-ink-soft)]">
              <Star className="size-3.5 fill-current text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
              <span>
                {total}
                {locale === "ja" ? t("favorites.countSuffix") : ` ${t("favorites.countSuffix")}`}
              </span>
            </div>
            }
          />

          <div className="rounded-[24px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(255,253,249,0.98))] px-5 py-5 shadow-[var(--shadow-card)]">
            <h2 className="font-serif text-[24px] leading-[1.2] text-[var(--color-editorial-ink)]">
              {t("favorites.export.title")}
            </h2>
            <p className="mt-2 text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">
              {t("favorites.export.subtitle")}
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
                {EXPORT_RANGES.map((range) => (
                  <button
                    key={range.days}
                    type="button"
                    onClick={() => setExportDays(range.days)}
                    className={`rounded-full border px-3 py-2 text-xs font-medium press focus-ring ${
                      exportDays === range.days
                        ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
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
                  className="inline-flex min-h-[42px] items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:cursor-not-allowed disabled:opacity-60 press focus-ring"
                >
                  <Copy className="size-4" aria-hidden="true" />
                  <span>{exporting === "copy" ? t("common.loading") : t("favorites.export.copy")}</span>
                </button>
                <button
                  type="button"
                  onClick={() => void handleExport("download")}
                  disabled={exporting != null}
                  className="inline-flex min-h-[42px] items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-60 press focus-ring"
                >
                  <Download className="size-4" aria-hidden="true" />
                  <span>{exporting === "download" ? t("common.loading") : t("favorites.export.download")}</span>
                </button>
                <Link
                  href="/items?favorite=1&sort=score"
                  className="inline-flex min-h-[42px] items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] press focus-ring"
                >
                  <ExternalLink className="size-4" aria-hidden="true" />
                  <span>{t("favorites.openItems")}</span>
                </Link>
              </div>
          </div>
        </section>

        <section className="space-y-3">
          {loading && Array.from({ length: 4 }).map((_, idx) => <FavoriteArchiveSkeleton key={idx} />)}
          {!loading && error && (
            <div className="rounded-[22px] border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
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
              <FavoriteArchiveCard
                key={item.id}
                item={item}
                locale={locale}
                featured={idx === 0}
                note={detailNotes[item.id]}
                readUpdating={Boolean(readUpdatingIds[item.id])}
                favoriteUpdating={Boolean(favoriteUpdatingIds[item.id])}
                onOpen={() => setInlineItemId(item.id)}
                onOpenDetail={() => router.push(`/items/${item.id}?from=${encodeURIComponent("/favorites")}`)}
                onToggleRead={() => void toggleRead(item)}
                onRemoveFavorite={() => void removeFavorite(item)}
                onPrefetch={() => {
                  void prefetchDetail(item.id);
                }}
                t={t}
              />
            ))}
        </section>

        {insights.length > 0 ? (
          <section className="rounded-[26px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.78)] p-5 shadow-[var(--shadow-card)]">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="font-serif text-[24px] leading-[1.2] text-[var(--color-editorial-ink)]">
                  {t("favorites.insights.title")}
                </h2>
                <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("favorites.insights.subtitle")}</p>
              </div>
            </div>
            <div className="mt-4 grid gap-3 xl:grid-cols-2">
              {insights.map((insight) => (
                <div
                  key={insight.id}
                  className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.64)] p-4"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-sm font-semibold leading-7 text-[var(--color-editorial-ink)]">{insight.title}</div>
                      <p className="mt-2 line-clamp-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{insight.body}</p>
                      {insight.tags && insight.tags.length > 0 ? (
                        <div className="mt-3 flex flex-wrap gap-2">
                          {insight.tags.map((tag) => (
                            <Tag key={tag}>{tag}</Tag>
                          ))}
                        </div>
                      ) : null}
                    </div>
                    <button
                      type="button"
                      onClick={() => void deleteInsight(insight)}
                      disabled={deletingInsightId === insight.id}
                      className="rounded-full border border-[var(--color-editorial-line)] px-3 py-2 text-xs text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
                    >
                      {deletingInsightId === insight.id ? t("common.loading") : t("common.delete")}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </section>
        ) : null}
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

function FavoriteArchiveCard({
  item,
  locale,
  featured,
  note,
  readUpdating,
  favoriteUpdating,
  onOpen,
  onOpenDetail,
  onToggleRead,
  onRemoveFavorite,
  onPrefetch,
  t,
}: {
  item: Item;
  locale: "ja" | "en";
  featured: boolean;
  note?: string;
  readUpdating: boolean;
  favoriteUpdating: boolean;
  onOpen: () => void;
  onOpenDetail: () => void;
  onToggleRead: () => void;
  onRemoveFavorite: () => void;
  onPrefetch: () => void;
  t: (key: string) => string;
}) {
  const displayTitle = item.translated_title?.trim() ? item.translated_title : item.title;
  const lead =
    excerptText(item.content_text, 520) ||
    item.recommendation_reason?.trim() ||
    note?.trim() ||
    null;
  const savedReason = note?.trim() || item.recommendation_reason?.trim() || t("favorites.archive.savedReasonEmpty");
  const isRead = item.is_read;

  return (
    <article className="overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.78)] shadow-[var(--shadow-card)]">
      <div className="grid gap-4 p-4 lg:grid-cols-[minmax(0,1fr)_190px] lg:p-[18px]">
        <div className="grid min-w-0 gap-4 md:grid-cols-[188px_minmax(0,1fr)]">
          <button
            type="button"
            onClick={onOpen}
            onMouseEnter={onPrefetch}
            onFocus={onPrefetch}
            onTouchStart={onPrefetch}
            className="block min-w-0 overflow-hidden rounded-[18px] border border-[var(--color-editorial-line)] text-left"
          >
            <Thumbnail
              src={item.thumbnail_url}
              title={displayTitle ?? item.url}
              size="lg"
              tone="editorial"
              className={`h-full min-h-[138px] w-full ${featured ? "md:min-h-[160px]" : ""}`}
            />
          </button>

          <div className="min-w-0">
            <div className="flex flex-wrap gap-2">
              <StatusPill tone={isRead ? "neutral" : "default"}>
                {isRead ? t("items.read.read") : t("items.read.unread")}
              </StatusPill>
              {item.source_title ? <Tag tone="subtle">{item.source_title}</Tag> : null}
              <Tag tone="subtle">{formatFavoriteDate(item.published_at ?? item.created_at, locale)}</Tag>
            </div>

            <button
              type="button"
              onClick={onOpen}
              onMouseEnter={onPrefetch}
              onFocus={onPrefetch}
              onTouchStart={onPrefetch}
              className="mt-3 block text-left"
            >
              <h2 className="font-serif text-[24px] leading-[1.18] tracking-[-0.03em] text-[var(--color-editorial-ink)] md:text-[29px]">
                {displayTitle ?? item.url}
              </h2>
            </button>

            {lead ? (
              <p className="mt-3 text-[14px] leading-8 text-[var(--color-editorial-ink-soft)]">{lead}</p>
            ) : null}

            <div className="mt-3">
              <CheckStatusBadges
                factsCheckResult={item.facts_check_result}
                faithfulnessResult={item.faithfulness_result}
                t={t}
                compact
              />
            </div>
          </div>
        </div>

        <div className="grid content-start gap-3">
          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,#faf6ef,#fffdfa)] px-4 py-3">
            <div className="text-center text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("items.sort.score")}
            </div>
            <div className="mt-2 text-center font-serif text-[28px] leading-none text-[var(--color-editorial-ink)]">
              {item.summary_score != null ? item.summary_score.toFixed(2) : "—"}
            </div>
          </div>

          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,#faf6ef,#fffdfa)] px-4 py-3">
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("favorites.archive.savedReason")}
            </div>
            <p className="mt-2 text-[13px] leading-7 text-[var(--color-editorial-ink-soft)]">{savedReason}</p>
          </div>

          <div className="grid gap-2">
            <button
              type="button"
              onClick={onOpenDetail}
              className="inline-flex min-h-[42px] items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-semibold text-[var(--color-editorial-panel-strong)]"
            >
              {t("items.action.openDetail")}
            </button>
            <button
              type="button"
              onClick={onToggleRead}
              disabled={readUpdating}
              className="inline-flex min-h-[42px] items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-semibold text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
            >
              {readUpdating ? t("common.loading") : isRead ? t("items.read.markUnread") : t("items.read.markRead")}
            </button>
            <button
              type="button"
              onClick={onRemoveFavorite}
              disabled={favoriteUpdating}
              className="inline-flex min-h-[42px] items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-semibold text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
            >
              <Star className="size-3.5 fill-current" aria-hidden="true" />
              <span>{favoriteUpdating ? t("common.loading") : t("favorites.remove")}</span>
            </button>
          </div>
        </div>
      </div>
    </article>
  );
}

function FavoriteArchiveSkeleton() {
  return (
    <div className="overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.78)] p-4 shadow-[var(--shadow-card)]">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_190px]">
        <div className="grid gap-4 md:grid-cols-[188px_minmax(0,1fr)]">
          <div className="min-h-[138px] animate-pulse rounded-[18px] bg-[var(--color-editorial-panel)]" />
          <div className="space-y-3">
            <div className="h-5 w-40 animate-pulse rounded-full bg-[var(--color-editorial-panel)]" />
            <div className="h-10 w-5/6 animate-pulse rounded-2xl bg-[var(--color-editorial-panel)]" />
            <div className="h-20 animate-pulse rounded-2xl bg-[var(--color-editorial-panel)]" />
          </div>
        </div>
        <div className="space-y-3">
          <div className="h-24 animate-pulse rounded-[18px] bg-[var(--color-editorial-panel)]" />
          <div className="h-24 animate-pulse rounded-[18px] bg-[var(--color-editorial-panel)]" />
          <div className="h-11 animate-pulse rounded-full bg-[var(--color-editorial-panel)]" />
        </div>
      </div>
    </div>
  );
}

function formatFavoriteDate(value: string | null | undefined, locale: "ja" | "en") {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString(locale === "ja" ? "ja-JP" : "en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function excerptText(value: string | null | undefined, maxLength: number) {
  const normalized = value?.trim().replace(/\s+/g, " ");
  if (!normalized) return null;
  if (normalized.length <= maxLength) return normalized;
  return `${normalized.slice(0, maxLength).trimEnd()}...`;
}
