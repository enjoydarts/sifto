"use client";

import { ExternalLink, Star, ThumbsDown, ThumbsUp } from "lucide-react";
import { type Item } from "@/lib/api";
import { Thumbnail } from "@/components/thumbnail";
import { CheckStatusBadges } from "@/components/items/check-status-badges";
import { ListRowCard } from "@/components/ui/list-row-card";
import { StatusPill } from "@/components/ui/status-pill";
import { Tag } from "@/components/ui/tag";

function processingStatusLabel(status: Item["status"], t: (key: string) => string) {
  switch (status) {
    case "new":
      return t("items.processing.new");
    case "fetched":
      return t("items.processing.fetched");
    case "facts_extracted":
      return t("items.processing.factsExtracted");
    case "failed":
      return t("items.processing.failed");
    default:
      return status;
  }
}

function searchSnippetLabel(field: string, t: (key: string) => string) {
  switch (field) {
    case "title":
      return t("items.search.snippet.title");
    case "summary":
      return t("items.search.snippet.summary");
    case "facts":
      return t("items.search.snippet.facts");
    case "content":
      return t("items.search.snippet.content");
    default:
      return field;
  }
}

export function ItemCard({
  item,
  featured = false,
  rank,
  locale,
  readUpdating,
  retrying,
  onOpen,
  onOpenDetail,
  onToggleRead,
  onRetry,
  onPrefetch,
  animationDelay,
  t,
}: {
  item: Item;
  featured?: boolean;
  rank?: number;
  locale: "ja" | "en";
  readUpdating: boolean;
  retrying: boolean;
  onOpen: () => void;
  onOpenDetail: () => void;
  onToggleRead: () => void;
  onRetry: () => void;
  onPrefetch: () => void;
  animationDelay?: number;
  t: (key: string) => string;
}) {
  const displayTitle = item.translated_title?.trim() ? item.translated_title : item.title;
  const isRead = item.is_read;
  const canToggleRead = item.status !== "deleted";
  const pendingState = item.status !== "summarized";
  const isFailed = item.status === "failed";
  const processingErrorSnippet = item.processing_error?.trim()
    ? item.processing_error.trim().slice(0, 160)
    : null;
  const dek =
    item.content_text?.trim()?.replace(/\s+/g, " ").slice(0, 150) ||
    item.recommendation_reason?.trim() ||
    null;

  const reactionPill = item.is_favorite
    ? { icon: <Star className="size-3 fill-current" aria-hidden="true" />, label: t("items.feedback.favorite"), tone: "success" as const }
    : item.feedback_rating === 1
      ? { icon: <ThumbsUp className="size-3" aria-hidden="true" />, label: t("items.feedback.like"), tone: "success" as const }
      : item.feedback_rating === -1
        ? { icon: <ThumbsDown className="size-3" aria-hidden="true" />, label: t("items.feedback.dislike"), tone: "error" as const }
        : null;

  const style = animationDelay != null ? { animationDelay: `${animationDelay}ms` } : undefined;

  return (
    <ListRowCard className="motion-safe:animate-fade-in-up" featured={featured} read={isRead} style={style}>
      <div
        data-item-row-id={item.id}
        role="button"
        tabIndex={0}
        onClick={onOpen}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onOpen();
          }
        }}
        onMouseEnter={onPrefetch}
        onFocus={onPrefetch}
        onTouchStart={onPrefetch}
        className={`group grid min-w-0 gap-4 ${featured ? "md:grid-cols-[144px_minmax(0,1fr)_188px] md:items-start" : "sm:grid-cols-[132px_minmax(0,1fr)_188px] sm:items-start"}`}
      >
        <div
          className={`shrink-0 ${
            featured
              ? "h-36 w-full md:h-[108px] md:w-[144px]"
              : "h-[188px] w-full sm:h-[99px] sm:w-[132px]"
          }`}
        >
          <Thumbnail
            src={item.thumbnail_url}
            title={displayTitle ?? item.url}
            size={featured ? "lg" : "md"}
            tone="editorial"
            className="h-full w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-muted)]"
          />
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="min-w-0 flex-1">
            <div className="mb-2 flex flex-wrap items-center gap-1.5">
              {featured && rank != null && rank > 0 && (
                <span className="inline-flex items-center rounded-full bg-[var(--color-editorial-ink)] px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-panel-strong)]">
                  Pick #{rank}
                </span>
              )}
              <StatusPill tone={isRead ? "neutral" : "default"}>
                {isRead ? t("items.read.read") : t("items.read.unread")}
              </StatusPill>
              {pendingState && (
                <StatusPill tone={isFailed ? "error" : "info"}>
                  {processingStatusLabel(item.status, t)}
                </StatusPill>
              )}
            </div>

            <div
              className={`overflow-hidden break-words font-serif tracking-[-0.025em] ${
                featured
                  ? isRead
                    ? "line-clamp-3 text-[22px] font-semibold leading-[1.3] text-[var(--color-editorial-ink-faint)]"
                    : "line-clamp-3 text-[26px] font-semibold leading-[1.25] text-[var(--color-editorial-ink)]"
                  : isRead
                    ? "line-clamp-4 text-[21px] font-semibold leading-[1.35] text-[var(--color-editorial-ink-faint)] sm:line-clamp-3 sm:text-[20px]"
                    : "line-clamp-4 text-[24px] font-semibold leading-[1.3] text-[var(--color-editorial-ink)] sm:line-clamp-3 sm:text-[23px]"
              }`}
            >
              {displayTitle ?? item.url}
            </div>

            {dek ? (
              <p className="mt-2 line-clamp-3 break-words text-[15px] leading-[1.7] text-[var(--color-editorial-ink-soft)] sm:line-clamp-2 sm:text-[14px] sm:leading-[1.65]">
                {dek}
              </p>
            ) : null}

            {Array.isArray(item.search_snippets) && item.search_snippets.length > 0 ? (
              <div className="mt-3 space-y-2">
                {item.search_snippets.slice(0, 3).map((snippet, index) => (
                  <div
                    key={`${snippet.field}-${index}`}
                    className="rounded-xl border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-muted)] px-3 py-2"
                  >
                    <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {searchSnippetLabel(snippet.field, t)}
                    </div>
                    <div
                      className="mt-1 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)] [&_mark]:rounded-[4px] [&_mark]:bg-[rgba(246,199,94,0.42)] [&_mark]:px-1 [&_mark]:py-0.5"
                      dangerouslySetInnerHTML={{ __html: snippet.snippet_html }}
                    />
                  </div>
                ))}
              </div>
            ) : null}

            <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] uppercase tracking-[0.1em] text-[var(--color-editorial-ink-faint)] sm:mt-2 sm:text-xs">
              <span>
                {new Date(item.published_at ?? item.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US", {
                  year: "numeric",
                  month: "2-digit",
                  day: "2-digit",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </span>
              {item.source_title ? <span>{item.source_title}</span> : null}
            </div>

            {reactionPill ? (
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Tag tone={reactionPill.tone} icon={reactionPill.icon}>
                  {reactionPill.label}
                </Tag>
              </div>
            ) : null}

            {!isFailed && pendingState ? (
              <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)] sm:text-xs">
                {t("items.processing.pendingHint")}
              </p>
            ) : null}

            <div className="mt-2">
              <CheckStatusBadges
                factsCheckResult={item.facts_check_result}
                faithfulnessResult={item.faithfulness_result}
                t={t}
                compact
              />
            </div>

            {isFailed && processingErrorSnippet ? (
              <p className="mt-2 line-clamp-2 text-[11px] leading-5 text-[var(--color-editorial-error)] sm:text-xs">
                {processingErrorSnippet}
              </p>
            ) : null}
          </div>

          <div className="hidden h-4 truncate break-all text-[12px] text-[var(--color-editorial-ink-faint)] sm:block">
            {item.url}
          </div>
        </div>

        <div
          className={`flex gap-3 ${
            featured
              ? "self-start md:w-[188px] md:flex-col md:items-stretch"
              : "w-full flex-col items-stretch justify-start border-t border-[var(--color-editorial-line)] pt-4 sm:w-[188px] sm:border-t-0 sm:pt-0"
          }`}
        >
          <div className="w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2.5 sm:px-3 sm:py-3">
            <div className="sm:hidden">
              <div className="flex items-center justify-between gap-3">
                <div className="text-[10px] font-semibold uppercase tracking-[0.15em] text-[var(--color-editorial-ink-faint)]">
                  {t("items.state.meta")}
                </div>
                <div
                  className={`grid min-w-0 flex-1 gap-2 ${
                    item.personal_score != null ? "grid-cols-2" : "grid-cols-1"
                  }`}
                >
                  {item.personal_score != null && (
                    <div className="min-w-0 text-right text-[var(--color-editorial-ink)]">
                      <div className="truncate text-[10px] leading-none text-[var(--color-editorial-ink-faint)]">
                        {t("items.sort.personal_score")}
                      </div>
                      <div className="mt-1 text-[17px] leading-none tracking-[-0.03em]">
                        {item.personal_score.toFixed(2)}
                      </div>
                    </div>
                  )}
                  <div className="min-w-0 text-right text-[var(--color-editorial-ink)]">
                    <div className="truncate text-[10px] leading-none text-[var(--color-editorial-ink-faint)]">
                      {t("items.sort.score")}
                    </div>
                    <div className="mt-1 text-[17px] leading-none tracking-[-0.03em]">
                      {item.summary_score != null ? item.summary_score.toFixed(2) : "—"}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div className="hidden sm:block">
              <div className="text-center text-[10px] font-semibold uppercase tracking-[0.15em] text-[var(--color-editorial-ink-faint)]">
                {t("items.state.meta")}
              </div>
              <div
                className={`mt-3 grid gap-3 ${
                  item.personal_score != null ? "grid-cols-2" : "grid-cols-1"
                }`}
              >
                {item.personal_score != null && (
                  <div className="min-w-0 text-center">
                    <div className="text-center text-[10px] leading-tight text-[var(--color-editorial-ink-faint)]">
                      {t("items.sort.personal_score")}
                    </div>
                    <div className="mt-1 text-[20px] font-semibold leading-none tabular-nums tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                      {item.personal_score.toFixed(2)}
                    </div>
                  </div>
                )}
                <div className="min-w-0 text-center">
                  <div className="text-center text-[10px] leading-tight text-[var(--color-editorial-ink-faint)]">
                    {t("items.sort.score")}
                  </div>
                  <div className="mt-1 text-[20px] font-semibold leading-none tabular-nums tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    {item.summary_score != null ? item.summary_score.toFixed(2) : "—"}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="grid w-full gap-2">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onOpenDetail();
              }}
              className="inline-flex min-h-11 w-full items-center justify-center gap-1 rounded-full border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition-colors hover:bg-[var(--color-editorial-panel-strong)] press focus-ring sm:h-9 sm:min-h-0 sm:text-sm"
            >
              <ExternalLink className="size-3.5" aria-hidden="true" />
              <span className="sm:hidden">{t("items.action.openShort")}</span>
              <span className="hidden sm:inline">{t("items.action.openDetail")}</span>
            </button>

            {canToggleRead && (
              <button
                type="button"
                disabled={readUpdating}
                onClick={(e) => {
                  e.stopPropagation();
                  onToggleRead();
                }}
                className={`min-h-11 w-full rounded-full px-3 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 press focus-ring sm:h-9 sm:min-h-0 sm:text-sm ${
                  isRead
                    ? "border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    : "border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                }`}
              >
                {readUpdating
                  ? t("items.action.updating")
                  : isRead
                    ? (
                      <>
                        <span className="sm:hidden">{t("items.action.markUnreadShort")}</span>
                        <span className="hidden sm:inline">{t("items.action.markUnread")}</span>
                      </>
                    )
                    : (
                      <>
                        <span className="sm:hidden">{t("items.action.markReadShort")}</span>
                        <span className="hidden sm:inline">{t("items.action.markRead")}</span>
                      </>
                    )}
              </button>
            )}

            {item.status === "failed" && (
              <button
                type="button"
                disabled={retrying}
                onClick={(e) => {
                  e.stopPropagation();
                  onRetry();
                }}
                className="min-h-11 w-full rounded-full border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition-colors hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50 press focus-ring sm:h-9 sm:min-h-0 sm:text-sm"
              >
                {retrying ? t("items.retrying") : (
                  <>
                    <span className="sm:hidden">{t("items.retryShort")}</span>
                    <span className="hidden sm:inline">{t("items.retry")}</span>
                  </>
                )}
              </button>
            )}
          </div>
        </div>
      </div>
    </ListRowCard>
  );
}
