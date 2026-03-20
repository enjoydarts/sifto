"use client";

import { useMemo } from "react";
import { useI18n } from "@/components/i18n-provider";

type Props = {
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  className?: string;
};

export default function Pagination({
  total,
  page,
  pageSize,
  onPageChange,
  className,
}: Props) {
  const { t } = useI18n();
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const current = Math.min(Math.max(1, page), totalPages);

  const pages = useMemo(() => {
    const out: number[] = [];
    const start = Math.max(1, current - 2);
    const end = Math.min(totalPages, current + 2);
    for (let p = start; p <= end; p++) out.push(p);
    return out;
  }, [current, totalPages]);

  if (total <= pageSize) return null;

  return (
    <div className={`max-w-full overflow-x-hidden flex flex-wrap items-center justify-between gap-2 ${className ?? ""}`}>
      <div className="text-xs uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">
        {total.toLocaleString()} {t("common.rows")}
      </div>
      <div className="flex max-w-full flex-wrap items-center gap-1 sm:flex-nowrap">
        <button
          type="button"
          onClick={() => onPageChange(current - 1)}
          disabled={current <= 1}
          className="rounded-full border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs text-[var(--color-editorial-ink-soft)] disabled:opacity-40"
        >
          {t("common.prev")}
        </button>
        {pages.map((p) => (
          <button
            key={p}
            type="button"
            onClick={() => onPageChange(p)}
            className={`rounded-full px-3 py-1.5 text-xs ${
              p === current
                ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                : "border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)]"
            }`}
          >
            {p}
          </button>
        ))}
        <button
          type="button"
          onClick={() => onPageChange(current + 1)}
          disabled={current >= totalPages}
          className="rounded-full border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs text-[var(--color-editorial-ink-soft)] disabled:opacity-40"
        >
          {t("common.next")}
        </button>
        <span className="ml-0 w-full pt-1 text-xs uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)] sm:ml-2 sm:w-auto sm:pt-0">
          {t("common.page")} {current}/{totalPages}
        </span>
      </div>
    </div>
  );
}
