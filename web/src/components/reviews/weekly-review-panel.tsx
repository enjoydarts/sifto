"use client";

import Link from "next/link";
import { WeeklyReviewSnapshot } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

interface WeeklyReviewPanelProps {
  review: WeeklyReviewSnapshot | null;
}

export function WeeklyReviewPanel({ review }: WeeklyReviewPanelProps) {
  const { t } = useI18n();
  if (!review) return null;
  return (
    <section className="rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-400">{t("weeklyReview.eyebrow")}</p>
          <h2 className="mt-2 text-lg font-semibold text-zinc-950">{t("weeklyReview.title")}</h2>
          <p className="mt-1 text-sm text-zinc-500">
            {review.week_start} - {review.week_end}
          </p>
        </div>
        <Link href="/digests" className="text-sm text-zinc-600 hover:text-zinc-900">
          {t("weeklyReview.openDigests")}
        </Link>
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-4">
        <div className="rounded-2xl bg-zinc-50 p-3 text-sm"><div className="text-zinc-500">{t("weeklyReview.reads")}</div><div className="mt-1 text-xl font-semibold text-zinc-950">{review.read_count}</div></div>
        <div className="rounded-2xl bg-zinc-50 p-3 text-sm"><div className="text-zinc-500">{t("weeklyReview.notes")}</div><div className="mt-1 text-xl font-semibold text-zinc-950">{review.note_count}</div></div>
        <div className="rounded-2xl bg-zinc-50 p-3 text-sm"><div className="text-zinc-500">{t("weeklyReview.insights")}</div><div className="mt-1 text-xl font-semibold text-zinc-950">{review.insight_count}</div></div>
        <div className="rounded-2xl bg-zinc-50 p-3 text-sm"><div className="text-zinc-500">{t("weeklyReview.favorites")}</div><div className="mt-1 text-xl font-semibold text-zinc-950">{review.favorite_count}</div></div>
      </div>
      {review.dominant_topics && review.dominant_topics.length > 0 ? (
        <div className="mt-4">
          <p className="text-sm font-medium text-zinc-800">{t("weeklyReview.topics")}</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {review.dominant_topics.map((topic) => (
              <span key={topic.topic} className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs text-zinc-700">
                {topic.topic} · {topic.count}
              </span>
            ))}
          </div>
        </div>
      ) : null}
    </section>
  );
}
