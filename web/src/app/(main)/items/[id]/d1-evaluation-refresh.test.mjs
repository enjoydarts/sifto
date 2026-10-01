import assert from "node:assert/strict";
import test from "node:test";
import { getPendingD1Evaluations, startD1EvaluationRefresh } from "./d1-evaluation-refresh.ts";

const now = Date.parse("2026-10-01T05:40:00Z");
const item = {
  status: "summarized",
  facts: { facts: ["fact"], extracted_at: "2026-10-01T05:33:00Z" },
  summary: { summary: "summary", summarized_at: "2026-10-01T05:34:00Z" },
};

test("refreshes missing D1 results after article processing finishes", () => {
  assert.deepEqual(getPendingD1Evaluations(item, true, now), { facts: true, faithfulness: true });
});

test("stops each refresh when its evaluation arrives, including final errors", () => {
  const next = { ...item, facts_d1_quality_evaluation: { decision: "error" } };
  assert.deepEqual(getPendingD1Evaluations(next, true, now), { facts: false, faithfulness: true });
  next.faithfulness_d1_quality_evaluation = { decision: "accepted" };
  assert.deepEqual(getPendingD1Evaluations(next, true, now), { facts: false, faithfulness: false });
});

test("does not poll without a D1 key, candidate, or for deleted articles", () => {
  for (const [candidate, enabled] of [[item, false], [null, true], [{ ...item, status: "deleted" }, true], [{ status: "fetched" }, true]]) {
    assert.deepEqual(getPendingD1Evaluations(candidate, enabled, now), { facts: false, faithfulness: false });
  }
});

test("bounds polling for old articles and invalid timestamps", () => {
  assert.deepEqual(getPendingD1Evaluations(item, true, now + 60 * 60_000), { facts: false, faithfulness: false });
  assert.deepEqual(getPendingD1Evaluations({ ...item, facts: { facts: ["fact"], extracted_at: "invalid" }, summary: null }, true, now), { facts: false, faithfulness: false });
});

test("facts refresh can finish independently of a summary", () => {
  assert.deepEqual(getPendingD1Evaluations({ ...item, summary: null }, true, now), { facts: true, faithfulness: false });
});

const flush = () => new Promise((resolve) => setImmediate(resolve));

test("polls until both results arrive and then stops", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now });
  const updates = [];
  let calls = 0;
  const stop = startD1EvaluationRefresh({
    item, enabled: true,
    load: async () => {
      calls++;
      return { ...item, facts_d1_quality_evaluation: { decision: "error" },
        ...(calls > 1 ? { faithfulness_d1_quality_evaluation: { decision: "accepted" } } : {}) };
    },
    onDetail: (detail) => updates.push(detail),
  });
  t.after(stop);
  t.mock.timers.tick(15_000);
  await flush();
  assert.equal(updates.length, 1);
  t.mock.timers.tick(15_000);
  await flush();
  t.mock.timers.tick(60_000);
  await flush();
  assert.equal(calls, 2);
  assert.equal(updates.length, 2);
});

test("retries transient load failures and bounds polling by article age", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now });
  let calls = 0;
  const stop = startD1EvaluationRefresh({ item, enabled: true,
    load: async () => { calls++; throw new Error("temporary network failure"); },
    onDetail: () => assert.fail("failed load must preserve the displayed article"),
  });
  t.after(stop);
  t.mock.timers.tick(15_000);
  await flush();
  t.mock.timers.tick(15_000);
  await flush();
  assert.equal(calls, 2);
  t.mock.timers.tick(60 * 60_000);
  await flush();
  const finalCalls = calls;
  t.mock.timers.tick(60_000);
  await flush();
  assert.equal(calls, finalCalls);
});

test("navigation cancels pending requests and ignores in-flight responses", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now });
  let resolve;
  let calls = 0;
  const stop = startD1EvaluationRefresh({ item, enabled: true,
    load: () => { calls++; return new Promise((done) => { resolve = done; }); },
    onDetail: () => assert.fail("old article response applied after navigation"),
  });
  t.mock.timers.tick(15_000);
  stop();
  resolve(item);
  await flush();
  t.mock.timers.tick(60_000);
  await flush();
  assert.equal(calls, 1);
});
