type Evaluation = {
  provider: string;
  decision: string;
  gate_policy_version: string;
  attempt_index: number;
};

type FinalCheck = { retry_count: number; short_comment?: string | null };

export function qualityEvaluationRoute(evaluation: Evaluation, finalCheck?: FinalCheck | null): string {
  const isD1 = evaluation.provider === "d1";
  const isD1Gate = evaluation.gate_policy_version === "d1-conditional-gate-v1";
  if (isD1 && !isD1Gate) return "itemDetail.d1.shadow";
  if (finalCheck?.short_comment === "D1の高信頼品質ゲートを通過しました。" && finalCheck.retry_count === evaluation.attempt_index) {
    return "itemDetail.d1.route.accepted";
  }
  if (isD1) return "itemDetail.d1.gate";
  return evaluation.decision === "accepted" ? "itemDetail.jev.route.accepted" : "itemDetail.jev.route.escalated";
}
