"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, ItemDetail } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

const STATUS_COLOR: Record<string, string> = {
  new: "bg-zinc-100 text-zinc-600",
  fetched: "bg-blue-50 text-blue-600",
  facts_extracted: "bg-purple-50 text-purple-600",
  summarized: "bg-green-50 text-green-700",
  failed: "bg-red-50 text-red-600",
};

export default function ItemDetailPage() {
  const { t, locale } = useI18n();
  const { id } = useParams<{ id: string }>();
  const [item, setItem] = useState<ItemDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getItem(id)
      .then(setItem)
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, [id]);

  const dateLocale = useMemo(() => (locale === "ja" ? "ja-JP" : "en-US"), [locale]);

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;
  if (!item) return null;

  return (
    <div className="space-y-6">
      <Link href="/items" className="inline-block text-sm text-zinc-500 hover:text-zinc-900">
        ← {t("nav.items")}
      </Link>

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span
            className={`rounded px-2 py-0.5 text-xs font-medium ${
              STATUS_COLOR[item.status] ?? "bg-zinc-100 text-zinc-600"
            }`}
          >
            {t(`status.${item.status}`, item.status)}
          </span>
          {item.published_at && (
            <span className="text-sm text-zinc-500">
              {new Date(item.published_at).toLocaleString(dateLocale)}
            </span>
          )}
          <span className="text-xs text-zinc-400">id: {item.id}</span>
        </div>

        <h1 className="mb-2 text-2xl font-bold leading-snug text-zinc-900">
          {item.title ?? (locale === "ja" ? "タイトルなし" : "No title")}
        </h1>
        <a
          href={item.url}
          target="_blank"
          rel="noopener noreferrer"
          className="block break-all text-sm text-blue-600 hover:underline"
        >
          {item.url}
        </a>

        <div className="mt-4 grid gap-2 text-xs text-zinc-500 sm:grid-cols-2">
          <div>
            <span className="font-medium text-zinc-600">created_at:</span>{" "}
            {new Date(item.created_at).toLocaleString(dateLocale)}
          </div>
          <div>
            <span className="font-medium text-zinc-600">updated_at:</span>{" "}
            {new Date(item.updated_at).toLocaleString(dateLocale)}
          </div>
          {item.fetched_at && (
            <div>
              <span className="font-medium text-zinc-600">fetched_at:</span>{" "}
              {new Date(item.fetched_at).toLocaleString(dateLocale)}
            </div>
          )}
          {item.summary?.summarized_at && (
            <div>
              <span className="font-medium text-zinc-600">summarized_at:</span>{" "}
              {new Date(item.summary.summarized_at).toLocaleString(dateLocale)}
            </div>
          )}
        </div>
      </section>

      {item.summary && (
        <section className="rounded-xl border border-zinc-200 bg-white p-6 shadow-sm">
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <h2 className="text-sm font-semibold text-zinc-800">
              {locale === "ja" ? "要約" : "Summary"}
            </h2>
            {item.summary.score != null && (
              <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-700">
                score {item.summary.score.toFixed(2)}
              </span>
            )}
          </div>
          <div className="rounded-lg border border-zinc-200 bg-zinc-50/60 p-4">
            <p className="text-base leading-8 text-zinc-900">{item.summary.summary}</p>
          </div>
          {item.summary.topics.length > 0 && (
            <div className="mt-4 flex flex-wrap gap-1.5">
              {item.summary.topics.map((topic) => (
                <span
                  key={topic}
                  className="rounded-full bg-zinc-100 px-2.5 py-1 text-xs text-zinc-700 ring-1 ring-zinc-200"
                >
                  {topic}
                </span>
              ))}
            </div>
          )}
        </section>
      )}

      {item.facts && item.facts.facts.length > 0 && (
        <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <h2 className="mb-3 text-sm font-semibold text-zinc-700">
            {locale === "ja" ? "事実抽出" : "Facts"}
          </h2>
          <ul className="space-y-2">
            {item.facts.facts.map((f, i) => (
              <li key={i} className="flex gap-2 rounded-lg bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
                <span className="shrink-0 text-zinc-400">{i + 1}.</span>
                <span>{f}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {item.content_text && (
        <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <h2 className="mb-3 text-sm font-semibold text-zinc-700">
            {locale === "ja" ? "本文" : "Content"}
          </h2>
          <div className="max-h-[40rem] overflow-y-auto rounded-lg border border-zinc-200 bg-zinc-50 p-4 text-sm leading-relaxed whitespace-pre-wrap text-zinc-700">
            {item.content_text}
          </div>
        </section>
      )}
    </div>
  );
}
