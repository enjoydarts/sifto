"use client";

import Link from "next/link";
import { FormEvent, useMemo, useState } from "react";
import { Loader2, Search } from "lucide-react";
import { api, AskResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

const EMPTY: AskResponse | null = null;
const PRESET_KEYS = [
  "ask.preset.topics",
  "ask.preset.unread",
  "ask.preset.ai",
  "ask.preset.followups",
] as const;

export default function AskPage() {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [days, setDays] = useState("30");
  const [unreadOnly, setUnreadOnly] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<AskResponse | null>(EMPTY);

  const canSubmit = query.trim().length > 1 && !loading;
  const relatedItems = useMemo(() => result?.related_items ?? [], [result]);
  const bullets = useMemo(() => result?.bullets ?? [], [result]);
  const citations = useMemo(() => result?.citations ?? [], [result]);
  const presets = useMemo(() => PRESET_KEYS.map((key) => t(key)), [t]);
  const scopeLabel = useMemo(
    () => `${days}d / ${unreadOnly ? t("ask.unreadOnly") : t("ask.allItems")} / top 8`,
    [days, unreadOnly, t]
  );

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setLoading(true);
    setError(null);
    try {
      const next = await api.ask({
        query: query.trim(),
        days: Number(days),
        unread_only: unreadOnly,
        limit: 8,
      });
      setResult(next);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-6">
      <div className="rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-zinc-900 text-white">
            <Search className="h-5 w-5" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight text-zinc-950">{t("ask.title")}</h1>
            <p className="text-sm text-zinc-500">{t("ask.subtitle")}</p>
          </div>
        </div>

        <form onSubmit={onSubmit} className="mt-5 space-y-4">
          <div className="flex flex-wrap gap-2">
            {presets.map((preset) => (
              <button
                key={preset}
                type="button"
                onClick={() => setQuery(preset)}
                className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1.5 text-sm text-zinc-700 transition hover:border-zinc-300 hover:bg-white"
              >
                {preset}
              </button>
            ))}
          </div>
          <textarea
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("ask.placeholder")}
            className="min-h-28 w-full rounded-2xl border border-zinc-200 px-4 py-3 text-sm text-zinc-900 outline-none transition focus:border-zinc-400"
          />
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex flex-wrap items-center gap-3 text-sm text-zinc-600">
              <label className="inline-flex items-center gap-2">
                <span>{t("ask.days")}</span>
                <select
                  value={days}
                  onChange={(e) => setDays(e.target.value)}
                  className="rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700"
                >
                  <option value="7">7d</option>
                  <option value="30">30d</option>
                  <option value="90">90d</option>
                </select>
              </label>
              <label className="inline-flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={unreadOnly}
                  onChange={(e) => setUnreadOnly(e.target.checked)}
                  className="h-4 w-4 rounded border-zinc-300"
                />
                <span>{t("ask.unreadOnly")}</span>
              </label>
            </div>
            <button
              type="submit"
              disabled={!canSubmit}
              className="inline-flex items-center justify-center gap-2 rounded-2xl bg-zinc-900 px-5 py-3 text-sm font-medium text-white transition hover:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
              <span>{t("ask.submit")}</span>
            </button>
          </div>
        </form>
      </div>

      {error ? (
        <div className="mt-4 rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
      ) : null}

      {result ? (
        <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1.25fr)_minmax(320px,0.75fr)]">
          <section className="rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
            <div className="flex items-center justify-between gap-3">
              <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-400">{t("ask.answerLabel")}</p>
              <p className="text-xs text-zinc-400">{scopeLabel}</p>
            </div>
            <p className="mt-3 whitespace-pre-wrap text-[15px] leading-7 text-zinc-900">{result.answer}</p>
            {bullets.length > 0 ? (
              <ul className="mt-4 space-y-2">
                {bullets.map((bullet, idx) => (
                  <li key={`${idx}-${bullet}`} className="rounded-2xl bg-zinc-50 px-4 py-3 text-sm text-zinc-700">
                    {bullet}
                  </li>
                ))}
              </ul>
            ) : null}
          </section>

          <aside className="space-y-6">
            <section className="rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
              <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-400">{t("ask.citationsLabel")}</p>
              <div className="mt-3 space-y-3">
                {citations.map((citation) => (
                  <div key={citation.item_id} className="rounded-2xl border border-zinc-200 p-4">
                    <Link href={`/items/${citation.item_id}`} className="line-clamp-2 text-sm font-semibold text-zinc-900 hover:text-zinc-700">
                      {citation.title}
                    </Link>
                    {citation.reason ? <p className="mt-2 text-sm text-zinc-600">{citation.reason}</p> : null}
                  </div>
                ))}
              </div>
            </section>

            <section className="rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
              <p className="text-xs font-medium uppercase tracking-[0.18em] text-zinc-400">{t("ask.relatedLabel")}</p>
              <div className="mt-3 space-y-3">
                {relatedItems.map((item) => (
                  <div key={item.id} className="rounded-2xl border border-zinc-200 p-4">
                    <Link href={`/items/${item.id}`} className="line-clamp-2 text-sm font-semibold text-zinc-900 hover:text-zinc-700">
                      {item.translated_title || item.title || item.url}
                    </Link>
                    <p className="mt-2 line-clamp-3 text-sm text-zinc-600">{item.summary}</p>
                  </div>
                ))}
              </div>
            </section>
          </aside>
        </div>
      ) : (
        <div className="mt-6 rounded-3xl border border-dashed border-zinc-200 bg-zinc-50 px-5 py-10 text-center text-sm text-zinc-500">
          {t("ask.empty")}
        </div>
      )}
    </div>
  );
}
