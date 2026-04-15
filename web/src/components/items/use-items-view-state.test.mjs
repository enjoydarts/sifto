import test from "node:test";
import assert from "node:assert/strict";

import {
  buildItemsSearchParams,
  itemsViewStateReducer,
  normalizeItemsViewState,
  parseItemsQueryState,
} from "./items-view-state-core.ts";
import {
  UNCATEGORIZED_GENRE_PARAM,
  normalizeGenreNavigationValue,
} from "./item-genre.ts";

function makeState(overrides = {}) {
  return {
    feedMode: "unread",
    sortMode: "score",
    filter: "",
    genre: "",
    topic: "",
    sourceID: "",
    searchQuery: "",
    searchMode: "natural",
    unreadOnly: false,
    favoriteOnly: false,
    page: 3,
    ...overrides,
  };
}

test("normalizeItemsViewState forces pending feed to newest without unread or favorite flags", () => {
  const normalized = normalizeItemsViewState(
    makeState({
      feedMode: "pending",
      sortMode: "personal_score",
      unreadOnly: true,
      favoriteOnly: true,
      page: 4,
    })
  );

  assert.equal(normalized.sortMode, "newest");
  assert.equal(normalized.unreadOnly, false);
  assert.equal(normalized.favoriteOnly, false);
  assert.equal(normalized.page, 4);
});

test("normalizeItemsViewState forces later feed to unread", () => {
  const normalized = normalizeItemsViewState(
    makeState({
      feedMode: "later",
      unreadOnly: false,
    })
  );

  assert.equal(normalized.unreadOnly, true);
});

test("itemsViewStateReducer resets page when feed changes", () => {
  const next = itemsViewStateReducer(
    makeState({
      page: 5,
      unreadOnly: true,
    }),
    {
      type: "set_feed",
      feed: "pending",
    }
  );

  assert.equal(next.feedMode, "pending");
  assert.equal(next.page, 1);
  assert.equal(next.sortMode, "newest");
  assert.equal(next.unreadOnly, false);
});

test("itemsViewStateReducer clears unread filter when switching to read feed", () => {
  const next = itemsViewStateReducer(
    makeState({
      feedMode: "later",
      unreadOnly: true,
      page: 4,
    }),
    {
      type: "set_feed",
      feed: "read",
    }
  );

  assert.equal(next.feedMode, "read");
  assert.equal(next.unreadOnly, false);
  assert.equal(next.page, 1);
});

test("itemsViewStateReducer restores unread default sort when moving from pending to unread", () => {
  const next = itemsViewStateReducer(
    makeState({
      feedMode: "pending",
      sortMode: "newest",
      page: 4,
    }),
    {
      type: "set_feed",
      feed: "unread",
    }
  );

  assert.equal(next.feedMode, "unread");
  assert.equal(next.sortMode, "personal_score");
  assert.equal(next.page, 1);
});

test("itemsViewStateReducer restores non-unread default sort when moving away from unread", () => {
  const next = itemsViewStateReducer(
    makeState({
      feedMode: "unread",
      sortMode: "personal_score",
      page: 2,
    }),
    {
      type: "set_feed",
      feed: "read",
    }
  );

  assert.equal(next.feedMode, "read");
  assert.equal(next.sortMode, "newest");
  assert.equal(next.page, 1);
});

test("itemsViewStateReducer resets page and updates search", () => {
  const next = itemsViewStateReducer(makeState({ page: 6 }), {
    type: "set_search",
    searchQuery: "fish audio",
    searchMode: "and",
  });

  assert.equal(next.searchQuery, "fish audio");
  assert.equal(next.searchMode, "and");
  assert.equal(next.page, 1);
});

test("itemsViewStateReducer resets page and updates genre", () => {
  const next = itemsViewStateReducer(makeState({ page: 6 }), {
    type: "set_genre",
    genre: "AI agents",
  });

  assert.equal(next.genre, "AI agents");
  assert.equal(next.page, 1);
});

test("buildItemsSearchParams serializes normalized state", () => {
  const params = buildItemsSearchParams(
    makeState({
      feedMode: "later",
      sortMode: "score",
      genre: "AI agents",
      searchQuery: "tts",
      searchMode: "or",
      favoriteOnly: true,
      page: 2,
    })
  );

  assert.equal(
    params.toString(),
    "feed=later&genre=AI+agents&q=tts&search_mode=or&sort=score&unread=1&favorite=1&page=2"
  );
});

test("parseItemsQueryState reads current URL flags", () => {
  const parsed = parseItemsQueryState(
    new URLSearchParams("feed=deleted&status=deleted&genre=Security&sort=personal_score&favorite=1&page=7")
  );

  assert.equal(parsed.feedMode, "deleted");
  assert.equal(parsed.genre, "Security");
  assert.equal(parsed.sortMode, "personal_score");
  assert.equal(parsed.favoriteOnly, false);
  assert.equal(parsed.page, 7);
});

test("normalizeGenreNavigationValue preserves uncategorized aliases for navigation state", () => {
  assert.equal(normalizeGenreNavigationValue("uncategorized"), UNCATEGORIZED_GENRE_PARAM);
  assert.equal(normalizeGenreNavigationValue("untagged"), UNCATEGORIZED_GENRE_PARAM);
});

test("parseItemsQueryState keeps bookmarked uncategorized genre intact", () => {
  const parsed = parseItemsQueryState(
    new URLSearchParams("feed=unread&genre=uncategorized&sort=personal_score")
  );

  assert.equal(parsed.genre, UNCATEGORIZED_GENRE_PARAM);
  assert.equal(buildItemsSearchParams(parsed).get("genre"), UNCATEGORIZED_GENRE_PARAM);
});

test("parseItemsQueryState canonicalizes untagged alias to uncategorized", () => {
  const parsed = parseItemsQueryState(
    new URLSearchParams("feed=unread&genre=untagged&sort=personal_score")
  );

  assert.equal(parsed.genre, UNCATEGORIZED_GENRE_PARAM);
  assert.equal(buildItemsSearchParams(parsed).get("genre"), UNCATEGORIZED_GENRE_PARAM);
});

test("itemsViewStateReducer reset_filters clears genre along with other filter state", () => {
  const next = itemsViewStateReducer(
    makeState({
      filter: "failed",
      genre: "Security",
      topic: "infra",
      sourceID: "source-1",
      searchQuery: "incident",
      favoriteOnly: true,
      page: 9,
    }),
    { type: "reset_filters" }
  );

  assert.equal(next.filter, "");
  assert.equal(next.genre, "");
  assert.equal(next.topic, "");
  assert.equal(next.sourceID, "");
  assert.equal(next.searchQuery, "");
  assert.equal(next.favoriteOnly, false);
  assert.equal(next.page, 1);
});
