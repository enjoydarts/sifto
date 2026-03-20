import type { ReactNode } from "react";
import { AlertTriangle, type LucideIcon } from "lucide-react";

type ErrorTone = "neutral" | "accent" | "warning" | "danger" | "info";

type ErrorStateProps = {
  title: ReactNode;
  description: ReactNode;
  action?: ReactNode;
  actionLabel?: ReactNode;
  onAction?: () => void;
  icon?: ReactNode | LucideIcon;
  tone?: ErrorTone;
  compact?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

function isLucideIcon(value: ErrorStateProps["icon"]): value is LucideIcon {
  return typeof value === "function";
}

const TONE_CLASS: Record<ErrorTone, { border: string; iconBg: string; iconText: string }> = {
  neutral: {
    border: "border-editorial",
    iconBg: "bg-[var(--panel-muted)]",
    iconText: "text-editorial-muted",
  },
  accent: {
    border: "border-[#d7b4a9]",
    iconBg: "bg-[var(--accent-soft)]",
    iconText: "text-[var(--accent)]",
  },
  warning: {
    border: "border-[#e1cb9e]",
    iconBg: "bg-[var(--warning-soft)]",
    iconText: "text-[var(--warning)]",
  },
  danger: {
    border: "border-[#dbb3b1]",
    iconBg: "bg-[var(--error-soft)]",
    iconText: "text-[var(--error)]",
  },
  info: {
    border: "border-[#bdd5e8]",
    iconBg: "bg-[var(--info-soft)]",
    iconText: "text-[var(--info)]",
  },
};

export function ErrorState({
  title,
  description,
  action,
  actionLabel,
  onAction,
  icon,
  tone = "danger",
  compact = false,
  className = "",
}: ErrorStateProps) {
  const toneClass = TONE_CLASS[tone];
  let iconNode: ReactNode = <AlertTriangle className="size-6" aria-hidden="true" />;
  if (isLucideIcon(icon)) {
    const IconComponent = icon;
    iconNode = <IconComponent className="size-6" aria-hidden="true" />;
  } else if (icon) {
    iconNode = <>{icon}</>;
  }

  return (
    <div
      className={joinClassNames(
        "surface-editorial rounded-[var(--radius-panel)] px-5 py-6 text-center",
        toneClass.border,
        compact ? "py-5" : "",
        className
      )}
    >
      <div className={joinClassNames("mx-auto inline-flex rounded-2xl p-4", toneClass.iconBg, toneClass.iconText)}>
        {iconNode}
      </div>
      <h3 className={joinClassNames("mt-4 text-[1.1rem] font-semibold tracking-[-0.02em]", compact ? "mt-3" : "")}>{title}</h3>
      <p className="mx-auto mt-2 max-w-xl text-[14px] leading-7 text-editorial-muted">{description}</p>
      {action && <div className="mt-5 flex justify-center">{action}</div>}
      {!action && actionLabel && onAction && (
        <div className="mt-5 flex justify-center">
          <button
            type="button"
            onClick={onAction}
            className="inline-flex items-center rounded-lg bg-[var(--foreground)] px-4 py-2 text-sm font-medium text-white press focus-ring"
          >
            {actionLabel}
          </button>
        </div>
      )}
    </div>
  );
}
