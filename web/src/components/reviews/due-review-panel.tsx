"use client";

import { ReviewQueueItem } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

interface DueReviewPanelProps {
  items: ReviewQueueItem[];
  onOpen: (itemId: string) => void;
  onDone: (id: string) => void;
  onSnooze: (id: string) => void;
}

export function DueReviewPanel({ items, onOpen, onDone, onSnooze }: DueReviewPanelProps) {
  const { t } = useI18n();
  if (items.length === 0) return null;
  return (
    <section className="rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div>
        <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-400">{t("reviewQueue.eyebrow")}</p>
        <h2 className="mt-2 text-lg font-semibold text-zinc-950">{t("reviewQueue.title")}</h2>
        <p className="mt-1 text-sm text-zinc-500">{t("reviewQueue.subtitle")}</p>
      </div>
      <div className="mt-4 space-y-3">
        {items.map((entry) => (
          <div key={entry.id} className="rounded-2xl border border-zinc-200 p-4">
            <button type="button" onClick={() => onOpen(entry.item.id)} className="text-left">
              <div className="text-sm font-semibold text-zinc-900">{entry.item.translated_title || entry.item.title || entry.item.url}</div>
              <div className="mt-1 text-xs text-zinc-500">{(entry.reason_labels ?? []).join(" · ")}</div>
            </button>
            <div className="mt-3 flex gap-2">
              <button type="button" onClick={() => onDone(entry.id)} className="rounded-lg bg-zinc-900 px-3 py-2 text-xs font-medium text-white">
                {t("reviewQueue.done")}
              </button>
              <button type="button" onClick={() => onSnooze(entry.id)} className="rounded-lg border border-zinc-200 px-3 py-2 text-xs font-medium text-zinc-700">
                {t("reviewQueue.snooze")}
              </button>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
