"use client";

import Link from "next/link";
import { ReadingGoal } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

export function ReadingGoalsPanel({ goals }: { goals: ReadingGoal[] }) {
  const { t } = useI18n();
  if (goals.length === 0) return null;

  return (
    <section className="rounded-2xl border border-zinc-200 bg-zinc-50/80 p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.goals.title")}</h2>
          <p className="mt-1 text-sm text-zinc-500">{t("briefing.goals.subtitle")}</p>
        </div>
        <Link href="/settings" className="text-xs text-zinc-500 transition-colors hover:text-zinc-900">
          {t("briefing.goals.openSettings")}
        </Link>
      </div>
      <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {goals.map((goal) => (
          <article key={goal.id} className="rounded-xl border border-zinc-200 bg-white p-3">
            <div className="flex items-center justify-between gap-2">
              <h3 className="line-clamp-2 text-sm font-semibold text-zinc-900">{goal.title}</h3>
              <span className="rounded-full bg-zinc-100 px-2 py-1 text-[11px] font-medium text-zinc-700">
                P{goal.priority}
              </span>
            </div>
            {goal.description ? (
              <p className="mt-2 line-clamp-2 text-sm text-zinc-600">{goal.description}</p>
            ) : null}
            {goal.due_date ? (
              <p className="mt-2 text-xs text-zinc-500">
                {t("briefing.goals.due")}: {goal.due_date}
              </p>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  );
}
