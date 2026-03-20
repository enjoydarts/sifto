"use client";

import Link from "next/link";
import { ReadingGoal } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";

export function ReadingGoalsPanel({ goals }: { goals: ReadingGoal[] }) {
  const { t } = useI18n();
  if (goals.length === 0) return null;

  return (
    <SectionCard>
      <div className="flex items-end justify-between gap-3">
        <div>
          <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
            Reading Goals
          </div>
          <h2 className="mt-2 font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
            {t("briefing.goals.title")}
          </h2>
          <p className="mt-2 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
            {t("briefing.goals.subtitle")}
          </p>
        </div>
        <Link href="/goals" className="font-sans text-[12px] font-semibold text-[var(--color-editorial-ink-faint)] hover:text-[var(--color-editorial-ink)]">
          {t("briefing.goals.openGoals")}
        </Link>
      </div>
      <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {goals.map((goal) => (
          <article key={goal.id} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="flex items-center justify-between gap-2">
              <h3 className="line-clamp-2 font-serif text-[1rem] font-semibold leading-[1.35] text-[var(--color-editorial-ink)]">
                {goal.title}
              </h3>
              <Tag>{`P${goal.priority}`}</Tag>
            </div>
            {goal.description ? (
              <p className="mt-2 line-clamp-2 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                {goal.description}
              </p>
            ) : null}
            {goal.due_date ? (
              <p className="mt-3 font-sans text-[11px] font-semibold uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                {t("briefing.goals.due")}: {goal.due_date}
              </p>
            ) : null}
          </article>
        ))}
      </div>
    </SectionCard>
  );
}
