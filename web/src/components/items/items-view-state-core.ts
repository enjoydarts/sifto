import type { FeedMode, SortMode } from "./feed-tabs";

const FILTERS = ["", "summarized", "pending", "new", "fetched", "facts_extracted", "failed", "deleted"] as const;

export type ItemsViewState = {
  feedMode: FeedMode;
  sortMode: SortMode;
  filter: string;
  topic: string;
  sourceID: string;
  searchQuery: string;
  searchMode: "natural" | "and" | "or";
  unreadOnly: boolean;
  favoriteOnly: boolean;
  page: number;
};

export type ItemsViewStateAction =
  | { type: "hydrate_from_url"; state: ItemsViewState }
  | { type: "set_feed"; feed: FeedMode }
  | { type: "set_sort"; sort: SortMode }
  | { type: "set_filter"; filter: string }
  | { type: "set_topic"; topic: string }
  | { type: "set_source"; sourceID: string }
  | { type: "set_search"; searchQuery: string; searchMode: "natural" | "and" | "or" }
  | { type: "set_unread"; unreadOnly: boolean }
  | { type: "set_favorite"; favoriteOnly: boolean }
  | { type: "set_page"; page: number }
  | { type: "reset_filters" };

export function parseItemsQueryState(searchParams: URLSearchParams): ItemsViewState {
  const qFeed = searchParams.get("feed");
  const qFilter = searchParams.get("status");
  const deletedViaLegacyStatus = qFilter === "deleted";
  const feedMode: FeedMode =
    qFeed === "later"
      ? "later"
      : qFeed === "read"
        ? "read"
        : qFeed === "pending"
          ? "pending"
          : qFeed === "deleted"
            ? "deleted"
            : "unread";

  const qSort = searchParams.get("sort");
  const sortMode: SortMode = qSort === "score" ? "score" : qSort === "personal_score" ? "personal_score" : "newest";

  const filter =
    qFilter && FILTERS.includes(qFilter as (typeof FILTERS)[number]) && qFilter !== "deleted" ? qFilter : "";
  const topic = (searchParams.get("topic") ?? "").trim();
  const sourceID = (searchParams.get("source_id") ?? "").trim();
  const searchQuery = (searchParams.get("q") ?? "").trim();
  const qSearchMode = searchParams.get("search_mode");
  const searchMode: "natural" | "and" | "or" = qSearchMode === "and" ? "and" : qSearchMode === "or" ? "or" : "natural";

  const pendingFeed = qFeed === "pending";
  const unreadOnly = !pendingFeed && searchParams.get("unread") === "1";
  const favoriteOnly = !pendingFeed && searchParams.get("favorite") === "1";

  const qPage = Number(searchParams.get("page"));
  const page = Number.isFinite(qPage) && qPage >= 1 ? Math.floor(qPage) : 1;

  return normalizeItemsViewState({
    feedMode: deletedViaLegacyStatus ? "deleted" : feedMode,
    sortMode,
    filter,
    topic,
    sourceID,
    searchQuery,
    searchMode,
    unreadOnly,
    favoriteOnly,
    page,
  });
}

export function normalizeItemsViewState(state: ItemsViewState): ItemsViewState {
  const next = { ...state };
  if (next.feedMode === "read") {
    next.unreadOnly = false;
  }
  if (next.feedMode === "pending") {
    next.sortMode = "newest";
    next.unreadOnly = false;
    next.favoriteOnly = false;
  }
  if (next.feedMode === "deleted") {
    next.unreadOnly = false;
    next.favoriteOnly = false;
  }
  if (next.feedMode === "later") {
    next.unreadOnly = true;
  }
  if (!Number.isFinite(next.page) || next.page < 1) {
    next.page = 1;
  }
  return next;
}

export function buildItemsSearchParams(state: ItemsViewState): URLSearchParams {
  const normalized = normalizeItemsViewState(state);
  const q = new URLSearchParams();
  q.set("feed", normalized.feedMode);
  if (normalized.filter) q.set("status", normalized.filter);
  if (normalized.sourceID) q.set("source_id", normalized.sourceID);
  if (normalized.topic) q.set("topic", normalized.topic);
  if (normalized.searchQuery) q.set("q", normalized.searchQuery);
  if (normalized.searchQuery) q.set("search_mode", normalized.searchMode);
  q.set("sort", normalized.feedMode === "pending" ? "newest" : normalized.sortMode);
  if (normalized.unreadOnly) q.set("unread", "1");
  if (normalized.favoriteOnly) q.set("favorite", "1");
  if (normalized.page > 1) q.set("page", String(normalized.page));
  return q;
}

export function itemsViewStateReducer(state: ItemsViewState, action: ItemsViewStateAction): ItemsViewState {
  switch (action.type) {
    case "hydrate_from_url":
      return normalizeItemsViewState(action.state);
    case "set_feed":
      return normalizeItemsViewState({
        ...state,
        feedMode: action.feed,
        page: 1,
      });
    case "set_sort":
      return normalizeItemsViewState({
        ...state,
        sortMode: action.sort,
        page: 1,
      });
    case "set_filter":
      return normalizeItemsViewState({
        ...state,
        filter: action.filter,
        page: 1,
      });
    case "set_topic":
      return normalizeItemsViewState({
        ...state,
        topic: action.topic,
        page: 1,
      });
    case "set_source":
      return normalizeItemsViewState({
        ...state,
        sourceID: action.sourceID,
        page: 1,
      });
    case "set_search":
      return normalizeItemsViewState({
        ...state,
        searchQuery: action.searchQuery.trim(),
        searchMode: action.searchMode,
        page: 1,
      });
    case "set_unread":
      return normalizeItemsViewState({
        ...state,
        unreadOnly: action.unreadOnly,
        page: 1,
      });
    case "set_favorite":
      return normalizeItemsViewState({
        ...state,
        favoriteOnly: action.favoriteOnly,
        page: 1,
      });
    case "set_page":
      return normalizeItemsViewState({
        ...state,
        page: action.page,
      });
    case "reset_filters":
      return normalizeItemsViewState({
        ...state,
        filter: "",
        topic: "",
        sourceID: "",
        searchQuery: "",
        page: 1,
        favoriteOnly: false,
      });
    default:
      return state;
  }
}
