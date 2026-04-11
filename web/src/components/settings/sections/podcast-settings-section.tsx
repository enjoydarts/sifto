"use client";

import type { FormEvent } from "react";
import type { Translate } from "@/components/settings/sections/audio-briefing-settings-types";
import { SectionCard } from "@/components/ui/section-card";
import type { PodcastCategoryOption } from "@/lib/api";

export default function PodcastSettingsSection({
  t,
  form,
  state,
  actions,
}: {
  t: Translate;
  form: {
    onSubmit: (event: FormEvent<HTMLFormElement>) => void;
    saving: boolean;
  };
  state: {
    enabled: boolean;
    rssURL: string;
    feedSlug: string;
    language: string;
    category: string;
    subcategory: string;
    availableCategories: PodcastCategoryOption[];
    selectedCategory: PodcastCategoryOption | null;
    title: string;
    author: string;
    description: string;
    artworkURL: string;
    uploadingArtwork: boolean;
    explicit: boolean;
  };
  actions: {
    onChangeEnabled: (value: boolean) => void;
    onCopyRSSURL: () => void;
    onChangeLanguage: (value: string) => void;
    onChangeCategory: (value: string) => void;
    onChangeSubcategory: (value: string) => void;
    onChangeTitle: (value: string) => void;
    onChangeAuthor: (value: string) => void;
    onChangeDescription: (value: string) => void;
    onChangeArtworkURL: (value: string) => void;
    onUploadArtwork: (file: File | null) => void;
    onUseDefaultArtwork: () => void;
    onChangeExplicit: (value: boolean) => void;
  };
}) {
  const { onSubmit, saving } = form;
  const {
    enabled,
    rssURL,
    feedSlug,
    language,
    category,
    subcategory,
    availableCategories,
    selectedCategory,
    title,
    author,
    description,
    artworkURL,
    uploadingArtwork,
    explicit,
  } = state;
  const {
    onChangeEnabled,
    onCopyRSSURL,
    onChangeLanguage,
    onChangeCategory,
    onChangeSubcategory,
    onChangeTitle,
    onChangeAuthor,
    onChangeDescription,
    onChangeArtworkURL,
    onUploadArtwork,
    onUseDefaultArtwork,
    onChangeExplicit,
  } = actions;
  return (
    <SectionCard>
      <form onSubmit={onSubmit} className="space-y-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.podcast.title")}</div>
            <p className="mt-1 max-w-3xl text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.podcast.description")}</p>
          </div>
          <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
            <button
              type="submit"
              disabled={saving}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
            >
              {saving ? t("common.saving") : t("settings.podcast.save")}
            </button>
            <button
              type="button"
              disabled={!rssURL}
              onClick={onCopyRSSURL}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
            >
              {t("settings.podcast.copyRSS")}
            </button>
          </div>
        </div>

        <div className="grid gap-3 lg:grid-cols-2">
          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("settings.podcast.enabled")}
                </div>
                <p className="mt-2 text-sm text-[var(--color-editorial-ink)]">{enabled ? t("settings.on") : t("settings.off")}</p>
              </div>
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => onChangeEnabled(e.target.checked)}
                className="size-4 rounded border-[var(--color-editorial-line-strong)]"
              />
            </div>
          </label>

          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.rssUrl")}
            </div>
            <div className="mt-3 break-all rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]">
              {rssURL || t("settings.podcast.rssUrlPending")}
            </div>
          </div>

          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.feedSlug")}
            </div>
            <input
              value={feedSlug}
              readOnly
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
            />
          </label>

          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.language")}
            </div>
            <select
              value={language}
              onChange={(e) => onChangeLanguage(e.target.value)}
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
            >
              <option value="ja">ja</option>
              <option value="en">en</option>
            </select>
          </label>

          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.category")}
            </div>
            <select
              value={category}
              onChange={(e) => onChangeCategory(e.target.value)}
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
            >
              <option value="">{t("settings.podcast.categoryUnset")}</option>
              {availableCategories.map((option) => (
                <option key={option.category} value={option.category}>
                  {option.category}
                </option>
              ))}
            </select>
          </label>

          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.subcategory")}
            </div>
            <select
              value={subcategory}
              onChange={(e) => onChangeSubcategory(e.target.value)}
              disabled={!selectedCategory || selectedCategory.subcategories.length === 0}
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)] disabled:opacity-60"
            >
              <option value="">{t("settings.podcast.subcategoryUnset")}</option>
              {(selectedCategory?.subcategories ?? []).map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>
        </div>

        <div className="grid gap-3 lg:grid-cols-2">
          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.showTitle")}
            </div>
            <input
              value={title}
              onChange={(e) => onChangeTitle(e.target.value)}
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
            />
          </label>

          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.author")}
            </div>
            <input
              value={author}
              onChange={(e) => onChangeAuthor(e.target.value)}
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
            />
          </label>
        </div>

        <label className="block rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            {t("settings.podcast.summary")}
          </div>
          <textarea
            value={description}
            onChange={(e) => onChangeDescription(e.target.value)}
            rows={5}
            className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
          />
        </label>

        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.artworkUrl")}
            </div>
            <input
              value={artworkURL}
              onChange={(e) => onChangeArtworkURL(e.target.value)}
              className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
            />
            <div className="mt-3 flex flex-wrap gap-2">
              <label className="inline-flex min-h-10 cursor-pointer items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)]">
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  className="hidden"
                  onChange={(e) => onUploadArtwork(e.target.files?.[0] ?? null)}
                />
                {uploadingArtwork ? t("common.saving") : t("settings.podcast.uploadArtwork")}
              </label>
              <button
                type="button"
                onClick={onUseDefaultArtwork}
                className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)]"
              >
                {t("settings.podcast.useDefaultArtwork")}
              </button>
            </div>
          </div>

          <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.podcast.explicit")}
            </div>
            <div className="mt-3 flex items-center justify-between gap-3">
              <div className="text-sm text-[var(--color-editorial-ink)]">{explicit ? t("settings.podcast.explicitYes") : t("settings.podcast.explicitNo")}</div>
              <input
                type="checkbox"
                checked={explicit}
                onChange={(e) => onChangeExplicit(e.target.checked)}
                className="size-4 rounded border-[var(--color-editorial-line-strong)]"
              />
            </div>
          </label>
        </div>

        <p className="text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
          {t("settings.podcast.help")}
        </p>
      </form>
    </SectionCard>
  );
}
