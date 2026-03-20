"use client";

import type { ReactNode } from "react";

type PageHeaderProps = {
  eyebrow?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
  compact?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export function PageHeader({
  eyebrow,
  title,
  description,
  meta,
  actions,
  compact = false,
  className = "",
}: PageHeaderProps) {
  return (
    <section
      className={joinClassNames(
        "surface-editorial rounded-[var(--radius-panel)] px-5 py-5 sm:px-6",
        compact ? "px-4 py-3 sm:px-5 sm:py-4" : "",
        className
      )}
    >
      {eyebrow && (
        <div className={joinClassNames(
          "font-semibold uppercase tracking-[0.18em] text-editorial-accent",
          compact ? "text-[10px]" : "text-[11px]"
        )}>
          {eyebrow}
        </div>
      )}

      <div className={joinClassNames("flex flex-col", compact ? "mt-1.5 gap-3 sm:mt-2 sm:gap-3.5" : "mt-3 gap-4 sm:mt-4")}>
        <div className={joinClassNames(
          "flex flex-col lg:flex-row lg:items-start lg:justify-between",
          compact ? "gap-3" : "gap-4"
        )}>
          <div className={joinClassNames("min-w-0", compact ? "space-y-1.5" : "space-y-2")}>
            <h1
              className={joinClassNames(
                "min-w-0 tracking-[-0.03em] text-editorial-strong",
                compact ? "text-[1.65rem] leading-none sm:text-[1.8rem]" : "text-[2rem] sm:text-[2.6rem]"
              )}
            >
              {title}
            </h1>
            {description && (
              <div className={joinClassNames(
                "max-w-3xl text-editorial-muted",
                compact ? "text-[13px] leading-5 sm:text-[14px] sm:leading-6" : "text-[15px] leading-7"
              )}>
                {description}
              </div>
            )}
          </div>

          {(actions || meta) && (
            <div className={joinClassNames(
              "flex flex-col items-start lg:items-end",
              compact ? "gap-2" : "gap-3"
            )}>
              {actions && (
                <div className={joinClassNames(
                  "flex flex-wrap items-center gap-2",
                  compact ? "w-full justify-end lg:w-auto" : ""
                )}>
                  {actions}
                </div>
              )}
              {meta && <div className="flex flex-wrap items-center gap-2 text-sm text-editorial-muted">{meta}</div>}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
