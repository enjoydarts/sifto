import type { ReactNode } from "react";

type StatusTone = "neutral" | "default" | "muted" | "accent" | "success" | "warning" | "danger" | "error" | "info";

type StatusPillProps = {
  children: ReactNode;
  tone?: StatusTone;
  icon?: ReactNode;
  compact?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

const TONE_CLASS: Record<StatusTone, string> = {
  neutral: "border-editorial bg-[var(--panel)] text-editorial-muted",
  default: "border-editorial bg-[var(--panel)] text-editorial-muted",
  muted: "border-editorial bg-[var(--panel-muted)] text-editorial-muted",
  accent: "border-[#d7b4a9] bg-[var(--accent-soft)] text-[var(--accent)]",
  success: "border-[#b9d4c9] bg-[var(--success-soft)] text-[var(--success)]",
  warning: "border-[#e1cb9e] bg-[var(--warning-soft)] text-[var(--warning)]",
  danger: "border-[#dbb3b1] bg-[var(--error-soft)] text-[var(--error)]",
  error: "border-[#dbb3b1] bg-[var(--error-soft)] text-[var(--error)]",
  info: "border-[#bdd5e8] bg-[var(--info-soft)] text-[var(--info)]",
};

export function StatusPill({
  children,
  tone = "neutral",
  icon,
  compact = false,
  className = "",
}: StatusPillProps) {
  return (
    <span
      className={joinClassNames(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.13em]",
        compact ? "px-2 py-0.5 text-[9px]" : "",
        TONE_CLASS[tone],
        className
      )}
    >
      {icon && <span className="shrink-0">{icon}</span>}
      <span>{children}</span>
    </span>
  );
}
