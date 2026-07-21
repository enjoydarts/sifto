import assert from "node:assert/strict";
import test from "node:test";

import {
  ITEM_DETAIL_STALE_TIME_MS,
  ITEM_RELATED_STALE_TIME_MS,
  ITEMS_FEED_STALE_TIME_MS,
  canReuseItemsFeedPlaceholder,
  isItemDetailPrimaryContentReady,
  isItemScopedStateCurrent,
  itemDetailPrimaryContentRoute,
  itemsPrimaryContentRoute,
} from "./items-performance-policy.ts";

const itemsFeedKey = ({
  feed = "all",
  filter = "",
  search = "",
  page = 1,
} = {}) => [
  "items-feed",
  feed,
  filter,
  "",
  "",
  "",
  search,
  "natural",
  page,
  "newest",
  0,
  0,
  0,
];

test("defines item screen stale-time policy", () => {
  assert.equal(ITEMS_FEED_STALE_TIME_MS, 30_000);
  assert.equal(ITEM_DETAIL_STALE_TIME_MS, 5 * 60_000);
  assert.equal(ITEM_RELATED_STALE_TIME_MS, 5 * 60_000);
});

test("defines item screen primary content routes", () => {
  assert.equal(itemsPrimaryContentRoute, "items");
  assert.equal(itemDetailPrimaryContentRoute, "item-detail");
});

test("reuses item feed placeholder for pagination within the same result set", () => {
  assert.equal(canReuseItemsFeedPlaceholder(itemsFeedKey({ page: 1 }), itemsFeedKey({ page: 2 })), true);
});

test("does not reuse item feed placeholder when the feed changes", () => {
  assert.equal(canReuseItemsFeedPlaceholder(itemsFeedKey(), itemsFeedKey({ feed: "unread" })), false);
});

test("does not reuse item feed placeholder when a filter or search changes", () => {
  assert.equal(canReuseItemsFeedPlaceholder(itemsFeedKey(), itemsFeedKey({ filter: "failed" })), false);
  assert.equal(canReuseItemsFeedPlaceholder(itemsFeedKey(), itemsFeedKey({ search: "query" })), false);
});

test("does not reuse item feed placeholder without a previous key", () => {
  assert.equal(canReuseItemsFeedPlaceholder(undefined, itemsFeedKey()), false);
});

test("item detail primary content is not ready for a previously displayed item", () => {
  assert.equal(
    isItemDetailPrimaryContentReady({
      requestedItemId: "next-item",
      displayedItemId: "previous-item",
      loading: false,
      hasError: false,
    }),
    false
  );
});

test("item detail primary content is ready for the settled requested item", () => {
  assert.equal(
    isItemDetailPrimaryContentReady({
      requestedItemId: "next-item",
      displayedItemId: "next-item",
      loading: false,
      hasError: false,
    }),
    true
  );
});

test("item detail primary content is not ready while loading", () => {
  assert.equal(
    isItemDetailPrimaryContentReady({
      requestedItemId: "next-item",
      displayedItemId: "next-item",
      loading: true,
      hasError: false,
    }),
    false
  );
});

test("item detail primary content is not ready after a load error", () => {
  assert.equal(
    isItemDetailPrimaryContentReady({
      requestedItemId: "next-item",
      displayedItemId: "next-item",
      loading: false,
      hasError: true,
    }),
    false
  );
});

test("item-scoped state is not current when it belongs to the previous item", () => {
  assert.equal(isItemScopedStateCurrent("next-item", "previous-item"), false);
});

test("item-scoped state is current when its owner matches the requested item", () => {
  assert.equal(isItemScopedStateCurrent("next-item", "next-item"), true);
});

test("item-scoped state without an owner is not current", () => {
  assert.equal(isItemScopedStateCurrent("next-item", null), false);
});
