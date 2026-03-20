"use client";

import type { MouseEvent, ReactNode } from "react";

type TagTone = "neutral" | "default" | "muted" | "accent" | "subtle" | "success" | "warning" | "danger" | "error" | "info";

type TagProps = {
  children: ReactNode;
  tone?: TagTone;
  icon?: ReactNode;
  removable?: boolean;
  onRemove?: () => void;
  removeLabel?: string;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

const TONE_CLASS: Record<TagTone, string> = {
  neutral: "border-editorial bg-[var(--panel)] text-editorial-muted",
  default: "border-editorial bg-[var(--panel)] text-editorial-muted",
  muted: "border-editorial bg-[var(--panel-muted)] text-editorial-muted",
  accent: "border-[#d7b4a9] bg-[var(--accent-soft)] text-[var(--accent)]",
  subtle: "border-editorial bg-[var(--panel-muted)] text-editorial-muted",
  success: "border-[#b9d4c9] bg-[var(--success-soft)] text-[var(--success)]",
  warning: "border-[#e1cb9e] bg-[var(--warning-soft)] text-[var(--warning)]",
  danger: "border-[#dbb3b1] bg-[var(--error-soft)] text-[var(--error)]",
  error: "border-[#dbb3b1] bg-[var(--error-soft)] text-[var(--error)]",
  info: "border-[#bdd5e8] bg-[var(--info-soft)] text-[var(--info)]",
};

export function Tag({
  children,
  tone = "neutral",
  icon,
  removable = false,
  onRemove,
  removeLabel = "Remove tag",
  className = "",
}: TagProps) {
  const isInteractive = removable || Boolean(onRemove);

  const handleRemove = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation();
    onRemove?.();
  };

  const sharedClassName = joinClassNames(
    "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-semibold",
    TONE_CLASS[tone],
    className
  );

  if (!isInteractive) {
    return (
      <span className={sharedClassName}>
        {icon && <span className="shrink-0">{icon}</span>}
        <span>{children}</span>
      </span>
    );
  }

  return (
    <button
      type="button"
      className={joinClassNames(sharedClassName, "press focus-ring", onRemove ? "cursor-pointer" : "cursor-default")}
      onClick={handleRemove}
      aria-label={removeLabel}
      disabled={!onRemove}
    >
      {icon && <span className="shrink-0">{icon}</span>}
      <span>{children}</span>
      <span aria-hidden="true" className="ml-0.5 text-[10px]">
        ×
      </span>
    </button>
  );
}
