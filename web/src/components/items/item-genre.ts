"use client";

import type { ItemGenreCount } from "@/lib/api";
import {
  genreTaxonomyOrderIndex,
  isKnownGenreKey,
  ITEM_GENRE_KEYS,
  OTHER_GENRE_KEY,
  UNCATEGORIZED_GENRE_PARAM,
  isUncategorizedGenreParam,
  normalizeGenreNavigationValue,
  normalizeStoredGenreValue,
} from "./item-genre-shared.js";

export {
  ITEM_GENRE_KEYS,
  OTHER_GENRE_KEY,
  UNCATEGORIZED_GENRE_PARAM,
  genreTaxonomyOrderIndex,
  isKnownGenreKey,
  isUncategorizedGenreParam,
  normalizeGenreNavigationValue,
  normalizeStoredGenreValue,
};

function fallbackGenreLabel(value: string): string {
  return value
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

export function normalizeOtherGenreLabel(value: string | null | undefined): string {
  return (value ?? "").trim();
}

export function displayGenreLabel(
  value: string | null | undefined,
  translate: (key: string) => string,
  options?: { otherLabel?: string | null }
): string {
  const normalized = normalizeStoredGenreValue(value) || UNCATEGORIZED_GENRE_PARAM;
  if (normalized === OTHER_GENRE_KEY) {
    const otherLabel = normalizeOtherGenreLabel(options?.otherLabel);
    if (otherLabel) return otherLabel;
  }
  if (isKnownGenreKey(normalized)) {
    const translationKey = `items.genre.label.${normalized}`;
    const translated = translate(translationKey);
    return translated === translationKey ? fallbackGenreLabel(normalized) : translated;
  }
  return fallbackGenreLabel(normalized);
}

export function getGenreOptions(translate: (key: string) => string): Array<{ value: string; label: string }> {
  return ITEM_GENRE_KEYS.map((value) => ({
    value,
    label: displayGenreLabel(value, translate),
  }));
}

export function genreValueFromCountEntry(entry: ItemGenreCount): string {
  return normalizeStoredGenreValue(entry.genre?.trim() ? entry.genre : entry.label ?? "");
}

export function orderGenreCountEntries(entries: ItemGenreCount[]): ItemGenreCount[] {
  return [...entries].sort((a, b) => {
    const aValue = genreValueFromCountEntry(a);
    const bValue = genreValueFromCountEntry(b);
    const aOrder = genreTaxonomyOrderIndex(aValue);
    const bOrder = genreTaxonomyOrderIndex(bValue);
    if (Number.isFinite(aOrder) || Number.isFinite(bOrder)) {
      if (!Number.isFinite(aOrder)) return 1;
      if (!Number.isFinite(bOrder)) return -1;
      if (aOrder !== bOrder) return aOrder - bOrder;
    }
    if ((b.count ?? 0) !== (a.count ?? 0)) return (b.count ?? 0) - (a.count ?? 0);
    return aValue.localeCompare(bValue);
  });
}
