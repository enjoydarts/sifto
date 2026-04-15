"use client";

import type { ItemGenreCount } from "@/lib/api";
import {
  UNCATEGORIZED_GENRE_PARAM,
  isUncategorizedGenreParam,
  normalizeGenreNavigationValue,
  normalizeStoredGenreValue,
} from "./item-genre-shared.js";

export {
  UNCATEGORIZED_GENRE_PARAM,
  isUncategorizedGenreParam,
  normalizeGenreNavigationValue,
  normalizeStoredGenreValue,
};

export function displayGenreLabel(value: string | null | undefined, uncategorizedLabel: string): string {
  return normalizeStoredGenreValue(value) || uncategorizedLabel;
}

export function genreValueFromCountEntry(entry: ItemGenreCount): string {
  return normalizeStoredGenreValue(entry.genre ?? entry.label ?? "");
}

export function orderGenreCountEntries(entries: ItemGenreCount[]): ItemGenreCount[] {
  const regular = entries.filter((entry) => genreValueFromCountEntry(entry));
  const uncategorized = entries.filter((entry) => !genreValueFromCountEntry(entry));
  return [...regular, ...uncategorized];
}
