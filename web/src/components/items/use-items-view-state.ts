"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef } from "react";
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

  const initialState = useMemo(
    () => parseItemsQueryState(new URLSearchParams(searchParamsString)),
    []
  );
  const [state, dispatch] = useReducer(itemsViewStateReducer, initialState);
  const stateRef = useRef(initialState);
  const stateQueryRef = useRef(buildItemsSearchParams(initialState).toString());
  const intendedQueryRef = useRef<string | null>(null);
  const pendingUrlQueryRef = useRef<string | null>(null);
  const externalUrlQueryRef = useRef<string | null>(null);

  stateRef.current = state;
  stateQueryRef.current = buildItemsSearchParams(state).toString();
  const localStateAheadOfURL = intendedQueryRef.current === stateQueryRef.current;
  const renderState = localStateAheadOfURL || urlQuery === stateQueryRef.current ? state : urlState;
  const renderStateQuery = buildItemsSearchParams(renderState).toString();

  useEffect(() => {
    if (pendingUrlQueryRef.current === urlQuery) {
      pendingUrlQueryRef.current = null;
      externalUrlQueryRef.current = null;
      intendedQueryRef.current = null;
      return;
    }
    if (urlQuery === stateQueryRef.current) {
      if (externalUrlQueryRef.current === urlQuery) {
        externalUrlQueryRef.current = null;
      }
      if (intendedQueryRef.current === urlQuery) {
        intendedQueryRef.current = null;
      }
      return;
    }
    if (localStateAheadOfURL) {
      return;
    }
    externalUrlQueryRef.current = urlQuery;
    dispatch({ type: "hydrate_from_url", state: urlState });
  }, [localStateAheadOfURL, urlQuery, urlState]);

  useEffect(() => {
    const nextQuery = stateQueryRef.current;
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
  }, [pathname, router, searchParamsString, state]);

  const currentItemsHref = useMemo(() => {
    const query = renderStateQuery;
    return query ? `${pathname}?${query}` : pathname;
  }, [pathname, renderStateQuery]);

  const dispatchAction = useCallback((action: Parameters<typeof itemsViewStateReducer>[1]) => {
    const nextState = itemsViewStateReducer(stateRef.current, action);
    intendedQueryRef.current = buildItemsSearchParams(nextState).toString();
    dispatch(action);
  }, []);

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
