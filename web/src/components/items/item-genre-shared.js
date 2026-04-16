const UNTAGGED_ALIASES = new Set(["uncategorized", "untagged"]);

export const ITEM_GENRE_KEYS = [
  "ai",
  "devtools",
  "security",
  "cloud",
  "data",
  "infra",
  "web",
  "mobile",
  "robotics",
  "semiconductor",
  "research",
  "product",
  "business",
  "funding",
  "regulation",
  "design",
  "uncategorized",
  "other",
];

const KNOWN_GENRE_KEY_SET = new Set(ITEM_GENRE_KEYS);
const GENRE_TAXONOMY_ORDER = new Map(ITEM_GENRE_KEYS.map((key, index) => [key, index]));

export const UNCATEGORIZED_GENRE_PARAM = "uncategorized";
export const OTHER_GENRE_KEY = "other";

export function normalizeStoredGenreValue(value) {
  const trimmed = (value ?? "").trim();
  if (!trimmed) return "";
  const lower = trimmed.toLowerCase();
  if (UNTAGGED_ALIASES.has(lower)) return UNCATEGORIZED_GENRE_PARAM;
  if (lower === "agent") return "ai";
  if (KNOWN_GENRE_KEY_SET.has(lower)) return lower;
  return OTHER_GENRE_KEY;
}

export function normalizeGenreNavigationValue(value) {
  const normalized = normalizeStoredGenreValue(value);
  return normalized === "" ? "" : normalized;
}

export function isUncategorizedGenreParam(value) {
  return normalizeGenreNavigationValue(value) === UNCATEGORIZED_GENRE_PARAM;
}

export function isKnownGenreKey(value) {
  return KNOWN_GENRE_KEY_SET.has(normalizeStoredGenreValue(value));
}

export function genreTaxonomyOrderIndex(value) {
  const normalized = normalizeStoredGenreValue(value);
  return GENRE_TAXONOMY_ORDER.get(normalized) ?? Number.POSITIVE_INFINITY;
}
