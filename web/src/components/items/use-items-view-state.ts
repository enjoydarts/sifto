"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import {
  buildItemsSearchParams,
  itemsViewStateReducer,
  parseItemsQueryState,
} from "./items-view-state-core";

export { buildItemsSearchParams, itemsViewStateReducer, normalizeItemsViewState, parseItemsQueryState, type ItemsViewState } from "./items-view-state-core";

export function useItemsViewState() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const searchParamsString = searchParams.toString();
  const urlState = useMemo(
    () => parseItemsQueryState(new URLSearchParams(searchParamsString)),
    [searchParamsString]
  );
  const urlQuery = useMemo(() => buildItemsSearchParams(urlState).toString(), [urlState]);

  const [state, dispatch] = useReducer(
    itemsViewStateReducer,
    searchParamsString,
    (query) => parseItemsQueryState(new URLSearchParams(query))
  );
  const [intendedQuery, setIntendedQuery] = useState<string | null>(null);
  const pendingUrlQueryRef = useRef<string | null>(null);
  const externalUrlQueryRef = useRef<string | null>(null);

  const stateQuery = useMemo(() => buildItemsSearchParams(state).toString(), [state]);
  const localStateAheadOfURL = intendedQuery === stateQuery;
  const renderState = localStateAheadOfURL || urlQuery === stateQuery ? state : urlState;
  const renderStateQuery = buildItemsSearchParams(renderState).toString();

  useEffect(() => {
    if (pendingUrlQueryRef.current === urlQuery) {
      pendingUrlQueryRef.current = null;
      externalUrlQueryRef.current = null;
      window.setTimeout(() => setIntendedQuery(null), 0);
      return;
    }
    if (urlQuery === stateQuery) {
      if (externalUrlQueryRef.current === urlQuery) {
        externalUrlQueryRef.current = null;
      }
      if (intendedQuery === urlQuery) {
        window.setTimeout(() => setIntendedQuery(null), 0);
      }
      return;
    }
    if (localStateAheadOfURL) {
      return;
    }
    externalUrlQueryRef.current = urlQuery;
    dispatch({ type: "hydrate_from_url", state: urlState });
  }, [intendedQuery, localStateAheadOfURL, stateQuery, urlQuery, urlState]);

  useEffect(() => {
    const nextQuery = stateQuery;
    const currentQuery =
      typeof window === "undefined"
        ? searchParamsString
        : window.location.search.replace(/^\?/, "");
    if (nextQuery === currentQuery) {
      if (pendingUrlQueryRef.current === currentQuery) {
        pendingUrlQueryRef.current = null;
      }
      return;
    }
    if (externalUrlQueryRef.current === currentQuery) {
      return;
    }
    const nextUrl = nextQuery ? `${pathname}?${nextQuery}` : pathname;
    pendingUrlQueryRef.current = nextQuery;
    router.replace(nextUrl, { scroll: false });
  }, [pathname, router, searchParamsString, stateQuery]);

  const currentItemsHref = useMemo(() => {
    const query = renderStateQuery;
    return query ? `${pathname}?${query}` : pathname;
  }, [pathname, renderStateQuery]);

  const dispatchAction = useCallback((action: Parameters<typeof itemsViewStateReducer>[1]) => {
    const nextState = itemsViewStateReducer(state, action);
    setIntendedQuery(buildItemsSearchParams(nextState).toString());
    dispatch(action);
  }, [state]);

  const setFeed = useCallback((feed: import("./feed-tabs").FeedMode) => dispatchAction({ type: "set_feed", feed }), [dispatchAction]);
  const setSort = useCallback((sort: import("./feed-tabs").SortMode) => dispatchAction({ type: "set_sort", sort }), [dispatchAction]);
  const setFilter = useCallback((filter: string) => dispatchAction({ type: "set_filter", filter }), [dispatchAction]);
  const setGenre = useCallback((genre: string) => dispatchAction({ type: "set_genre", genre }), [dispatchAction]);
  const setTopic = useCallback((topic: string) => dispatchAction({ type: "set_topic", topic }), [dispatchAction]);
  const setSource = useCallback((sourceID: string) => dispatchAction({ type: "set_source", sourceID }), [dispatchAction]);
  const setSearch = useCallback(
    (searchQuery: string, searchMode: "natural" | "and" | "or") =>
      dispatchAction({ type: "set_search", searchQuery, searchMode }),
    [dispatchAction]
  );
  const setUnread = useCallback((unreadOnly: boolean) => dispatchAction({ type: "set_unread", unreadOnly }), [dispatchAction]);
  const setFavorite = useCallback((favoriteOnly: boolean) => dispatchAction({ type: "set_favorite", favoriteOnly }), [dispatchAction]);
  const setPage = useCallback((page: number) => dispatchAction({ type: "set_page", page }), [dispatchAction]);
  const resetFilters = useCallback(() => dispatchAction({ type: "reset_filters" }), [dispatchAction]);

  return {
    state: renderState,
    dispatch: dispatchAction,
    currentItemsHref,
    setFeed,
    setSort,
    setFilter,
    setGenre,
    setTopic,
    setSource,
    setSearch,
    setUnread,
    setFavorite,
    setPage,
    resetFilters,
  };
}
