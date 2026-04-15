import { normalizeStoredGenreValue } from "./item-genre-shared.js";

export function patchGenreSuggestionsResponse(prev, {
  beforeEffectiveGenre,
  afterEffectiveGenre,
}) {
  if (!prev) return prev;

  const normalizedBefore = normalizeStoredGenreValue(beforeEffectiveGenre);
  const normalizedAfter = normalizeStoredGenreValue(afterEffectiveGenre);
  const nextCounts = [...(prev.genre_counts ?? [])].map((entry) => ({ ...entry }));

  const findIndex = (genreValue) =>
    nextCounts.findIndex((entry) => normalizeStoredGenreValue(entry.genre ?? entry.label ?? "") === genreValue);

  const ensureEntry = (genreValue) => {
    if (!genreValue) return;
    if (findIndex(genreValue) >= 0) return;
    nextCounts.unshift({ genre: genreValue, label: genreValue, count: 1 });
  };

  const adjustCount = (genreValue, delta) => {
    if (!genreValue || delta === 0) return;
    const index = findIndex(genreValue);
    if (index < 0) {
      if (delta > 0) {
        nextCounts.unshift({ genre: genreValue, label: genreValue, count: delta });
      }
      return;
    }
    const entry = nextCounts[index];
    const nextCount = Math.max(0, (entry.count ?? 0) + delta);
    if (nextCount === 0) {
      nextCounts.splice(index, 1);
      return;
    }
    nextCounts[index] = {
      ...entry,
      genre: genreValue,
      label: genreValue,
      count: nextCount,
    };
  };

  if (normalizedBefore && normalizedBefore !== normalizedAfter) {
    adjustCount(normalizedBefore, -1);
  }
  if (normalizedAfter && normalizedBefore !== normalizedAfter) {
    adjustCount(normalizedAfter, 1);
  } else if (normalizedAfter) {
    ensureEntry(normalizedAfter);
  }

  nextCounts.sort((a, b) => {
    if (b.count !== a.count) return b.count - a.count;
    return (a.genre ?? a.label ?? "").localeCompare(b.genre ?? b.label ?? "");
  });

  return {
    ...prev,
    genre_counts: nextCounts,
  };
}
