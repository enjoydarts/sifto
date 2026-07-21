import assert from "node:assert/strict";
import test from "node:test";

import {
  ITEM_DETAIL_STALE_TIME_MS,
  ITEM_RELATED_STALE_TIME_MS,
  ITEMS_FEED_STALE_TIME_MS,
  itemDetailPrimaryContentRoute,
  itemsPrimaryContentRoute,
} from "./items-performance-policy.ts";

test("defines item screen stale-time policy", () => {
  assert.equal(ITEMS_FEED_STALE_TIME_MS, 30_000);
  assert.equal(ITEM_DETAIL_STALE_TIME_MS, 5 * 60_000);
  assert.equal(ITEM_RELATED_STALE_TIME_MS, 5 * 60_000);
});

test("defines item screen primary content routes", () => {
  assert.equal(itemsPrimaryContentRoute, "items");
  assert.equal(itemDetailPrimaryContentRoute, "item-detail");
});
