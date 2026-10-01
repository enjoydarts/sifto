import type { ItemDetail } from "../../../../types/api/items";

export type D1PendingItem = Pick<ItemDetail, "status" | "facts" | "summary" | "facts_d1_quality_evaluation" | "faithfulness_d1_quality_evaluation">;

export function getPendingD1Evaluations(item: D1PendingItem | null, enabled: boolean, now = Date.now()) {
  if (!enabled || !item || item.status === "deleted") return { facts: false, faithfulness: false };
  // Old articles may never have had D1 enabled. Bound background refreshes.
  const isRecent = (timestamp?: string) => {
    const age = now - Date.parse(timestamp ?? "");
    return Number.isFinite(age) && age >= 0 && age < 60 * 60_000;
  };
  return {
    facts: Boolean(item.facts?.facts.length && !item.facts_d1_quality_evaluation && isRecent(item.facts.extracted_at)),
    faithfulness: Boolean(item.summary?.summary && !item.faithfulness_d1_quality_evaluation && isRecent(item.summary.summarized_at)),
  };
}

export function startD1EvaluationRefresh<T extends D1PendingItem>({
  item, enabled, load, onDetail,
}: { item: T; enabled: boolean; load: () => Promise<T>; onDetail: (detail: T) => void }) {
  let cancelled = false;
  let latest = item;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const schedule = () => {
    const pending = getPendingD1Evaluations(latest, enabled);
    if (cancelled || (!pending.facts && !pending.faithfulness)) return;
    timer = setTimeout(async () => {
      try {
        const detail = await load();
        if (cancelled) return;
        latest = detail;
        onDetail(detail);
      } catch {
        // Keep the article visible and try again while results are pending.
      }
      schedule();
    }, 15_000);
  };
  schedule();
  return () => {
    cancelled = true;
    clearTimeout(timer);
  };
}
