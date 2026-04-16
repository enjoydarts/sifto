"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useI18n } from "@/components/i18n-provider";
import {
  displayGenreLabel,
  getGenreOptions,
  normalizeOtherGenreLabel,
  normalizeStoredGenreValue,
  OTHER_GENRE_KEY,
} from "@/components/items/item-genre";

type GenreSuggestion = {
  value: string;
  count?: number;
};

type ItemGenreEditorProps = {
  genre?: string | null;
  genreOtherLabel?: string | null;
  userGenre?: string | null;
  userOtherGenreLabel?: string | null;
  summaryGenre?: string | null;
  summaryOtherGenreLabel?: string | null;
  suggestions: GenreSuggestion[];
  disabled?: boolean;
  onSave: (input: {
    userGenre: string | null;
    userOtherGenreLabel: string | null;
  }) => Promise<{
    genre?: string | null;
    other_genre_label?: string | null;
    user_genre?: string | null;
    user_other_genre_label?: string | null;
    summary_genre?: string | null;
    summary_other_genre_label?: string | null;
  } | void>;
};

type FeedbackState =
  | { tone: "success" | "error"; text: string }
  | null;

function resolveEffectiveOtherGenreLabel({
  genreKey,
  genreOtherLabel,
  userGenreKey,
  userOtherGenreLabel,
  summaryGenreKey,
  summaryOtherGenreLabel,
}: {
  genreKey: string;
  genreOtherLabel?: string | null;
  userGenreKey: string;
  userOtherGenreLabel?: string | null;
  summaryGenreKey: string;
  summaryOtherGenreLabel?: string | null;
}) {
  if (genreKey !== OTHER_GENRE_KEY) return "";
  if (userGenreKey === OTHER_GENRE_KEY) return normalizeOtherGenreLabel(userOtherGenreLabel);
  if (summaryGenreKey === OTHER_GENRE_KEY) return normalizeOtherGenreLabel(summaryOtherGenreLabel);
  return normalizeOtherGenreLabel(genreOtherLabel);
}

export function ItemGenreEditor({
  genre,
  genreOtherLabel,
  userGenre,
  userOtherGenreLabel,
  summaryGenre,
  summaryOtherGenreLabel,
  suggestions,
  disabled = false,
  onSave,
}: ItemGenreEditorProps) {
  const { t } = useI18n();
  const [draftGenre, setDraftGenre] = useState(userGenre ?? "");
  const [draftOtherGenreLabel, setDraftOtherGenreLabel] = useState(userOtherGenreLabel ?? "");
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [localGenre, setLocalGenre] = useState(genre ?? null);
  const [localGenreOtherLabel, setLocalGenreOtherLabel] = useState(genreOtherLabel ?? null);
  const [localUserGenre, setLocalUserGenre] = useState(userGenre ?? null);
  const [localUserOtherGenreLabel, setLocalUserOtherGenreLabel] = useState(userOtherGenreLabel ?? null);
  const [localSummaryGenre, setLocalSummaryGenre] = useState(summaryGenre ?? null);
  const [localSummaryOtherGenreLabel, setLocalSummaryOtherGenreLabel] = useState(summaryOtherGenreLabel ?? null);

  useEffect(() => {
    setDraftGenre(userGenre ?? "");
    setDraftOtherGenreLabel(userOtherGenreLabel ?? "");
    setLocalGenre(genre ?? null);
    setLocalGenreOtherLabel(genreOtherLabel ?? null);
    setLocalUserGenre(userGenre ?? null);
    setLocalUserOtherGenreLabel(userOtherGenreLabel ?? null);
    setLocalSummaryGenre(summaryGenre ?? null);
    setLocalSummaryOtherGenreLabel(summaryOtherGenreLabel ?? null);
  }, [genre, genreOtherLabel, summaryGenre, summaryOtherGenreLabel, userGenre, userOtherGenreLabel]);

  const normalizedLocalGenre = normalizeStoredGenreValue(localGenre);
  const normalizedLocalUserGenre = normalizeStoredGenreValue(localUserGenre);
  const normalizedLocalSummaryGenre = normalizeStoredGenreValue(localSummaryGenre);
  const currentRelevantOtherGenreLabel = normalizedLocalUserGenre === OTHER_GENRE_KEY
    ? normalizeOtherGenreLabel(localUserOtherGenreLabel)
    : "";
  const normalizedDraftGenre = normalizeStoredGenreValue(draftGenre);
  const normalizedDraftOtherGenreLabel = normalizeOtherGenreLabel(draftOtherGenreLabel);
  const draftRelevantOtherGenreLabel = normalizedDraftGenre === OTHER_GENRE_KEY ? normalizedDraftOtherGenreLabel : "";
  const saveDisabled =
    disabled ||
    saving ||
    (normalizedDraftGenre === OTHER_GENRE_KEY && draftRelevantOtherGenreLabel === "") ||
    (normalizedDraftGenre === normalizedLocalUserGenre && draftRelevantOtherGenreLabel === currentRelevantOtherGenreLabel);

  const genreOptions = useMemo(() => getGenreOptions(t), [t]);

  const effectiveGenreLabel = displayGenreLabel(localGenre, t, {
    otherLabel: resolveEffectiveOtherGenreLabel({
      genreKey: normalizedLocalGenre,
      genreOtherLabel: localGenreOtherLabel,
      userGenreKey: normalizedLocalUserGenre,
      userOtherGenreLabel: localUserOtherGenreLabel,
      summaryGenreKey: normalizedLocalSummaryGenre,
      summaryOtherGenreLabel: localSummaryOtherGenreLabel,
    }),
  });
  const manualGenreLabel = normalizedLocalUserGenre
    ? displayGenreLabel(localUserGenre, t, { otherLabel: localUserOtherGenreLabel })
    : "";
  const summaryGenreLabel = normalizedLocalSummaryGenre
    ? displayGenreLabel(localSummaryGenre, t, { otherLabel: localSummaryOtherGenreLabel })
    : "";

  const visibleSuggestions = useMemo(
    () => suggestions.filter((suggestion) => normalizeStoredGenreValue(suggestion.value) !== ""),
    [suggestions]
  );

  async function persistGenre(nextUserGenre: string | null, nextUserOtherGenreLabel: string | null) {
    setSaving(true);
    setFeedback(null);
    try {
      const requestedUserGenre = normalizeStoredGenreValue(nextUserGenre);
      const requestedUserOtherGenreLabel = requestedUserGenre === OTHER_GENRE_KEY
        ? normalizeOtherGenreLabel(nextUserOtherGenreLabel)
        : "";
      const next = await onSave({
        userGenre: requestedUserGenre || null,
        userOtherGenreLabel: requestedUserOtherGenreLabel || null,
      });
      const savedUserGenre = normalizeStoredGenreValue(next?.user_genre ?? requestedUserGenre);
      const savedUserOtherGenreLabel = savedUserGenre === OTHER_GENRE_KEY
        ? normalizeOtherGenreLabel(next?.user_other_genre_label ?? requestedUserOtherGenreLabel)
        : "";
      const savedSummaryGenre = normalizeStoredGenreValue(next?.summary_genre ?? localSummaryGenre);
      const savedSummaryOtherGenreLabel = savedSummaryGenre === OTHER_GENRE_KEY
        ? normalizeOtherGenreLabel(next?.summary_other_genre_label ?? localSummaryOtherGenreLabel)
        : "";
      const savedEffectiveGenre = normalizeStoredGenreValue(next?.genre ?? (savedUserGenre || savedSummaryGenre));
      const savedEffectiveOtherGenreLabel = savedEffectiveGenre === OTHER_GENRE_KEY
        ? normalizeOtherGenreLabel(
            next?.other_genre_label ??
              resolveEffectiveOtherGenreLabel({
                genreKey: savedEffectiveGenre,
                genreOtherLabel: localGenreOtherLabel,
                userGenreKey: savedUserGenre,
                userOtherGenreLabel: savedUserOtherGenreLabel,
                summaryGenreKey: savedSummaryGenre,
                summaryOtherGenreLabel: savedSummaryOtherGenreLabel,
              })
          )
        : "";
      setLocalUserGenre(savedUserGenre || null);
      setLocalUserOtherGenreLabel(savedUserOtherGenreLabel || null);
      setLocalSummaryGenre(savedSummaryGenre || null);
      setLocalSummaryOtherGenreLabel(savedSummaryOtherGenreLabel || null);
      setLocalGenre(savedEffectiveGenre || null);
      setLocalGenreOtherLabel(savedEffectiveOtherGenreLabel || null);
      setDraftGenre(savedUserGenre);
      setDraftOtherGenreLabel(savedUserOtherGenreLabel);
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
    await persistGenre(normalizedDraftGenre || null, draftRelevantOtherGenreLabel || null);
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
          {t("itemDetail.genre.selectLabel")}
        </span>
        <select
          value={draftGenre}
          onChange={(e) => {
            setDraftGenre(e.target.value);
            setFeedback(null);
          }}
          disabled={disabled || saving}
          className="mt-2 min-h-11 w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink)] placeholder:text-[var(--color-editorial-ink-faint)] focus-ring"
        >
          <option value="">{t("itemDetail.genre.manualEmpty")}</option>
          {genreOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </label>

      {normalizedDraftGenre === OTHER_GENRE_KEY ? (
        <label className="mt-4 block">
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {t("itemDetail.genre.otherInputLabel")}
          </span>
          <input
            value={draftOtherGenreLabel}
            onChange={(e) => {
              setDraftOtherGenreLabel(e.target.value);
              setFeedback(null);
            }}
            disabled={disabled || saving}
            placeholder={t("itemDetail.genre.otherPlaceholder")}
            className="mt-2 min-h-11 w-full rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink)] placeholder:text-[var(--color-editorial-ink-faint)] focus-ring"
          />
        </label>
      ) : null}

      {visibleSuggestions.length > 0 ? (
        <div className="mt-4">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {t("itemDetail.genre.suggestions")}
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            {visibleSuggestions.map((suggestion) => {
              const normalizedSuggestionValue = normalizeStoredGenreValue(suggestion.value);
              const active = normalizedDraftGenre === normalizedSuggestionValue;
              return (
                <button
                  key={suggestion.value}
                  type="button"
                  onClick={() => {
                    setDraftGenre(suggestion.value);
                    setFeedback(null);
                  }}
                  className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm transition press focus-ring ${
                    active
                      ? "border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  <span>{displayGenreLabel(suggestion.value, t)}</span>
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
            onClick={() => void persistGenre(null, null)}
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
