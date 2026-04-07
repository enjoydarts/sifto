import test from "node:test";
import assert from "node:assert/strict";

import {
  buildItemsSearchParams,
  itemsViewStateReducer,
  normalizeItemsViewState,
  parseItemsQueryState,
} from "./items-view-state-core.ts";

function makeState(overrides = {}) {
  return {
    feedMode: "unread",
    sortMode: "score",
    filter: "",
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

test("buildItemsSearchParams serializes normalized state", () => {
  const params = buildItemsSearchParams(
    makeState({
      feedMode: "later",
      sortMode: "score",
      searchQuery: "tts",
      searchMode: "or",
      favoriteOnly: true,
      page: 2,
    })
  );

  assert.equal(
    params.toString(),
    "feed=later&q=tts&search_mode=or&sort=score&unread=1&favorite=1&page=2"
  );
});

test("parseItemsQueryState reads current URL flags", () => {
  const parsed = parseItemsQueryState(
    new URLSearchParams("feed=deleted&status=deleted&sort=personal_score&favorite=1&page=7")
  );

  assert.equal(parsed.feedMode, "deleted");
  assert.equal(parsed.sortMode, "personal_score");
  assert.equal(parsed.favoriteOnly, false);
  assert.equal(parsed.page, 7);
});
