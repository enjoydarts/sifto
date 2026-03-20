import type { ReactNode } from "react";

type ScoreTone = "neutral" | "default" | "muted" | "accent" | "success" | "warning" | "danger" | "error" | "info";

type ScoreBadgeProps = {
  label?: ReactNode;
  value: ReactNode;
  helper?: ReactNode;
  tone?: ScoreTone;
  compact?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

const TONE_CLASS: Record<ScoreTone, { border: string; value: string; label: string }> = {
  neutral: { border: "border-editorial", value: "text-editorial-strong", label: "text-editorial-muted" },
  default: { border: "border-editorial", value: "text-editorial-strong", label: "text-editorial-muted" },
  muted: { border: "border-editorial", value: "text-editorial-muted", label: "text-editorial-muted" },
  accent: { border: "border-[#d7b4a9]", value: "text-[var(--accent)]", label: "text-editorial-muted" },
  success: { border: "border-[#b9d4c9]", value: "text-[var(--success)]", label: "text-editorial-muted" },
  warning: { border: "border-[#e1cb9e]", value: "text-[var(--warning)]", label: "text-editorial-muted" },
  danger: { border: "border-[#dbb3b1]", value: "text-[var(--error)]", label: "text-editorial-muted" },
  error: { border: "border-[#dbb3b1]", value: "text-[var(--error)]", label: "text-editorial-muted" },
  info: { border: "border-[#bdd5e8]", value: "text-[var(--info)]", label: "text-editorial-muted" },
};

export function ScoreBadge({
  label,
  value,
  helper,
  tone = "neutral",
  compact = false,
  className = "",
}: ScoreBadgeProps) {
  const toneClass = TONE_CLASS[tone];

  return (
    <div
      className={joinClassNames(
        "surface-editorial rounded-[18px] px-3 py-3",
        toneClass.border,
        compact ? "px-2.5 py-2.5" : "",
        className
      )}
    >
      {label && (
        <div className={joinClassNames("text-[10px] font-semibold uppercase tracking-[0.16em]", toneClass.label)}>
          {label}
        </div>
      )}
      <div className={joinClassNames("tabular-nums tracking-[-0.03em] font-semibold", toneClass.value, compact ? "mt-1 text-[1.25rem]" : "mt-2 text-[1.6rem]")}>
        {value}
      </div>
      {helper && <div className="mt-1.5 text-[12px] leading-5 text-editorial-muted">{helper}</div>}
    </div>
  );
}
