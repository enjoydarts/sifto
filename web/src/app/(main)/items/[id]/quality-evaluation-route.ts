type Evaluation = {
  provider: string;
  decision: string;
  gate_policy_version: string;
  attempt_index: number;
};

type FinalCheck = { retry_count: number; short_comment?: string | null };

export function qualityEvaluationReason(evaluation: {
  provider: string;
  escalation_reason?: string | null;
  reason_detail?: string | null;
}): string | null {
  // Older evaluations store all HTTP failures under http_error. Interpret the
  // recorded status so billing failures also have a useful historical label.
  const status = /^(Jev|D1) HTTP status=(\d{3})\b/.exec(evaluation.reason_detail ?? "");
  if (evaluation.escalation_reason === "http_error" && status?.[1].toLowerCase() === evaluation.provider && status[2] === "402") {
    return "payment_required";
  }
  return evaluation.escalation_reason ?? null;
}

export function qualityEvaluationRoute(evaluation: Evaluation, finalCheck?: FinalCheck | null): string {
  const isD1 = evaluation.provider === "d1";
  const isD1Gate = evaluation.gate_policy_version === "d1-conditional-gate-v1";
  const approvedByD1 =
    finalCheck?.short_comment === "D1の高信頼品質ゲートを通過しました。" &&
    finalCheck.retry_count === evaluation.attempt_index;
  if (isD1 && !isD1Gate) {
    // Legacy late shadow results can replace the scores used by the gate.
    // Preserve the final approval while identifying the displayed scores.
    return approvedByD1 ? "itemDetail.d1.route.acceptedWithShadow" : "itemDetail.d1.shadow";
  }
  if (isD1 && evaluation.decision === "accepted" && approvedByD1) {
    return "itemDetail.d1.route.accepted";
  }
  if (isD1) return "itemDetail.d1.gate";
  return evaluation.decision === "accepted" ? "itemDetail.jev.route.accepted" : "itemDetail.jev.gate";
}
