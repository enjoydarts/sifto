"use client";

import { ExternalLink, Star, ThumbsDown, ThumbsUp } from "lucide-react";
import { type Item } from "@/lib/api";
import { Thumbnail } from "@/components/thumbnail";
import { ScoreIndicator } from "@/components/score-indicator";
import { CheckStatusBadges } from "@/components/items/check-status-badges";

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
  const canToggleRead = item.status === "summarized";

  const reactionPill = item.is_favorite
    ? { icon: <Star className="size-3 fill-current" aria-hidden="true" />, label: t("items.feedback.favorite"), className: "border-amber-200 bg-amber-50 text-amber-700" }
    : item.feedback_rating === 1
      ? { icon: <ThumbsUp className="size-3" aria-hidden="true" />, label: t("items.feedback.like"), className: "border-green-200 bg-green-50 text-green-700" }
      : item.feedback_rating === -1
        ? { icon: <ThumbsDown className="size-3" aria-hidden="true" />, label: t("items.feedback.dislike"), className: "border-rose-200 bg-rose-50 text-rose-700" }
        : null;

  const style = animationDelay != null ? { animationDelay: `${animationDelay}ms` } : undefined;

  return (
    <div
      data-item-row-id={item.id}
      className="min-w-0 motion-safe:animate-fade-in-up"
      style={style}
    >
      <div
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
        className={[
          "group",
          featured
            ? "flex w-full flex-col gap-3 md:flex-row md:items-start"
            : "flex flex-col gap-2.5 sm:flex-row sm:items-stretch sm:gap-4",
          "rounded-xl px-3 py-3 sm:px-4 sm:py-4 transition-all duration-200",
          featured
            ? isRead
              ? "cursor-pointer border border-zinc-300 bg-zinc-200 hover:border-zinc-400"
              : "cursor-pointer border border-zinc-200 bg-white shadow-sm hover:border-zinc-300 hover:shadow-md"
            : isRead
              ? "cursor-pointer border border-zinc-300 bg-zinc-200 hover:border-zinc-400"
              : "cursor-pointer border border-zinc-200 bg-white shadow-sm hover:shadow-md hover:border-zinc-300",
        ].join(" ")}
      >
        {/* Inner content row */}
        <div
          className={`min-w-0 flex-1 transition-colors ${
            featured
              ? "flex min-w-0 flex-col gap-3 md:flex-row md:items-start"
              : "flex min-w-0 items-start gap-3"
          }`}
        >
          {/* Thumbnail */}
          <div
            className={`shrink-0 ${
              featured
                ? "flex h-36 w-full md:h-[104px] md:w-[136px] md:shrink-0"
                : "flex h-[88px] w-[88px] sm:h-[84px] sm:w-[84px]"
            }`}
          >
            <Thumbnail
              src={item.thumbnail_url}
              title={displayTitle ?? item.url}
              size={featured ? "lg" : "md"}
              className="w-full h-full"
            />
          </div>

          {/* Text */}
          <div
            className={`flex min-w-0 flex-1 flex-col ${
              featured ? "justify-start gap-2 py-0.5" : "justify-between gap-1.5 py-0.5"
            }`}
          >
            <div className={featured ? "space-y-2" : "space-y-2 sm:flex sm:items-start sm:gap-2 sm:space-y-0"}>
              <div className="min-w-0 flex-1">
                {featured && rank != null && rank > 0 && (
                  <div className="mb-1 inline-flex items-center gap-1 rounded-full bg-zinc-900 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-white">
                    PICK #{rank}
                  </div>
                )}
                <div
                  className={`overflow-hidden ${
                    featured
                      ? isRead
                        ? "line-clamp-3 text-base leading-6 text-zinc-400 font-medium"
                        : "line-clamp-3 text-[17px] leading-6 text-zinc-950 font-semibold"
                      : isRead
                        ? "line-clamp-3 text-[16px] leading-6 text-zinc-400 font-medium"
                        : "line-clamp-3 text-[16px] leading-6 text-zinc-900 font-semibold"
                  }`}
                >
                  {displayTitle ?? item.url}
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-zinc-400 sm:text-xs">
                  <span
                    className={`rounded-full border px-2 py-0.5 text-[11px] font-semibold ${
                      isRead
                        ? "border-zinc-300 bg-zinc-50 text-zinc-500"
                        : "border-zinc-200 bg-white text-zinc-700"
                    }`}
                  >
                    {isRead ? t("items.read.read") : t("items.read.unread")}
                  </span>
                  <span>{new Date(item.published_at ?? item.created_at).toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US")}</span>
                  {reactionPill && (
                    <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-semibold ${reactionPill.className}`}>
                      {reactionPill.icon}
                      {reactionPill.label}
                    </span>
                  )}
                  {!featured && (
                    <span className="sm:hidden">
                      <ScoreIndicator
                        score={item.summary_score}
                        personalScore={item.personal_score}
                        personalScoreReason={item.personal_score_reason}
                        locale={locale}
                        size="sm"
                      />
                    </span>
                  )}
                </div>
                <div className="mt-1">
                  <CheckStatusBadges
                    factsCheckResult={item.facts_check_result}
                    faithfulnessResult={item.faithfulness_result}
                    t={t}
                    compact
                  />
                </div>
              </div>
              {!featured && (
                <div className="hidden sm:block">
                  <ScoreIndicator
                    score={item.summary_score}
                    personalScore={item.personal_score}
                    personalScoreReason={item.personal_score_reason}
                    locale={locale}
                    size="sm"
                  />
                </div>
              )}
            </div>
            <div className={`hidden h-4 truncate text-[12px] sm:block ${featured ? "w-full text-zinc-500" : "text-zinc-400"}`}>
              {displayTitle ? item.url : "\u00A0"}
            </div>
          </div>
        </div>

        {/* Actions */}
        <div
          className={`flex shrink-0 gap-2 ${
            featured
              ? "flex-row self-start md:flex-col md:items-end"
              : "flex w-full flex-wrap items-center justify-start border-t border-zinc-100 pt-2 sm:w-auto sm:border-t-0 sm:pt-0 sm:max-w-[248px] sm:justify-end"
          }`}
        >
          {featured && (
            <div className="self-start md:self-auto">
              <ScoreIndicator score={item.summary_score} personalScore={item.personal_score} personalScoreReason={item.personal_score_reason} locale={locale} size="md" />
            </div>
          )}
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); onOpenDetail(); }}
            className={`inline-flex items-center justify-center gap-1 rounded-lg border border-zinc-300 bg-white px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 press focus-ring ${
              featured
                ? "h-9 md:min-w-[116px]"
                : canToggleRead || item.status === "failed"
                  ? "h-8 min-w-[calc(50%-0.25rem)] flex-1 sm:h-9 sm:min-w-[104px] sm:flex-none"
                  : "h-8 w-full sm:h-9 sm:min-w-[140px] sm:flex-none"
            }`}
          >
            <ExternalLink className="size-3.5" aria-hidden="true" />
            <span className="sm:hidden">{t("items.action.openShort")}</span>
            <span className="hidden sm:inline">{t("items.action.openDetail")}</span>
          </button>
          {canToggleRead && (
            <button
              type="button"
              disabled={readUpdating}
              onClick={(e) => { e.stopPropagation(); onToggleRead(); }}
              className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 press focus-ring ${
                isRead
                  ? "border border-zinc-300 bg-white text-zinc-700 hover:bg-zinc-50"
                  : "border border-zinc-900 bg-zinc-900 text-white hover:bg-zinc-800"
              } ${
                featured ? "h-9 md:min-w-[116px]" : "h-8 min-w-[calc(50%-0.25rem)] flex-1 sm:h-9 sm:min-w-[104px] sm:flex-none"
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
              onClick={(e) => { e.stopPropagation(); onRetry(); }}
              className={`rounded-lg border border-zinc-300 bg-white px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50 press focus-ring ${
                featured ? "h-9 md:min-w-[116px]" : "h-8 min-w-[calc(50%-0.25rem)] flex-1 sm:h-9 sm:min-w-[104px] sm:flex-none"
              }`}
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
  );
}
