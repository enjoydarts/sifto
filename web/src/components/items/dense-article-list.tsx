"use client";

import type { ReactNode } from "react";
import Pagination from "@/components/pagination";
import { SectionCard } from "@/components/ui/section-card";

type DateSection = {
  date: string;
  items: ReactNode[];
};

export function DenseArticleList({
  sections,
  total,
  page,
  pageSize,
  onPageChange,
}: {
  sections: DateSection[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
}) {
  return (
    <SectionCard className="overflow-hidden border-0 bg-transparent shadow-none sm:border sm:bg-[var(--panel)] sm:shadow-[var(--shadow-card)]">
      <div className="space-y-5 p-0 sm:p-4">
        {sections.map((section) => (
          <section key={section.date} className="space-y-2">
            <h2 className="px-1 text-[9px] font-semibold uppercase tracking-[0.2em] text-[var(--color-editorial-ink-faint)] sm:text-[10px] sm:tracking-[0.22em]">
              {section.date}
            </h2>
            <ul className="list-none space-y-2">
              {section.items.map((item, idx) => (
                <li key={idx} className="min-w-0 list-none">
                  {item}
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
      <div className="border-t border-[var(--color-editorial-line)] px-0 py-3 sm:px-4">
        <Pagination
          total={total}
          page={page}
          pageSize={pageSize}
          onPageChange={onPageChange}
        />
      </div>
    </SectionCard>
  );
}
