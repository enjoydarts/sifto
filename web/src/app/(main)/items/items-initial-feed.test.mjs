import assert from "node:assert/strict";
import test from "node:test";
import { matchesItemsInitialFeed } from "./items-initial-feed.ts";

test("only uses the server response for the same account and canonical query", () => {
  const feed = { userId: "user-1", query: "feed=unread&sort=personal_score" };
  assert.equal(matchesItemsInitialFeed(feed, "user-1", feed.query), true);
  assert.equal(matchesItemsInitialFeed(feed, "user-2", feed.query), false);
  assert.equal(matchesItemsInitialFeed(feed, null, feed.query), false);
  assert.equal(matchesItemsInitialFeed(feed, "user-1", feed.query + "&page=2"), false);
  assert.equal(matchesItemsInitialFeed(undefined, "user-1", feed.query), false);
});
