import assert from "node:assert/strict";
import test from "node:test";
import { qualityEvaluationRoute } from "./quality-evaluation-route.ts";
import * as qualityEvaluation from "./quality-evaluation-route.ts";
import { jaDict } from "../../../../i18n/dictionaries/ja.ts";
import { enDict } from "../../../../i18n/dictionaries/en.ts";

const jev = { provider: "jev", decision: "escalated", gate_policy_version: "jev-quality-gate-v5", attempt_index: 1 };
const d1 = { ...jev, provider: "d1", decision: "accepted", gate_policy_version: "d1-conditional-gate-v1" };
const passed = { retry_count: 1, short_comment: "D1の高信頼品質ゲートを通過しました。" };

test("stored 402 failures explain billing instead of a connection problem in both languages", () => {
  assert.equal(typeof qualityEvaluation.qualityEvaluationReason, "function");
  for (const [provider, prefix] of [["jev", "Jev"], ["d1", "D1"]]) {
    const reason = qualityEvaluation.qualityEvaluationReason({
      provider, escalation_reason: "http_error", reason_detail: `${prefix} HTTP status=402`,
    });
    assert.equal(reason, "payment_required");
    for (const dictionary of [jaDict, enDict]) {
      const message = dictionary[`itemDetail.${provider}.reason.${reason}`];
      assert.ok(message);
      assert.match(message, /クレジット|credits/);
      assert.doesNotMatch(message, /接続|connection/);
    }
  }
});

test("other errors and gate decisions keep their recorded reason", () => {
  assert.equal(typeof qualityEvaluation.qualityEvaluationReason, "function");
  for (const reason_detail of [null, "", "Jev HTTP status=502", "Jev HTTP status=4020", "Jev request: connection refused", "Jev HTTP status=502 response: 402"]) {
    assert.equal(qualityEvaluation.qualityEvaluationReason({ provider: "jev", escalation_reason: "http_error", reason_detail }), "http_error");
  }
  assert.equal(qualityEvaluation.qualityEvaluationReason({ provider: "jev", escalation_reason: "timeout", reason_detail: "Jev HTTP status=402" }), "timeout");
  assert.equal(qualityEvaluation.qualityEvaluationReason({ provider: "jev", escalation_reason: "low_score" }), "low_score");
  assert.equal(qualityEvaluation.qualityEvaluationReason({ provider: "jev" }), null);
});

test("a D1 rescue keeps Jev's own gate label and attributes approval only to D1", () => {
  assert.equal(qualityEvaluationRoute(jev, passed), "itemDetail.jev.gate");
  assert.equal(qualityEvaluationRoute(d1, passed), "itemDetail.d1.route.accepted");
});

test("an old or other-attempt D1 acceptance does not imply final approval", () => {
  assert.equal(qualityEvaluationRoute(jev, { ...passed, retry_count: 2 }), "itemDetail.jev.gate");
  assert.equal(qualityEvaluationRoute(d1, { ...passed, retry_count: 2 }), "itemDetail.d1.gate");
  assert.equal(qualityEvaluationRoute(d1, { retry_count: 1, short_comment: "LLM review" }), "itemDetail.d1.gate");
  assert.equal(qualityEvaluationRoute({ ...d1, gate_policy_version: "d1-shadow-v1" }, { ...passed, retry_count: 2 }), "itemDetail.d1.shadow");
});

test("a late shadow distinguishes final D1 approval from its reference scores", () => {
  const shadow = { ...d1, gate_policy_version: "d1-shadow-v1" };
  for (const decision of ["accepted", "escalated", "error"]) {
    assert.equal(qualityEvaluationRoute({ ...shadow, decision }, passed), "itemDetail.d1.route.acceptedWithShadow");
  }
});

test("a shadow remains a reference when Jev or an LLM made the final decision", () => {
  const shadow = { ...d1, gate_policy_version: "d1-shadow-v1" };
  assert.equal(qualityEvaluationRoute(shadow), "itemDetail.d1.shadow");
  assert.equal(qualityEvaluationRoute(shadow, { retry_count: 1, short_comment: "Jevの高信頼品質ゲートを通過しました。" }), "itemDetail.d1.shadow");
  assert.equal(qualityEvaluationRoute(shadow, { retry_count: 1, short_comment: "LLM review" }), "itemDetail.d1.shadow");
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
