"use client";

import { Suspense, useCallback } from "react";
import { CheckCheck, Newspaper, Search, Volume2, X } from "lucide-react";
import { InlineReader } from "@/components/inline-reader";
import { PageTransition } from "@/components/page-transition";
import { FiltersBar } from "@/components/items/filters-bar";
import { ItemCard } from "@/components/items/item-card";
import { FeedTabs } from "@/components/items/feed-tabs";
import { ItemsSummaryStrip } from "@/components/items/items-summary-strip";
import { ItemsListState } from "@/components/items/items-list-state";
import { DenseArticleList } from "@/components/items/dense-article-list";
import { PageHeader } from "@/components/ui/page-header";
import { FilterBar } from "@/components/ui/filter-bar";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";
import { SkeletonList } from "@/components/ui/skeleton-list";
import { queryKeys } from "@/lib/query-keys";
import { useItemsPageData, type ItemsFeedQueryData } from "./use-items-page-data";

function ItemsPageContent() {
  const {
    t, locale,
    setFeed, setSort, setFilter, setTopic, setSource, setSearch, setFavorite, setPage, resetFilters,
    feedMode, sortMode, filter, topic, sourceID, searchQuery, searchMode, unreadOnly, favoriteOnly, page,
    unreadMode, readMode, pendingMode, deletedMode,
    summaryAudioPlaybackBlocked,
    setError,
    inlineItemId, setInlineItemId,
    retryingIds,
    readUpdatingIds,
    feedbackUpdatingIds,
    bulkMarkingRead,
    bulkRetrying,
    bulkRetryingFromFacts,
    bulkDeleting,
    toolbarAction, setToolbarAction,
    pendingBulkAction, setPendingBulkAction,
    searchOpen, setSearchOpen,
    searchDraft, setSearchDraft,
    searchModeDraft, setSearchModeDraft,
    activeSuggestionIndex, setActiveSuggestionIndex,
    inlineQueueItemIds, setInlineQueueItemIds,
    listQueryKey,
    items, itemsTotal, searchUnavailable,
    loading, visibleError,
    suggestions,
    suggestionsLoading,
    submitSearch,
    visibleSearchValue,
    applySuggestion,
    showFilterBadges,
    summaryAudioViewQuery,
    sortedItems,
    dateSections,
    inlineItemStatus,
    summaryMetrics,
    pageSubtitleKey,
    detailHref,
    rememberScroll,
    saveReadQueue,
    prefetchItemDetail,
    updateItemFeedback,
    retryItem,
    toggleRead,
    bulkMarkRead,
    selectedItemIDSet,
    visibleSelectedCount,
    toggleSelectedItem,
    selectAllVisibleItems,
    clearSelectedItems,
    bulkRetryFromFacts,
    bulkRetry,
    bulkDelete,
    queryClient,
    router,
  } = useItemsPageData();

  const renderItem = useCallback((item: Parameters<typeof ItemCard>[0]["item"], opts?: { featured?: boolean; rank?: number; animIdx?: number }) => {
    const featured = Boolean(opts?.featured);
    const href = detailHref(item.id);
    const openDetail = () => {
      rememberScroll(item.id);
      saveReadQueue(sortedItems.map((v) => v.id));
      router.push(href);
    };
    const openInlineReader = () => {
      setInlineQueueItemIds(sortedItems.map((v) => v.id));
      setInlineItemId(item.id);
      prefetchItemDetail(item.id);
    };
    return (
      <ItemCard
        key={item.id}
        item={item}
        featured={featured}
        rank={opts?.rank}
        locale={locale}
        selectable={pendingMode}
        selected={selectedItemIDSet.has(item.id)}
        readUpdating={!!readUpdatingIds[item.id]}
        feedbackUpdating={!!feedbackUpdatingIds[item.id]}
        retrying={!!retryingIds[item.id]}
        onOpen={openInlineReader}
        onOpenDetail={openDetail}
        onToggleSelected={() => toggleSelectedItem(item.id)}
        onToggleRead={() => void toggleRead(item)}
        onToggleLike={() => void updateItemFeedback(item, { rating: item.feedback_rating === 1 ? 0 : 1 })}
        onToggleDislike={() => void updateItemFeedback(item, { rating: item.feedback_rating === -1 ? 0 : -1 })}
        onRetry={() => void retryItem(item.id)}
        onPrefetch={() => prefetchItemDetail(item.id)}
        animationDelay={(opts?.animIdx ?? 0) * 40}
        t={t}
      />
    );
  }, [detailHref, feedbackUpdatingIds, locale, pendingMode, prefetchItemDetail, readUpdatingIds, rememberScroll, retryItem, retryingIds, router, saveReadQueue, selectedItemIDSet, sortedItems, t, toggleRead, toggleSelectedItem, updateItemFeedback, setInlineItemId, setInlineQueueItemIds]);

  const railFilterTags = [
    topic ? (
      <Tag key="topic" tone="accent" removable onRemove={() => setTopic("")}>
        {t("items.topic")}: {topic}
      </Tag>
    ) : null,
    sourceID ? (
      <Tag
        key="source"
        tone="accent"
        removable
        removeLabel={t("common.clear")}
        onRemove={() => setSource("")}
      >
        {t("items.filter.sourceApplied")}
      </Tag>
    ) : null,
    searchQuery ? (
      <Tag key="search" tone="success" removable onRemove={() => setSearch("", searchMode)}>
        {t("items.search.active")}: {searchQuery}
      </Tag>
    ) : null,
    filter && filter !== "pending" && filter !== "deleted" ? (
      <Tag
        key="status"
        tone="accent"
        removable
        removeLabel={t("common.clear")}
        onRemove={() => setFilter("")}
      >
        {t(`items.filter.${filter}`)}
      </Tag>
    ) : null,
  ].filter(Boolean);

  return (
    <PageTransition>
      <div className="space-y-3 pb-8">
        <div className="grid gap-3 xl:grid-cols-[248px_minmax(0,1fr)] xl:items-start">
          <aside className="hidden xl:sticky xl:top-[6.25rem] xl:flex xl:self-start xl:flex-col xl:gap-4">
            <SectionCard compact className="overflow-hidden">
              <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                {t("items.rail.metrics")}
              </div>
              <div className="mt-3 divide-y divide-[var(--color-editorial-line)]">
                {summaryMetrics.map((metric) => (
                  <div key={metric.key} className="grid gap-1 py-3 first:pt-0 last:pb-0">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {metric.label}
                    </div>
                    <div className="text-[2rem] leading-none tracking-[-0.04em] text-[var(--color-editorial-ink)]">
                      {metric.value}
                    </div>
                    <div className="text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                      {metric.hint}
                    </div>
                  </div>
                ))}
              </div>
            </SectionCard>

            <SectionCard compact>
              <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                {t("items.rail.actions")}
              </div>
              <div className="mt-3 grid gap-2">
                {!pendingMode && (
                  <button
                    type="button"
                    onClick={() => router.push("/triage?mode=all")}
                    className="inline-flex min-h-10 items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
                  >
                    <CheckCheck className="size-4" aria-hidden="true" />
                    <span>{t("items.openAllTriage")}</span>
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => {
                    setSearchDraft(searchQuery);
                    setSearchModeDraft(searchMode);
                    setSearchOpen(true);
                  }}
                  className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full border px-3 py-2 text-sm font-medium press focus-ring ${
                    searchQuery
                      ? "border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  <Search className="size-4" aria-hidden="true" />
                  <span>{t("items.search.open")}</span>
                </button>
                {!pendingMode && !deletedMode && (
                  <button
                    type="button"
                    onClick={() => {
                      if (summaryAudioPlaybackBlocked) {
                        return;
                      }
                      router.push(`/audio-player?queue=view&view=${encodeURIComponent(summaryAudioViewQuery)}`);
                    }}
                    disabled={summaryAudioPlaybackBlocked}
                    className="inline-flex min-h-10 items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:cursor-not-allowed disabled:opacity-50 press focus-ring"
                  >
                    <Volume2 className="size-4" aria-hidden="true" />
                    <span>{t("items.openAudioPlayer")}</span>
                  </button>
                )}
              </div>
            </SectionCard>

            {showFilterBadges && (
              <SectionCard compact>
                <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  {t("items.rail.filters")}
                </div>
                <div className="mt-3 flex flex-wrap gap-2">{railFilterTags}</div>
              </SectionCard>
            )}
          </aside>

          <div className="min-w-0 space-y-3">
            <PageHeader
              compact
              className="overflow-hidden"
              eyebrow={t("items.state.eyebrow")}
              title={t("nav.items")}
              titleIcon={Newspaper}
              description={`${t(pageSubtitleKey)} · ${itemsTotal.toLocaleString()} ${t("common.rows")}`}
              meta={(
                <div className="inline-flex items-center gap-2 text-[11px] font-medium uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  <Newspaper className="size-3.5" aria-hidden="true" />
                  <span>{t("items.state.meta")}</span>
                </div>
              )}
              actions={(
                <div className="flex w-full flex-wrap items-center justify-end gap-2 xl:hidden">
                  <button
                    type="button"
                    onClick={() => {
                      setSearchDraft(searchQuery);
                      setSearchModeDraft(searchMode);
                      setSearchOpen(true);
                    }}
                    className={`inline-flex min-h-9 items-center justify-center rounded-full border px-3 py-1.5 text-sm font-medium press focus-ring ${
                      searchQuery
                        ? "border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    }`}
                    aria-label={t("items.search.open")}
                  >
                    <Search className="size-4" aria-hidden="true" />
                  </button>
                  {!pendingMode && (
                    <button
                      type="button"
                      onClick={() => router.push("/triage?mode=all")}
                      className="inline-flex min-h-9 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-3.5 py-1.5 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
                    >
                      <CheckCheck className="size-4" aria-hidden="true" />
                      <span>{t("items.openAllTriage")}</span>
                    </button>
                  )}
                  {!pendingMode && !deletedMode && (
                    <button
                      type="button"
                      onClick={() => {
                        if (summaryAudioPlaybackBlocked) {
                          return;
                        }
                        router.push(`/audio-player?queue=view&view=${encodeURIComponent(summaryAudioViewQuery)}`);
                      }}
                      disabled={summaryAudioPlaybackBlocked}
                      className="inline-flex min-h-9 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3.5 py-1.5 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50 press focus-ring"
                    >
                      <Volume2 className="size-4" aria-hidden="true" />
                      <span>{t("items.openAudioPlayer")}</span>
                    </button>
                  )}
                </div>
              )}
            />

            <div className="hidden xl:hidden">
              <ItemsSummaryStrip metrics={summaryMetrics} />
            </div>

            <FilterBar
              compact
              leading={(
                <div className="xl:w-[550px] xl:max-w-full xl:flex-none">
                  <FeedTabs
                    feedMode={feedMode}
                    onSelect={(feed) => {
                      setFeed(feed);
                    }}
                    t={t}
                  />
                </div>
              )}
              filters={(
                <FiltersBar
                  feedMode={feedMode}
                  sortMode={sortMode}
                  favoriteOnly={favoriteOnly}
                  toolbarAction={toolbarAction}
                  bulkMarkingRead={bulkMarkingRead}
                  onSortChange={setSort}
                  onFavoriteChange={setFavorite}
                  onToolbarActionChange={setToolbarAction}
                  onToolbarRun={() => {
                    if (toolbarAction === "bulk_filtered") {
                      void bulkMarkRead("filtered");
                      return;
                    }
                    if (toolbarAction === "bulk_older") {
                      void bulkMarkRead("older_than_7d");
                    }
                  }}
                  t={t}
                />
              )}
              actions={
                pendingMode ? (
                  <div className="flex w-full flex-wrap items-center justify-end gap-1.5 xl:w-[372px] xl:flex-none xl:flex-nowrap xl:gap-1">
                    <button
                      type="button"
                      onClick={selectAllVisibleItems}
                      disabled={items.length === 0}
                      className="inline-flex min-h-10 items-center justify-center whitespace-nowrap rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:cursor-not-allowed disabled:opacity-50 xl:px-2.5"
                    >
                      {t("items.bulkRetryFromFacts.selectAll")}
                    </button>
                    <button
                      type="button"
                      onClick={clearSelectedItems}
                      disabled={visibleSelectedCount === 0}
                      className="inline-flex min-h-10 items-center justify-center whitespace-nowrap rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:cursor-not-allowed disabled:opacity-50 xl:px-2.5"
                    >
                      <span className="xl:hidden">{t("items.bulkRetryFromFacts.clearSelection")}</span>
                      <span className="hidden xl:inline">{t("items.bulkRetryFromFacts.clearSelectionShort")}</span>
                    </button>
                    <div className="inline-flex min-h-10 items-center whitespace-nowrap rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] xl:px-2.5">
                      <span className="xl:hidden">
                        {t("items.bulkRetryFromFacts.selectedCount").replace("{{count}}", String(visibleSelectedCount))}
                      </span>
                      <span className="hidden xl:inline">
                        {t("items.bulkRetryFromFacts.selectedCountShort").replace("{{count}}", String(visibleSelectedCount))}
                      </span>
                    </div>
                    <select
                      value={pendingBulkAction}
                      onChange={(e) => setPendingBulkAction(e.target.value as typeof pendingBulkAction)}
                      className="min-h-10 min-w-0 flex-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink-soft)] focus-ring xl:w-[188px] xl:flex-none xl:px-2.5"
                      aria-label={t("items.pendingActions.label")}
                    >
                      <option value="">{t("items.pendingActions.placeholder")}</option>
                      <option value="retry">{t("items.bulkRetry.run")}</option>
                      <option value="retry_from_facts">{t("items.bulkRetryFromFacts.run")}</option>
                      <option value="delete">{t("items.bulkDelete.run")}</option>
                    </select>
                    <button
                      type="button"
                      disabled={
                        visibleSelectedCount === 0 ||
                        !pendingBulkAction ||
                        bulkRetrying ||
                        bulkRetryingFromFacts ||
                        bulkDeleting
                      }
                      onClick={() => {
                        if (pendingBulkAction === "retry") {
                          void bulkRetry();
                          return;
                        }
                        if (pendingBulkAction === "retry_from_facts") {
                          void bulkRetryFromFacts();
                          return;
                        }
                        if (pendingBulkAction === "delete") {
                          void bulkDelete();
                        }
                      }}
                      className="inline-flex min-h-10 items-center justify-center whitespace-nowrap rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50 xl:px-2.5"
                    >
                      {bulkRetrying || bulkRetryingFromFacts || bulkDeleting ? t("common.saving") : (
                        <>
                          <span className="xl:hidden">{t("items.actions.run")}</span>
                          <span className="hidden xl:inline">{t("items.actions.run")}</span>
                        </>
                      )}
                    </button>
                  </div>
                ) : (
                  <div
                    aria-hidden={readMode}
                    className={`hidden w-full flex-wrap items-center justify-end gap-2 xl:flex xl:w-auto xl:flex-nowrap ${
                      readMode ? "pointer-events-none invisible" : ""
                    }`}
                  >
                      <select
                        value={toolbarAction}
                        onChange={(e) => setToolbarAction(e.target.value as typeof toolbarAction)}
                        className="min-h-10 min-w-0 flex-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3.5 py-2 text-sm text-[var(--color-editorial-ink-soft)] focus-ring xl:w-[188px] xl:flex-none"
                        aria-label={t("items.toolbar.actions")}
                      >
                        <option value="">{t("items.actions.placeholder")}</option>
                        <option value="bulk_filtered">{t("items.bulkRead.filtered")}</option>
                        <option value="bulk_older">{t("items.bulkRead.olderThan7d")}</option>
                      </select>
                      <button
                        type="button"
                        disabled={!toolbarAction || bulkMarkingRead}
                        onClick={() => {
                          if (toolbarAction === "bulk_filtered") {
                            void bulkMarkRead("filtered");
                            return;
                          }
                          if (toolbarAction === "bulk_older") {
                            void bulkMarkRead("older_than_7d");
                          }
                        }}
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {bulkMarkingRead ? t("common.saving") : t("items.actions.run")}
                      </button>
                    </div>
                )
              }
            >
              {showFilterBadges ? <div className="flex flex-wrap items-center gap-2">{railFilterTags}</div> : null}
            </FilterBar>

            {searchQuery && searchUnavailable ? (
              <SectionCard className="border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {t("items.search.unavailable")}
              </SectionCard>
            ) : null}

            {loading || items.length === 0 || !!visibleError ? (
              <ItemsListState
                loading={loading}
                error={visibleError}
                isSearchActive={Boolean(searchQuery)}
                hasFilters={Boolean(topic || sourceID || (filter && filter !== "pending"))}
                isPendingMode={pendingMode}
                onRetry={() => {
                  setError(null);
                  void queryClient.invalidateQueries({ queryKey: listQueryKey });
                }}
                onResetFilters={resetFilters}
                t={t}
              />
            ) : (
              <DenseArticleList
                sections={dateSections.map((section) => ({
                  date: section.date,
                  items: section.items.map((item, idx) => renderItem(item, { animIdx: idx })),
                }))}
                total={itemsTotal}
                page={page}
                pageSize={20}
                onPageChange={setPage}
              />
            )}
          </div>
        </div>

        {inlineItemId && (
          <InlineReader
            open={!!inlineItemId}
            itemId={inlineItemId}
            locale={locale}
            queueItemIds={inlineQueueItemIds}
            itemStatus={inlineItemStatus}
            onClose={() => {
              setInlineItemId(null);
              setInlineQueueItemIds([]);
            }}
            onOpenDetail={(itemId) => {
              setInlineItemId(null);
              rememberScroll(itemId);
              saveReadQueue(sortedItems.map((v) => v.id));
              router.push(detailHref(itemId));
            }}
            onOpenItem={(itemId) => setInlineItemId(itemId)}
            onReadToggled={(itemId, isRead) => {
              queryClient.setQueryData<ItemsFeedQueryData>(listQueryKey, (prev) =>
                prev
                  ? {
                      ...prev,
                      items: prev.items
                        .map((v) => (v.id === itemId ? { ...v, is_read: isRead } : v))
                        .filter((v) => {
                          if (unreadMode || unreadOnly) return !v.is_read;
                          if (readMode) return v.is_read;
                          return true;
                        }),
                      total:
                        (unreadMode || unreadOnly || readMode) && prev.items.some((v) => v.id === itemId)
                          ? Math.max(
                              0,
                              prev.total -
                                (unreadMode || unreadOnly
                                  ? isRead
                                    ? 1
                                    : 0
                                  : readMode
                                    ? isRead
                                      ? 0
                                      : 1
                                    : 0)
                            )
                          : prev.total,
                    }
                  : prev
              );
              void queryClient.invalidateQueries({ queryKey: ["items-feed"] });
              void queryClient.invalidateQueries({ queryKey: ["focus-queue"] });
              void queryClient.invalidateQueries({ queryKey: queryKeys.briefing.todayPrefix });
            }}
            onFeedbackUpdated={() => {
              void queryClient.invalidateQueries({ queryKey: ["items-feed"] });
              void queryClient.invalidateQueries({ queryKey: ["focus-queue"] });
            }}
          />
        )}

        {searchOpen && (
          <div
            className="fixed inset-0 z-50 flex items-start justify-center bg-zinc-950/45 px-4 py-10 sm:items-center"
            onClick={() => setSearchOpen(false)}
          >
            <div
              className="w-full max-w-lg rounded-2xl border border-zinc-200 bg-white p-5 shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <h2 className="text-lg font-semibold text-zinc-900">{t("items.search.title")}</h2>
                  <p className="text-sm text-zinc-500">{t("items.search.description")}</p>
                </div>
                <button
                  type="button"
                  onClick={() => setSearchOpen(false)}
                  className="inline-flex size-9 items-center justify-center rounded-lg border border-zinc-200 bg-white text-zinc-500 hover:bg-zinc-100 hover:text-zinc-700 press focus-ring"
                  aria-label={t("common.close")}
                >
                  <X className="size-4" aria-hidden="true" />
                </button>
              </div>

              <div className="mt-4 space-y-3">
                <input
                  autoFocus
                  type="search"
                  value={visibleSearchValue}
                  onChange={(e) => {
                    setSearchDraft(e.target.value);
                    setActiveSuggestionIndex(-1);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "ArrowDown" && suggestions.length > 0) {
                      e.preventDefault();
                      setActiveSuggestionIndex((prev) => {
                        if (prev < 0) return 0;
                        return (prev + 1) % suggestions.length;
                      });
                      return;
                    }
                    if (e.key === "ArrowUp" && suggestions.length > 0) {
                      e.preventDefault();
                      setActiveSuggestionIndex((prev) => {
                        if (prev < 0) return suggestions.length - 1;
                        return (prev - 1 + suggestions.length) % suggestions.length;
                      });
                      return;
                    }
                    if (e.key === "Enter") {
                      e.preventDefault();
                      if (activeSuggestionIndex >= 0 && suggestions[activeSuggestionIndex]) {
                        applySuggestion(suggestions[activeSuggestionIndex]);
                        return;
                      }
                      submitSearch();
                    }
                    if (e.key === "Escape") {
                      e.preventDefault();
                      setSearchOpen(false);
                    }
                  }}
                  placeholder={t("items.search.placeholder")}
                  className="min-h-11 w-full rounded-xl border border-zinc-200 bg-white px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus-ring"
                />
                <div className="rounded-xl border border-zinc-200 bg-zinc-50/80 p-2">
                  {!searchOpen || searchDraft.trim().length < 2 ? (
                    <div className="px-2 py-1 text-xs text-zinc-500">{t("items.search.suggestions.helper")}</div>
                  ) : suggestionsLoading ? (
                    <div className="px-2 py-1 text-xs text-zinc-500">{t("common.loading")}</div>
                  ) : suggestions.length === 0 ? (
                    <div className="px-2 py-1 text-xs text-zinc-500">{t("items.search.suggestions.empty")}</div>
                  ) : (
                    <div className="space-y-1">
                      {suggestions.map((suggestion, index) => {
                        const active = index === activeSuggestionIndex;
                        return (
                          <button
                            key={`${suggestion.kind}:${suggestion.source_id ?? suggestion.topic ?? suggestion.item_id ?? suggestion.label}:${index}`}
                            type="button"
                            onClick={() => applySuggestion(suggestion)}
                            className={`block w-full rounded-lg px-3 py-2 text-left text-sm transition ${
                              active ? "bg-zinc-900 text-white" : "bg-white text-zinc-900 hover:bg-zinc-100"
                            }`}
                          >
                            <span className="block truncate font-medium">{suggestion.label}</span>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
                <div className="space-y-2">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
                    {t("items.search.modeLabel")}
                  </div>
                  <div className="grid grid-cols-3 gap-2">
                    {(["natural", "and", "or"] as const).map((mode) => {
                      const active = searchModeDraft === mode;
                      const labelKey =
                        mode === "natural"
                          ? "items.search.mode.natural"
                          : mode === "and"
                            ? "items.search.mode.and"
                            : "items.search.mode.or";
                      return (
                        <button
                          key={mode}
                          type="button"
                          onClick={() => setSearchModeDraft(mode)}
                          className={`rounded-xl border px-3 py-2 text-sm font-medium transition ${
                            active
                              ? "border-zinc-900 bg-zinc-900 text-white"
                              : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50"
                          }`}
                        >
                          {t(labelKey)}
                        </button>
                      );
                    })}
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <button
                    type="button"
                    onClick={() => {
                      setSearchDraft("");
                      setActiveSuggestionIndex(-1);
                    }}
                    className="text-sm font-medium text-zinc-500 hover:text-zinc-700 press"
                  >
                    {t("common.clear")}
                  </button>
                  <button
                    type="button"
                    onClick={submitSearch}
                    className="inline-flex min-h-10 items-center rounded-lg border border-zinc-900 bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 press focus-ring"
                  >
                    {t("items.search.submit")}
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </PageTransition>
  );
}

export default function ItemsPage() {
  return (
    <Suspense
      fallback={
        <SkeletonList rows={8} />
      }
    >
      <ItemsPageContent />
    </Suspense>
  );
}
