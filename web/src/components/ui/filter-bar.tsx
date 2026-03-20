import type { ReactNode } from "react";

type FilterBarProps = {
  leading?: ReactNode;
  filters?: ReactNode;
  sort?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
  compact?: boolean;
  className?: string;
};

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

export function FilterBar({
  leading,
  filters,
  sort,
  actions,
  children,
  compact = false,
  className = "",
}: FilterBarProps) {
  return (
    <section
      className={joinClassNames(
        "surface-editorial max-w-full overflow-x-hidden rounded-[var(--radius-panel)] px-4 py-4 sm:px-5",
        compact ? "py-3 sm:py-4" : "",
        className
      )}
    >
      <div className="flex flex-col gap-3 xl:flex-row xl:flex-nowrap xl:items-center">
        {leading && <div className="min-w-0 flex-1">{leading}</div>}
        {filters && <div className="min-w-0 shrink-0">{filters}</div>}
        {sort && <div className="min-w-0 shrink-0">{sort}</div>}
        {actions && <div className="min-w-0 shrink-0">{actions}</div>}
      </div>

      {children && (
        <div className="mt-3 border-t border-editorial pt-3">
          {children}
        </div>
      )}
    </section>
  );
}
