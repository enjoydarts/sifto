import type { CSSProperties, HTMLAttributes, ReactNode } from "react";

type ListRowCardProps = HTMLAttributes<HTMLDivElement> & {
  children?: ReactNode;
  eyebrow?: ReactNode;
  title?: ReactNode;
  description?: ReactNode;
  meta?: ReactNode;
  status?: ReactNode;
  score?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
  media?: ReactNode;
  featured?: boolean;
  read?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export function ListRowCard({
  children,
  eyebrow,
  title,
  description,
  meta,
  status,
  score,
  actions,
  footer,
  media,
  featured = false,
  read = false,
  className = "",
  style,
  ...rest
}: ListRowCardProps) {
  const surfaceClass = read
    ? "bg-[var(--color-editorial-panel-muted)] border-[var(--color-editorial-line)]"
    : featured
      ? "bg-[linear-gradient(180deg,#fffaf7_0%,#fffdfb_100%)] border-[var(--color-editorial-accent-line)] shadow-[var(--shadow-dropdown)]"
      : "bg-[var(--color-editorial-panel-strong)] border-[var(--color-editorial-line)] shadow-[var(--shadow-card)]";

  return (
    <div
      className={joinClassNames(
        "max-w-full overflow-hidden rounded-[20px] border px-4 py-4 sm:px-4",
        surfaceClass,
        className
      )}
      style={style as CSSProperties | undefined}
      {...rest}
    >
      {children ?? (
        <article
          className={joinClassNames("flex flex-col gap-4", featured ? "md:flex-row md:items-start" : "sm:flex-row sm:items-start")}
        >
          {media && (
            <div
              className={joinClassNames(
                "shrink-0 overflow-hidden rounded-[14px] border border-editorial bg-[var(--panel-muted)]",
                featured ? "w-full md:w-[136px]" : "w-[88px] sm:w-[104px]"
              )}
            >
              {media}
            </div>
          )}

          <div className="min-w-0 flex-1 space-y-2">
            {eyebrow && <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-editorial-accent">{eyebrow}</div>}
            <div className={joinClassNames("min-w-0 tracking-[-0.02em] text-editorial-strong", featured ? "text-[1.4rem] leading-8" : "text-[1.15rem] leading-7")}>
              {title}
            </div>
            {description && <div className="max-w-4xl text-[14px] leading-7 text-editorial-muted">{description}</div>}

            {(meta || status) && (
              <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-[11px] font-semibold uppercase tracking-[0.11em] text-editorial-muted">
                {meta}
                {status}
              </div>
            )}

            {footer && <div>{footer}</div>}
          </div>

          {(score || actions) && (
            <div className={joinClassNames("flex shrink-0 flex-col gap-3", featured ? "md:items-end" : "sm:items-end")}>
              {score}
              {actions}
            </div>
          )}
        </article>
      )}
    </div>
  );
}
