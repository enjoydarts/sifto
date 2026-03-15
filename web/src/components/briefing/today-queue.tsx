"use client";

import { Clock3 } from "lucide-react";
import { TodayQueueItem } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

type TodayQueueProps = {
  items: TodayQueueItem[];
  onOpen: (itemId: string) => void;
  onRead: (itemId: string) => void;
  onLater: (itemId: string) => void;
};

export function TodayQueue({ items, onOpen, onRead, onLater }: TodayQueueProps) {
  const { t } = useI18n();
  if (items.length === 0) return null;
  const labelForReason = (reason: string) => {
    if (reason === "priority goal") return t("briefing.todayQueue.reason.goal");
    if (reason === "fresh") return t("briefing.todayQueue.reason.fresh");
    if (reason === "attention") return t("briefing.todayQueue.reason.attention");
    return reason;
  };

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm md:p-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.todayQueue.title")}</h2>
          <p className="mt-1 text-sm text-zinc-500">{t("briefing.todayQueue.subtitle")}</p>
        </div>
      </div>
      <div className="mt-4 space-y-3">
        {items.map((entry, index) => (
          <article key={entry.item.id} className="rounded-2xl border border-zinc-200 bg-zinc-50/70 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                  <span className="rounded-full bg-blue-100 px-2 py-1 font-semibold text-blue-700">
                    {t("briefing.todayQueue.rank")} {index + 1}
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <Clock3 className="size-3.5" />
                    {entry.estimated_reading_minutes}
                    {t("briefing.todayQueue.minutes")}
                  </span>
                </div>
                <button type="button" onClick={() => onOpen(entry.item.id)} className="mt-2 block text-left">
                  <h3 className="line-clamp-2 text-base font-semibold text-zinc-900 hover:underline">
                    {entry.item.translated_title || entry.item.title || entry.item.url}
                  </h3>
                </button>
                <div className="mt-2 flex flex-wrap gap-2">
                  {entry.reason_labels.map((reason) => (
                    <span key={`${entry.item.id}-${reason}`} className="rounded-full border border-zinc-200 bg-white px-2 py-1 text-[11px] text-zinc-600">
                      {labelForReason(reason)}
                    </span>
                  ))}
                  {(entry.matched_goals ?? []).slice(0, 2).map((goal) => (
                    <span key={goal.id} className="rounded-full bg-amber-100 px-2 py-1 text-[11px] font-medium text-amber-800">
                      {goal.title}
                    </span>
                  ))}
                </div>
              </div>
              <div className="flex shrink-0 flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => onOpen(entry.item.id)}
                  className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 transition-colors hover:bg-zinc-50"
                >
                  {t("briefing.todayQueue.open")}
                </button>
                <button
                  type="button"
                  onClick={() => onRead(entry.item.id)}
                  className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 transition-colors hover:bg-zinc-50"
                >
                  {t("briefing.todayQueue.read")}
                </button>
                <button
                  type="button"
                  onClick={() => onLater(entry.item.id)}
                  className="rounded-lg bg-zinc-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-800"
                >
                  {t("briefing.todayQueue.later")}
                </button>
              </div>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
