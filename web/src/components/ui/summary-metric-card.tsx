import type { ReactNode } from "react";

type SummaryMetricTone =
  | "neutral"
  | "default"
  | "muted"
  | "accent"
  | "success"
  | "warning"
  | "danger"
  | "info";

export type SummaryMetricCardProps = {
  label: ReactNode;
  value: ReactNode;
  hint?: ReactNode;
  tone?: SummaryMetricTone;
  compact?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

const TONE_CLASS: Record<SummaryMetricTone, { border: string; accent: string; value: string }> = {
  neutral: { border: "border-editorial", accent: "text-editorial-accent", value: "text-editorial-strong" },
  default: { border: "border-editorial", accent: "text-editorial-accent", value: "text-editorial-strong" },
  muted: { border: "border-editorial", accent: "text-editorial-muted", value: "text-editorial-strong" },
  accent: { border: "border-[#d7b4a9]", accent: "text-editorial-accent", value: "text-editorial-strong" },
  success: { border: "border-[#b9d4c9]", accent: "text-[var(--success)]", value: "text-[var(--success)]" },
  warning: { border: "border-[#e1cb9e]", accent: "text-[var(--warning)]", value: "text-[var(--warning)]" },
  danger: { border: "border-[#dbb3b1]", accent: "text-[var(--error)]", value: "text-[var(--error)]" },
  info: { border: "border-[#bdd5e8]", accent: "text-[var(--info)]", value: "text-[var(--info)]" },
};

export function SummaryMetricCard({
  label,
  value,
  hint,
  tone = "neutral",
  compact = false,
  className = "",
}: SummaryMetricCardProps) {
  const toneClass = TONE_CLASS[tone];

  return (
    <div
      className={joinClassNames(
        "surface-editorial rounded-[var(--radius-card)] px-4 py-4 sm:px-5",
        toneClass.border,
        compact ? "px-3 py-3 sm:px-4" : "",
        className
      )}
    >
      <div className={joinClassNames("text-[10px] font-semibold uppercase tracking-[0.16em]", toneClass.accent)}>
        {label}
      </div>
      <div className={joinClassNames("mt-3 tabular-nums tracking-[-0.03em]", toneClass.value, compact ? "text-[1.7rem]" : "text-[2.2rem]")}>
        {value}
      </div>
      {hint && <div className={joinClassNames("mt-2 text-editorial-muted", compact ? "text-[12px] leading-5" : "text-[13px] leading-6")}>{hint}</div>}
    </div>
  );
}
