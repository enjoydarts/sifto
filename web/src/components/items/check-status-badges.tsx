"use client";

type Props = {
  factsCheckResult?: string | null;
  faithfulnessResult?: string | null;
  t: (key: string, fallback?: string) => string;
  compact?: boolean;
};

function badgeTone(result: string) {
  return result === "fail"
    ? "border-rose-200 bg-rose-50 text-rose-700"
    : "border-amber-200 bg-amber-50 text-amber-700";
}

function labelFor(prefix: "facts" | "faithfulness", result: string, t: Props["t"]) {
  const kindKey = prefix === "facts" ? "items.check.facts" : "items.check.faithfulness";
  const resultKey = `items.check.${result}`;
  return `${t(kindKey)} ${t(resultKey, result)}`;
}

export function CheckStatusBadges({ factsCheckResult, faithfulnessResult, t, compact = false }: Props) {
  const entries = [
    factsCheckResult === "warn" || factsCheckResult === "fail"
      ? { key: "facts", result: factsCheckResult }
      : null,
    faithfulnessResult === "warn" || faithfulnessResult === "fail"
      ? { key: "faithfulness", result: faithfulnessResult }
      : null,
  ].filter(Boolean) as Array<{ key: "facts" | "faithfulness"; result: string }>;

  if (entries.length === 0) return null;

  return (
    <div className={`flex flex-wrap items-center gap-1.5 ${compact ? "" : "mt-2"}`}>
      {entries.map((entry) => (
        <span
          key={`${entry.key}-${entry.result}`}
          className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold ${badgeTone(entry.result)}`}
        >
          {labelFor(entry.key, entry.result, t)}
        </span>
      ))}
    </div>
  );
}
