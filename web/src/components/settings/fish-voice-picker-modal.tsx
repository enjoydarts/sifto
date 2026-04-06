"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { Search, X } from "lucide-react";
import { FishBrowseSort, FishBrowseResponse, FishModelSnapshot, api } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { MarkdownText } from "@/components/ui/markdown-text";

const FISH_BROWSE_TABS: FishBrowseSort[] = ["recommended", "trending", "latest"];

function authorName(model: FishModelSnapshot): string {
  return model.author?.name?.trim() || model.author?.username?.trim() || "—";
}

function formatLanguages(model: FishModelSnapshot): string {
  return model.languages.length > 0 ? model.languages.join(", ") : "—";
}

function formatCompactNumber(value: number): string {
  return new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

function topTags(model: FishModelSnapshot) {
  return model.tags
    .map((tag) => tag?.name?.trim() || "")
    .filter(Boolean)
    .slice(0, 3);
}

export default function FishVoicePickerModal({
  open,
  currentVoiceModel,
  onClose,
  onSelect,
}: {
  open: boolean;
  currentVoiceModel: string;
  onClose: () => void;
  onSelect: (selection: { voice_model: string; provider_voice_label: string; provider_voice_description: string }) => void;
}) {
  const { t } = useI18n();
  const [tab, setTab] = useState<FishBrowseSort>("recommended");
  const [queryInput, setQueryInput] = useState("");
  const [query, setQuery] = useState("");
  const [items, setItems] = useState<FishModelSnapshot[]>([]);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loadingInitial, setLoadingInitial] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedModelID, setSelectedModelID] = useState<string | null>(null);
  const requestSeqRef = useRef(0);

  useEffect(() => {
    if (!open) return;
    const timer = window.setTimeout(() => {
      setQuery(queryInput.trim());
    }, 250);
    return () => window.clearTimeout(timer);
  }, [open, queryInput]);

  useEffect(() => {
    if (!open) return;
    const req = ++requestSeqRef.current;
    setLoadingInitial(true);
    setError(null);
    setPage(1);
    api
      .browseFishModels({ sort: tab, query, page: 1, pageSize: 24 })
      .then((resp: FishBrowseResponse) => {
        if (req !== requestSeqRef.current) return;
        setItems(resp.items);
        setHasMore(resp.has_more);
        setPage(resp.page);
      })
      .catch((err) => {
        if (req !== requestSeqRef.current) return;
        setItems([]);
        setHasMore(false);
        setError(String(err));
      })
      .finally(() => {
        if (req === requestSeqRef.current) {
          setLoadingInitial(false);
        }
      });
  }, [open, query, tab]);

  useEffect(() => {
    if (!open) return;
    if (selectedModelID) return;
    if (currentVoiceModel) {
      setSelectedModelID(currentVoiceModel);
    }
  }, [currentVoiceModel, open, selectedModelID]);

  const selectedModel = useMemo(
    () => items.find((item) => item._id === (selectedModelID ?? currentVoiceModel)) ?? null,
    [currentVoiceModel, items, selectedModelID]
  );

  const currentSelectionPinned = useMemo(() => {
    if (!currentVoiceModel.trim()) return null;
    if (items.some((item) => item._id === currentVoiceModel)) return null;
    return {
      _id: currentVoiceModel,
      title: currentVoiceModel,
      description: t("fishModels.picker.currentSelectionFallback"),
    };
  }, [currentVoiceModel, items, t]);

  async function handleLoadMore() {
    if (loadingMore || loadingInitial || !hasMore) return;
    const nextPage = page + 1;
    const req = ++requestSeqRef.current;
    setLoadingMore(true);
    setError(null);
    try {
      const resp = await api.browseFishModels({ sort: tab, query, page: nextPage, pageSize: 24 });
      if (req !== requestSeqRef.current) return;
      setItems((prev) => {
        const seen = new Set(prev.map((item) => item._id));
        const merged = [...prev];
        for (const item of resp.items) {
          if (seen.has(item._id)) continue;
          seen.add(item._id);
          merged.push(item);
        }
        return merged;
      });
      setHasMore(resp.has_more);
      setPage(resp.page);
    } catch (err) {
      if (req !== requestSeqRef.current) return;
      setError(String(err));
    } finally {
      if (req === requestSeqRef.current) {
        setLoadingMore(false);
      }
    }
  }

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("fishModels.picker.title")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("fishModels.picker.subtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Link
              href="/fish-models"
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("fishModels.picker.openAdmin")}
            </Link>
            <button
              type="button"
              onClick={onClose}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("common.close")}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="border-b border-[var(--color-editorial-line)] px-5 py-4">
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
          <div className="mt-3 flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
            <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
            <input
              value={queryInput}
              onChange={(e) => setQueryInput(e.target.value)}
              placeholder={t("fishModels.search")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,0.95fr)_minmax(340px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] px-4 py-4 lg:border-b-0 lg:border-r">
            {currentSelectionPinned ? (
              <button
                type="button"
                onClick={() => setSelectedModelID(currentSelectionPinned._id)}
                className={`mb-3 flex w-full items-start justify-between gap-3 rounded-[20px] border px-4 py-3 text-left ${
                  selectedModelID === currentSelectionPinned._id
                    ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-panel)]"
                    : "border-[var(--color-editorial-line)] bg-white"
                }`}
              >
                <div className="min-w-0">
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.picker.currentSelection")}</div>
                  <div className="mt-1 break-all text-sm font-semibold text-[var(--color-editorial-ink)]">{currentSelectionPinned.title}</div>
                  <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{currentSelectionPinned.description}</div>
                </div>
              </button>
            ) : null}

            <div className="space-y-3">
              {loadingInitial ? (
                Array.from({ length: 5 }).map((_, index) => (
                  <div key={`fish-picker-skeleton-${index}`} className="rounded-[20px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="h-4 w-40 rounded bg-[var(--color-editorial-panel-strong)]" />
                    <div className="mt-3 h-3 w-24 rounded bg-[var(--color-editorial-panel-strong)]" />
                  </div>
                ))
              ) : items.length === 0 ? (
                <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                  {query ? t("fishModels.emptySearch") : t("fishModels.picker.noResults")}
                </div>
              ) : (
                items.map((model) => {
                  const tags = topTags(model);
                  const active = selectedModel?._id === model._id || (!selectedModel && currentVoiceModel === model._id);
                  return (
                    <button
                      key={model._id}
                      type="button"
                      onClick={() => setSelectedModelID(model._id)}
                      className={`flex w-full items-start justify-between gap-3 rounded-[20px] border px-4 py-4 text-left transition ${
                        active
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-panel)]"
                          : "border-[var(--color-editorial-line)] bg-white hover:bg-[var(--color-editorial-panel)]"
                      }`}
                    >
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-semibold text-[var(--color-editorial-ink)]">{model.title}</div>
                        <div className="mt-1 truncate text-xs text-[var(--color-editorial-ink-soft)]">{authorName(model)}</div>
                        <div className="mt-3 flex flex-wrap gap-2">
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
                      <div className="shrink-0 text-right text-xs text-[var(--color-editorial-ink-faint)]">
                        <div>{formatCompactNumber(model.task_count)} {t("fishModels.table.uses").toLowerCase()}</div>
                        <div className="mt-1">{formatCompactNumber(model.like_count)} {t("fishModels.table.likes").toLowerCase()}</div>
                      </div>
                    </button>
                  );
                })
              )}
            </div>

            {hasMore ? (
              <div className="mt-4">
                <button
                  type="button"
                  onClick={() => void handleLoadMore()}
                  disabled={loadingMore}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                >
                  {loadingMore ? t("common.loading") : t("fishModels.loadMore")}
                </button>
              </div>
            ) : null}
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedModel ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.picker.selected")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedModel.title}</h3>
                  <div className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {selectedModel.description ? <MarkdownText content={selectedModel.description} /> : t("fishModels.noDescription")}
                  </div>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.table.author")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{authorName(selectedModel)}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.table.languages")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{formatLanguages(selectedModel)}</div>
                  </div>
                </div>

                <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.modal.tags")}</div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {selectedModel.tags.some((tag) => tag?.name?.trim()) ? (
                      selectedModel.tags
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
                </div>

                {selectedModel.samples.length > 0 ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("fishModels.modal.samples")}</div>
                    <div className="mt-3 space-y-3">
                      {selectedModel.samples.slice(0, 2).map((sample, index) => (
                        <div key={`${selectedModel._id}-sample-${index}`} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                          <audio controls preload="none" className="w-full" src={sample.audio_url} />
                          <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                            {sample.text || sample.transcript || t("fishModels.modal.noSampleTranscript")}
                          </p>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({
                        voice_model: selectedModel._id,
                        provider_voice_label: selectedModel.title,
                        provider_voice_description: selectedModel.description,
                      });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("fishModels.picker.select")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("fishModels.picker.emptySelection")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
