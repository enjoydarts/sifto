"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Search, X } from "lucide-react";
import { FishBrowseResponse, FishBrowseSort, FishModelSnapshot, api } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { MarkdownText } from "@/components/ui/markdown-text";

const FISH_BROWSE_TABS: FishBrowseSort[] = ["recommended", "trending", "latest"];

function authorName(model: FishModelSnapshot) {
  return model.author?.name?.trim() || model.author?.username?.trim() || "—";
}

function formatCompactNumber(value: number) {
  return new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

function topTags(model: FishModelSnapshot) {
  return model.tags
    .map((tag) => tag?.name?.trim() || "")
    .filter(Boolean)
    .slice(0, 3);
}

export default function FishModelsPage() {
  const { t } = useI18n();
  const [tab, setTab] = useState<FishBrowseSort>("recommended");
  const [queryInput, setQueryInput] = useState("");
  const [query, setQuery] = useState("");
  const [items, setItems] = useState<FishModelSnapshot[]>([]);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingInitial, setLoadingInitial] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedModel, setSelectedModel] = useState<FishModelSnapshot | null>(null);
  const requestSeqRef = useRef(0);
  const loadMoreRef = useRef<HTMLDivElement | null>(null);

  const loadBrowse = useCallback(
    async (mode: "replace" | "append", nextPage: number, nextTab: FishBrowseSort, nextQuery: string) => {
      const requestID = ++requestSeqRef.current;
      if (mode === "replace") {
        setLoadingInitial(true);
        setError(null);
      } else {
        setLoadingMore(true);
      }
      try {
        const resp: FishBrowseResponse = await api.browseFishModels({
          sort: nextTab,
          query: nextQuery,
          page: nextPage,
          pageSize: 24,
        });
        if (requestID !== requestSeqRef.current) return;
        setItems((prev) => {
          if (mode === "replace") return resp.items;
          const seen = new Set(prev.map((item) => item._id));
          const merged = [...prev];
          for (const item of resp.items) {
            if (seen.has(item._id)) continue;
            seen.add(item._id);
            merged.push(item);
          }
          return merged;
        });
        setPage(resp.page);
        setHasMore(resp.has_more);
      } catch (err) {
        if (requestID !== requestSeqRef.current) return;
        if (mode === "replace") {
          setItems([]);
          setHasMore(false);
        }
        setError(String(err));
      } finally {
        if (requestID === requestSeqRef.current) {
          setLoadingInitial(false);
          setLoadingMore(false);
        }
      }
    },
    []
  );

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setQuery(queryInput.trim());
    }, 250);
    return () => window.clearTimeout(timer);
  }, [queryInput]);

  useEffect(() => {
    void loadBrowse("replace", 1, tab, query);
  }, [loadBrowse, query, tab]);

  async function handleLoadMore() {
    if (loadingInitial || loadingMore || !hasMore) return;
    await loadBrowse("append", page + 1, tab, query);
  }

  useEffect(() => {
    const target = loadMoreRef.current;
    if (!target || !hasMore || loadingInitial || loadingMore) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries[0]?.isIntersecting) return;
        void handleLoadMore();
      },
      {
        rootMargin: "320px 0px",
      }
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [hasMore, loadingInitial, loadingMore, page, tab, query]);

  const selectedModelTags = useMemo(() => selectedModel?.tags ?? [], [selectedModel]);

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader
          title={t("fishModels.title")}
          description={t("fishModels.subtitle")}
        />

        <section className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5 shadow-[0_18px_48px_rgba(35,24,12,0.08)]">
            <div className="flex flex-wrap items-center gap-2">
              {FISH_BROWSE_TABS.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => setTab(item)}
                  className={`rounded-full px-4 py-2 text-sm font-medium transition ${
                    tab === item
                      ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                      : "border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)]"
                  }`}
                >
                  {t(`fishModels.tabs.${item}`)}
                </button>
              ))}
            </div>

            <p className="mt-3 text-sm text-[var(--color-editorial-ink-soft)]">{t(`fishModels.tabs.${tab}Description`)}</p>

            <div className="mt-5 flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
              <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
              <input
                value={queryInput}
                onChange={(e) => setQueryInput(e.target.value)}
                placeholder={t("fishModels.search")}
                className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
              />
            </div>

            {query ? <p className="mt-3 text-sm text-[var(--color-editorial-ink-soft)]">{t("fishModels.searchResults").replace("{{query}}", query)}</p> : null}
            {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}

            <div className="mt-5 space-y-3">
              {loadingInitial ? (
                <div className="rounded-[22px] bg-white/70 px-5 py-6">
                  {Array.from({ length: 4 }).map((_, index) => (
                    <div key={`fish-page-skeleton-${index}`} className={index === 0 ? "" : "mt-6"}>
                      <div className="h-4 w-40 animate-pulse rounded-full bg-[var(--color-editorial-panel-strong)]" />
                      <div className="mt-3 h-3 w-24 animate-pulse rounded-full bg-[var(--color-editorial-panel-strong)]" />
                      <div className="mt-4 space-y-2">
                        <div className="h-3 w-full animate-pulse rounded-full bg-[var(--color-editorial-panel-strong)]" />
                        <div className="h-3 w-5/6 animate-pulse rounded-full bg-[var(--color-editorial-panel-strong)]" />
                      </div>
                    </div>
                  ))}
                  <p className="mt-6 text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>
                </div>
              ) : items.length === 0 ? (
                <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                  {query ? t("fishModels.emptySearch") : t("fishModels.noModels")}
                </div>
              ) : (
                <>
                  {items.map((model) => {
                    const tags = topTags(model);
                    return (
                      <button
                        key={model._id}
                        type="button"
                        onClick={() => setSelectedModel(model)}
                        className="flex w-full items-start justify-between gap-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-white px-5 py-4 text-left transition hover:bg-[var(--color-editorial-panel)]"
                      >
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-base font-semibold text-[var(--color-editorial-ink)]">{model.title}</div>
                          <div className="mt-2 text-sm text-[var(--color-editorial-ink-soft)]">{authorName(model)}</div>
                          <div className="mt-3 line-clamp-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                            {model.description ? <MarkdownText content={model.description} /> : t("fishModels.noDescription")}
                          </div>
                          <div className="mt-4 flex flex-wrap gap-2">
                            {tags.map((tag) => (
                              <span
                                key={`${model._id}-${tag}`}
                                className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[11px] text-[var(--color-editorial-ink-soft)]"
                              >
                                {tag}
                              </span>
                            ))}
                          </div>
                        </div>
                        <div className="shrink-0 text-right">
                          <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{formatCompactNumber(model.task_count)}</div>
                          <div className="text-xs text-[var(--color-editorial-ink-faint)]">{t("fishModels.table.uses")}</div>
                          <div className="mt-3 text-sm font-semibold text-[var(--color-editorial-ink)]">{formatCompactNumber(model.like_count)}</div>
                          <div className="text-xs text-[var(--color-editorial-ink-faint)]">{t("fishModels.table.likes")}</div>
                        </div>
                      </button>
                    );
                  })}

                  <div ref={loadMoreRef} className="h-1 w-full" aria-hidden="true" />

                  {loadingMore ? (
                    <div className="rounded-[18px] bg-white/70 px-4 py-3 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                      {t("common.loading")}
                    </div>
                  ) : null}

                  {!hasMore ? (
                    <div className="px-2 pt-2 text-center text-xs text-[var(--color-editorial-ink-faint)]">
                      {t("fishModels.endOfList")}
                    </div>
                  ) : null}
                </>
              )}
            </div>
        </section>

        {selectedModel ? (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={() => setSelectedModel(null)}>
            <div
              className="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
              onClick={(event) => event.stopPropagation()}
            >
              <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
                <div>
                  <h2 className="text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedModel.title}</h2>
                </div>
                <button
                  type="button"
                  onClick={() => setSelectedModel(null)}
                  className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                  aria-label={t("common.close")}
                >
                  <X className="size-4" />
                </button>
              </div>

              <div className="grid min-h-0 flex-1 gap-5 overflow-auto px-5 py-5 lg:grid-cols-[minmax(0,1fr)_minmax(300px,0.9fr)]">
                <div className="space-y-5">
                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.modal.description")}</div>
                    <div className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                      {selectedModel.description ? <MarkdownText content={selectedModel.description} /> : t("fishModels.noDescription")}
                    </div>
                  </section>

                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.modal.tags")}</div>
                    <div className="mt-3 flex flex-wrap gap-2">
                      {selectedModelTags.some((tag) => tag?.name?.trim()) ? (
                        selectedModelTags
                          .filter((tag) => tag?.name?.trim())
                          .map((tag) => (
                          <span
                            key={`${selectedModel._id}-${tag.name}`}
                            className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                          >
                            {tag.name}
                          </span>
                          ))
                      ) : (
                        <span className="text-sm text-[var(--color-editorial-ink-soft)]">{t("fishModels.modal.noTags")}</span>
                      )}
                    </div>
                  </section>

                  {selectedModel.samples.length > 0 ? (
                    <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                      <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.modal.samples")}</div>
                      <div className="mt-3 space-y-3">
                        {selectedModel.samples.map((sample, index) => (
                          <div key={`${selectedModel._id}-sample-${index}`} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                            <audio controls preload="none" className="w-full" src={sample.audio_url} />
                            <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                              {sample.text || sample.transcript || t("fishModels.modal.noSampleTranscript")}
                            </p>
                          </div>
                        ))}
                      </div>
                    </section>
                  ) : null}
                </div>

                <div className="space-y-5">
                  <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.modal.meta")}</div>
                    <dl className="mt-3 space-y-3 text-sm text-[var(--color-editorial-ink-soft)]">
                      <div className="flex items-start justify-between gap-3">
                        <dt>{t("fishModels.table.author")}</dt>
                        <dd className="text-right text-[var(--color-editorial-ink)]">{authorName(selectedModel)}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3">
                        <dt>{t("fishModels.table.languages")}</dt>
                        <dd className="text-right text-[var(--color-editorial-ink)]">{selectedModel.languages.join(", ") || "—"}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3">
                        <dt>{t("fishModels.table.likes")}</dt>
                        <dd className="text-right text-[var(--color-editorial-ink)]">{formatCompactNumber(selectedModel.like_count)}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3">
                        <dt>{t("fishModels.table.uses")}</dt>
                        <dd className="text-right text-[var(--color-editorial-ink)]">{formatCompactNumber(selectedModel.task_count)}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3">
                        <dt>{t("fishModels.modal.visibility")}</dt>
                        <dd className="text-right text-[var(--color-editorial-ink)]">{selectedModel.visibility || "—"}</dd>
                      </div>
                      <div className="flex items-start justify-between gap-3">
                        <dt>{t("fishModels.modal.fetchedAt")}</dt>
                        <dd className="text-right text-[var(--color-editorial-ink)]">{new Date(selectedModel.fetched_at).toLocaleString()}</dd>
                      </div>
                    </dl>
                  </section>
                </div>
              </div>
            </div>
          </div>
        ) : null}
      </div>
    </PageTransition>
  );
}
