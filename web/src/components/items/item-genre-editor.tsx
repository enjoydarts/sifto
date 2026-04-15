"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useI18n } from "@/components/i18n-provider";
import { displayGenreLabel, normalizeStoredGenreValue } from "@/components/items/item-genre";

type GenreSuggestion = {
  value: string;
  count?: number;
};

type ItemGenreEditorProps = {
  genre?: string | null;
  userGenre?: string | null;
  summaryGenre?: string | null;
  suggestions: GenreSuggestion[];
  disabled?: boolean;
  onSave: (userGenre: string | null) => Promise<{ genre?: string | null; user_genre?: string | null; summary_genre?: string | null } | void>;
};

type FeedbackState =
  | { tone: "success" | "error"; text: string }
  | null;

export function ItemGenreEditor({
  genre,
  userGenre,
  summaryGenre,
  suggestions,
  disabled = false,
  onSave,
}: ItemGenreEditorProps) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(userGenre ?? "");
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [localGenre, setLocalGenre] = useState(genre ?? null);
  const [localUserGenre, setLocalUserGenre] = useState(userGenre ?? null);
  const [localSummaryGenre, setLocalSummaryGenre] = useState(summaryGenre ?? null);

  useEffect(() => {
    setDraft(userGenre ?? "");
    setLocalGenre(genre ?? null);
    setLocalUserGenre(userGenre ?? null);
    setLocalSummaryGenre(summaryGenre ?? null);
  }, [genre, summaryGenre, userGenre]);

  const effectiveGenreLabel = displayGenreLabel(localGenre, t("items.genre.uncategorized"));
  const manualGenreLabel = normalizeStoredGenreValue(localUserGenre);
  const summaryGenreLabel = normalizeStoredGenreValue(localSummaryGenre);
  const normalizedDraft = normalizeStoredGenreValue(draft);
  const saveDisabled = disabled || saving || normalizedDraft === normalizeStoredGenreValue(localUserGenre);

  const visibleSuggestions = useMemo(
    () => suggestions.filter((suggestion) => normalizeStoredGenreValue(suggestion.value) !== ""),
    [suggestions]
  );

  async function persistGenre(nextUserGenre: string | null) {
    setSaving(true);
    setFeedback(null);
    try {
      const next = await onSave(nextUserGenre);
      const savedUserGenre = normalizeStoredGenreValue(next?.user_genre ?? nextUserGenre);
      const savedSummaryGenre = normalizeStoredGenreValue(next?.summary_genre ?? localSummaryGenre);
      const savedEffectiveGenre = normalizeStoredGenreValue(next?.genre ?? (savedUserGenre || savedSummaryGenre));
      setLocalUserGenre(savedUserGenre || null);
      setLocalSummaryGenre(savedSummaryGenre || null);
      setLocalGenre(savedEffectiveGenre || null);
      setDraft(savedUserGenre);
      setFeedback({
        tone: "success",
        text: savedUserGenre ? t("itemDetail.genre.saved") : t("itemDetail.genre.cleared"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        text: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setSaving(false);
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    await persistGenre(normalizedDraft || null);
  }

  return (
    <form
      onSubmit={submit}
      className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(245,240,233,0.78),rgba(255,255,255,0.92))] p-4"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("itemDetail.genre.title")}</h2>
          <p className="mt-1 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{t("itemDetail.genre.description")}</p>
        </div>
        <button
          type="submit"
          disabled={saveDisabled}
          className="rounded-full bg-[var(--color-editorial-ink)] px-3 py-2 text-xs font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
        >
          {saving ? t("common.saving") : t("itemDetail.genre.submit")}
        </button>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {t("itemDetail.genre.current")}
          </div>
          <div className="mt-2 text-sm font-medium text-[var(--color-editorial-ink)]">{effectiveGenreLabel}</div>
        </div>
        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {t("itemDetail.genre.manual")}
          </div>
          <div className="mt-2 text-sm font-medium text-[var(--color-editorial-ink)]">
            {manualGenreLabel || t("itemDetail.genre.manualEmpty")}
          </div>
        </div>
        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {t("itemDetail.genre.auto")}
          </div>
          <div className="mt-2 text-sm font-medium text-[var(--color-editorial-ink)]">
            {summaryGenreLabel || t("itemDetail.genre.autoEmpty")}
          </div>
        </div>
      </div>

      <label className="mt-4 block">
        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
          {t("itemDetail.genre.inputLabel")}
        </span>
        <input
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value);
            setFeedback(null);
          }}
          disabled={disabled}
          placeholder={t("itemDetail.genre.placeholder")}
          className="mt-2 min-h-11 w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink)] placeholder:text-[var(--color-editorial-ink-faint)] focus-ring"
        />
      </label>

      {visibleSuggestions.length > 0 ? (
        <div className="mt-4">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {t("itemDetail.genre.suggestions")}
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            {visibleSuggestions.map((suggestion) => {
              const active = normalizeStoredGenreValue(draft) === normalizeStoredGenreValue(suggestion.value);
              return (
                <button
                  key={suggestion.value}
                  type="button"
                  onClick={() => {
                    setDraft(suggestion.value);
                    setFeedback(null);
                  }}
                  className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm transition press focus-ring ${
                    active
                      ? "border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  <span>{suggestion.value}</span>
                  {typeof suggestion.count === "number" ? (
                    <span className="text-[11px] text-[var(--color-editorial-ink-faint)]">{suggestion.count}</span>
                  ) : null}
                </button>
              );
            })}
          </div>
        </div>
      ) : null}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        {manualGenreLabel ? (
          <button
            type="button"
            disabled={disabled || saving}
            onClick={() => void persistGenre(null)}
            className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-60"
          >
            {t("itemDetail.genre.clear")}
          </button>
        ) : null}
      </div>

      <div
        aria-live="polite"
        className={`mt-3 min-h-5 text-xs ${
          feedback?.tone === "error"
            ? "text-[var(--color-editorial-error)]"
            : "text-[var(--color-editorial-ink-soft)]"
        }`}
      >
        {feedback?.text ?? ""}
      </div>
    </form>
  );
}
