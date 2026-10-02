import assert from "node:assert/strict";
import test from "node:test";
import { qualityEvaluationRoute } from "./quality-evaluation-route.ts";

const jev = { provider: "jev", decision: "escalated", gate_policy_version: "jev-quality-gate-v5", attempt_index: 1 };
const d1 = { ...jev, provider: "d1", decision: "accepted", gate_policy_version: "d1-conditional-gate-v1" };
const passed = { retry_count: 1, short_comment: "D1の高信頼品質ゲートを通過しました。" };

test("both cards attribute a D1 rescue to D1 without claiming an LLM review", () => {
  assert.equal(qualityEvaluationRoute(jev, passed), "itemDetail.d1.route.accepted");
  assert.equal(qualityEvaluationRoute(d1, passed), "itemDetail.d1.route.accepted");
});

test("an old or other-attempt D1 acceptance does not imply final approval", () => {
  assert.equal(qualityEvaluationRoute(jev, { ...passed, retry_count: 2 }), "itemDetail.jev.route.escalated");
  assert.equal(qualityEvaluationRoute(d1, { ...passed, retry_count: 2 }), "itemDetail.d1.gate");
  assert.equal(qualityEvaluationRoute(d1, { retry_count: 1, short_comment: "LLM review" }), "itemDetail.d1.gate");
  assert.equal(qualityEvaluationRoute({ ...d1, gate_policy_version: "d1-shadow-v1" }, passed), "itemDetail.d1.shadow");
});

test("Jev acceptance and D1 errors retain their actual evaluation routes", () => {
  assert.equal(qualityEvaluationRoute({ ...jev, decision: "accepted" }), "itemDetail.jev.route.accepted");
  assert.equal(qualityEvaluationRoute({ ...d1, decision: "error" }), "itemDetail.d1.gate");
});
