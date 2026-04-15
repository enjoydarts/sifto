const UNTAGGED_ALIASES = new Set(["uncategorized", "untagged"]);

export const UNCATEGORIZED_GENRE_PARAM = "uncategorized";

export function normalizeStoredGenreValue(value) {
  const trimmed = (value ?? "").trim();
  if (!trimmed) return "";
  return UNTAGGED_ALIASES.has(trimmed.toLowerCase()) ? "" : trimmed;
}

export function normalizeGenreNavigationValue(value) {
  const trimmed = (value ?? "").trim();
  if (!trimmed) return "";
  return UNTAGGED_ALIASES.has(trimmed.toLowerCase()) ? UNCATEGORIZED_GENRE_PARAM : trimmed;
}

export function isUncategorizedGenreParam(value) {
  const trimmed = (value ?? "").trim().toLowerCase();
  return trimmed !== "" && UNTAGGED_ALIASES.has(trimmed);
}
