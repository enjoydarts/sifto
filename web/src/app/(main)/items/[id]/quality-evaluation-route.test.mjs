import assert from "node:assert/strict";
import test from "node:test";
import { qualityEvaluationRoute } from "./quality-evaluation-route.ts";

const jev = { provider: "jev", decision: "escalated", gate_policy_version: "jev-quality-gate-v5", attempt_index: 1 };
const d1 = { ...jev, provider: "d1", decision: "accepted", gate_policy_version: "d1-conditional-gate-v1" };
const passed = { retry_count: 1, short_comment: "D1の高信頼品質ゲートを通過しました。" };

test("a D1 rescue keeps Jev's own gate label and attributes approval only to D1", () => {
  assert.equal(qualityEvaluationRoute(jev, passed), "itemDetail.jev.gate");
  assert.equal(qualityEvaluationRoute(d1, passed), "itemDetail.d1.route.accepted");
});

test("an old or other-attempt D1 acceptance does not imply final approval", () => {
  assert.equal(qualityEvaluationRoute(jev, { ...passed, retry_count: 2 }), "itemDetail.jev.gate");
  assert.equal(qualityEvaluationRoute(d1, { ...passed, retry_count: 2 }), "itemDetail.d1.gate");
  assert.equal(qualityEvaluationRoute(d1, { retry_count: 1, short_comment: "LLM review" }), "itemDetail.d1.gate");
  assert.equal(qualityEvaluationRoute({ ...d1, gate_policy_version: "d1-shadow-v1" }, passed), "itemDetail.d1.shadow");
});

test("Jev acceptance and D1 errors retain their actual evaluation routes", () => {
  assert.equal(qualityEvaluationRoute({ ...jev, decision: "accepted" }), "itemDetail.jev.route.accepted");
  assert.equal(qualityEvaluationRoute({ ...d1, decision: "error" }), "itemDetail.d1.gate");
});

test("a final D1 comment does not relabel Jev acceptance or failure", () => {
  assert.equal(qualityEvaluationRoute({ ...jev, decision: "accepted" }, passed), "itemDetail.jev.route.accepted");
  assert.equal(qualityEvaluationRoute({ ...jev, decision: "error" }, passed), "itemDetail.jev.gate");
});

test("a D1 comment cannot turn an escalated or failed D1 evaluation into acceptance", () => {
  assert.equal(qualityEvaluationRoute({ ...d1, decision: "escalated" }, passed), "itemDetail.d1.gate");
  assert.equal(qualityEvaluationRoute({ ...d1, decision: "error" }, passed), "itemDetail.d1.gate");
});

test("Jev escalation identifies Jev without implying an LLM check ran", () => {
  assert.equal(qualityEvaluationRoute(jev), "itemDetail.jev.gate");
  assert.equal(qualityEvaluationRoute(jev, { retry_count: 1, short_comment: "LLM review" }), "itemDetail.jev.gate");
});
