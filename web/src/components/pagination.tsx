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
    <div className={`flex flex-wrap items-center justify-between gap-2 ${className ?? ""}`}>
      <div className="text-xs text-zinc-500">
        {total.toLocaleString()} {t("common.rows")}
      </div>
      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={() => onPageChange(current - 1)}
          disabled={current <= 1}
          className="rounded border border-zinc-300 bg-white px-2.5 py-1 text-xs text-zinc-700 disabled:opacity-40"
        >
          {t("common.prev")}
        </button>
        {pages.map((p) => (
          <button
            key={p}
            type="button"
            onClick={() => onPageChange(p)}
            className={`rounded px-2.5 py-1 text-xs ${
              p === current
                ? "bg-zinc-900 text-white"
                : "border border-zinc-300 bg-white text-zinc-700"
            }`}
          >
            {p}
          </button>
        ))}
        <button
          type="button"
          onClick={() => onPageChange(current + 1)}
          disabled={current >= totalPages}
          className="rounded border border-zinc-300 bg-white px-2.5 py-1 text-xs text-zinc-700 disabled:opacity-40"
        >
          {t("common.next")}
        </button>
        <span className="ml-2 text-xs text-zinc-500">
          {t("common.page")} {current}/{totalPages}
        </span>
      </div>
    </div>
  );
}
