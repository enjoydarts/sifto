import type { ReactNode } from "react";
import { SummaryMetricCard, type SummaryMetricCardProps } from "@/components/ui/summary-metric-card";

type SummaryStripProps = {
  title?: ReactNode;
  description?: ReactNode;
  items?: SummaryMetricCardProps[];
  children?: ReactNode;
  compact?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export function SummaryStrip({
  title,
  description,
  items,
  children,
  compact = false,
  className = "",
}: SummaryStripProps) {
  return (
    <section
      className={joinClassNames(
        "surface-editorial rounded-[var(--radius-panel)] px-5 py-5 sm:px-6",
        compact ? "py-4 sm:py-5" : "",
        className
      )}
    >
      {(title || description) && (
        <div className="flex flex-col gap-1">
          {title && <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-editorial-accent">{title}</div>}
          {description && <div className="max-w-3xl text-sm leading-6 text-editorial-muted">{description}</div>}
        </div>
      )}

      <div className={joinClassNames("mt-4", compact ? "mt-3" : "")}>
        {children ?? (
          <div className="grid grid-cols-2 gap-2 md:grid-cols-2 md:gap-3 xl:grid-cols-4">
            {(items ?? []).map((item, index) => (
              <SummaryMetricCard key={index} {...item} />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
