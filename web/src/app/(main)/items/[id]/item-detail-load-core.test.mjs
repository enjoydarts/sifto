import assert from "node:assert/strict";
import test from "node:test";

import { startItemDetailLoads } from "./item-detail-load-core.ts";

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });

  return { promise, resolve, reject };
}

test("detail completion does not wait for related", async () => {
  const events = [];
  const related = deferred();
  const loads = startItemDetailLoads({
    loadDetail: async () => ({ id: "item-1" }),
    loadRelated: () => related.promise,
    onDetail: (detail) => events.push("detail", detail.id),
    onDetailError: () => events.push("detail-error"),
    onRelated: () => events.push("related"),
    onRelatedError: () => events.push("related-error"),
  });

  await loads.detail;
  assert.deepEqual(events, ["detail", "item-1"]);

  related.resolve({ items: [] });
  await loads.related;
  assert.deepEqual(events, ["detail", "item-1", "related"]);
});

test("related failure is isolated from successful detail", async () => {
  const events = [];
  const loads = startItemDetailLoads({
    loadDetail: async () => ({ id: "item-1" }),
    loadRelated: async () => {
      throw new Error("related unavailable");
    },
    onDetail: () => events.push("detail"),
    onDetailError: () => events.push("detail-error"),
    onRelated: () => events.push("related"),
    onRelatedError: () => events.push("related-error"),
  });

  await Promise.all([loads.detail, loads.related]);

  assert.deepEqual(events, ["detail", "related-error"]);
});

test("detail failure remains primary error", async () => {
  const events = [];
  const loads = startItemDetailLoads({
    loadDetail: async () => {
      throw new Error("detail unavailable");
    },
    loadRelated: async () => ({ items: [] }),
    onDetail: () => events.push("detail"),
    onDetailError: () => events.push("detail-error"),
    onRelated: () => events.push("related"),
    onRelatedError: () => events.push("related-error"),
  });

  await Promise.all([loads.detail, loads.related]);

  assert.deepEqual(events, ["detail-error", "related"]);
});
