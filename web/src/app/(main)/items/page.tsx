"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { api, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import Pagination from "@/components/pagination";
import { useToast } from "@/components/toast-provider";

const STATUS_COLOR: Record<string, string> = {
  new: "bg-zinc-100 text-zinc-600",
  fetched: "bg-blue-50 text-blue-600",
  facts_extracted: "bg-purple-50 text-purple-600",
  summarized: "bg-green-50 text-green-700",
  failed: "bg-red-50 text-red-600",
};

const FILTERS = ["", "summarized", "new", "fetched", "facts_extracted", "failed"] as const;
type SortMode = "newest" | "score";
type FocusSize = 7 | 15 | 25;
type FocusWindow = "24h" | "today_jst" | "7d";

function scoreTone(score: number) {
  if (score >= 0.8) return "bg-green-50 text-green-700 border-green-200";
  if (score >= 0.65) return "bg-blue-50 text-blue-700 border-blue-200";
  if (score >= 0.5) return "bg-zinc-50 text-zinc-700 border-zinc-200";
  return "bg-amber-50 text-amber-700 border-amber-200";
}

export default function ItemsPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const [items, setItems] = useState<Item[]>([]);
  const [itemsTotal, setItemsTotal] = useState(0);
  const [planPoolCount, setPlanPoolCount] = useState(0);
  const [planTopics, setPlanTopics] = useState<{ label: string; count: number; maxScore: number }[]>([]);
  const [planLoading, setPlanLoading] = useState(false);
  const [filter, setFilter] = useState("");
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [page, setPage] = useState(1);
  const [sortMode, setSortMode] = useState<SortMode>("newest");
  const [focusMode, setFocusMode] = useState(true);
  const [readingPlanExpanded, setReadingPlanExpanded] = useState(false);
  const [focusSize, setFocusSize] = useState<FocusSize>(15);
  const [focusWindow, setFocusWindow] = useState<FocusWindow>("24h");
  const [diversifyTopics, setDiversifyTopics] = useState(true);
  const [excludeReadInPlan, setExcludeReadInPlan] = useState(true);
  const pageSize = 20;
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});

  const load = useCallback(async (status: string, pageNum: number, sort: SortMode) => {
    setLoading(true);
    try {
      const data = await api.getItems({
        ...(status ? { status } : {}),
        page: pageNum,
        page_size: pageSize,
        sort,
        unread_only: unreadOnly,
      });
      setItems(data?.items ?? []);
      setItemsTotal(data?.total ?? 0);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [unreadOnly]);

  const loadReadingPlan = useCallback(async () => {
    if (!focusMode) return;
    setPlanLoading(true);
    try {
      const data = await api.getReadingPlan({
        window: focusWindow,
        size: focusSize,
        diversify_topics: diversifyTopics,
        exclude_read: excludeReadInPlan,
      });
      setPlanPoolCount(data?.source_pool_count ?? 0);
      setPlanTopics(
        (data?.topics ?? []).map((t) => ({
          label:
            t.topic === "__untagged__"
              ? locale === "ja"
                ? "未分類"
                : "Other"
              : t.topic,
          count: t.count,
          maxScore: t.max_score ?? -1,
        }))
      );
      // Reuse items state only while focus mode is on; keeps rendering path simple.
      setItems(data?.items ?? []);
      setItemsTotal(data?.items?.length ?? 0);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setPlanLoading(false);
    }
  }, [excludeReadInPlan, diversifyTopics, focusMode, focusSize, focusWindow, locale]);

  useEffect(() => {
    setPage(1);
  }, [filter, sortMode, unreadOnly]);

  useEffect(() => {
    if (focusMode) setSortMode("score");
    setPage(1);
  }, [focusMode, focusSize, focusWindow, diversifyTopics, excludeReadInPlan]);

  useEffect(() => {
    if (focusMode) {
      loadReadingPlan();
      return;
    }
    load(filter, page, sortMode);
  }, [focusMode, loadReadingPlan, load, filter, page, sortMode]);

  const retryItem = useCallback(
    async (itemId: string) => {
      setRetryingIds((prev) => ({ ...prev, [itemId]: true }));
      try {
        await api.retryItem(itemId);
        showToast(locale === "ja" ? "再試行をキュー投入しました" : "Retry queued", "success");
        if (focusMode) {
          await loadReadingPlan();
        } else {
          await load(filter, page, sortMode);
        }
      } catch (e) {
        setError(String(e));
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setRetryingIds((prev) => {
          const next = { ...prev };
          delete next[itemId];
          return next;
        });
      }
    },
    [filter, focusMode, load, loadReadingPlan, locale, page, showToast, sortMode, t]
  );

  const toggleRead = useCallback(
    async (item: Item) => {
      setReadUpdatingIds((prev) => ({ ...prev, [item.id]: true }));
      try {
        if (item.is_read) {
          await api.markItemUnread(item.id);
          showToast(locale === "ja" ? "未読に戻しました" : "Marked as unread", "success");
        } else {
          await api.markItemRead(item.id);
          showToast(locale === "ja" ? "既読にしました" : "Marked as read", "success");
        }
        setItems((prev) =>
          prev.map((v) => (v.id === item.id ? { ...v, is_read: !item.is_read } : v))
        );
      } catch (e) {
        setError(String(e));
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setReadUpdatingIds((prev) => {
          const next = { ...prev };
          delete next[item.id];
          return next;
        });
      }
    },
    [locale, showToast, t]
  );

  const sortedItems = [...items].sort((a, b) => {
    if (sortMode === "score") {
      const as = a.summary_score ?? -1;
      const bs = b.summary_score ?? -1;
      if (bs !== as) return bs - as;
    }
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });

  const summarizedRanked = sortedItems.filter((i) => i.status === "summarized");
  const now = new Date();
  const nowMs = now.getTime();
  const todayJstKey = new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Tokyo",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(now);
  const itemTimeMs = (item: Item) => new Date(item.published_at ?? item.created_at).getTime();
  const itemJstDateKey = (item: Item) =>
    new Intl.DateTimeFormat("en-CA", {
      timeZone: "Asia/Tokyo",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date(item.published_at ?? item.created_at));
  const inFocusWindow = (item: Item) => {
    const tMs = itemTimeMs(item);
    switch (focusWindow) {
      case "24h":
        return nowMs-tMs <= 24 * 60 * 60 * 1000 && nowMs >= tMs;
      case "today_jst":
        return itemJstDateKey(item) === todayJstKey;
      case "7d":
        return nowMs-tMs <= 7 * 24 * 60 * 60 * 1000 && nowMs >= tMs;
      default:
        return true;
    }
  };
  const focusSourceItems = summarizedRanked.filter((item) => {
    if (!inFocusWindow(item)) return false;
    if (excludeReadInPlan && item.is_read) return false;
    return true;
  });

  const focusCandidates = (sortMode === "score" ? focusSourceItems : [...focusSourceItems].sort((a, b) => {
    const as = a.summary_score ?? -1;
    const bs = b.summary_score ?? -1;
    if (bs !== as) return bs - as;
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  }));

  const topicKey = (item: Item) => {
    const topics = item.summary_topics ?? [];
    const first = topics.find((t) => (t ?? "").trim() !== "");
    return (first ?? "__untagged__").trim().toLowerCase();
  };

  const focusItems: Item[] = [];
  const seenTopics = new Set<string>();
  for (const item of focusCandidates) {
    if (focusItems.length >= focusSize) break;
    if (!diversifyTopics) {
      focusItems.push(item);
      continue;
    }
    const key = topicKey(item);
    if (seenTopics.has(key)) continue;
    seenTopics.add(key);
    focusItems.push(item);
  }
  if (diversifyTopics && focusItems.length < focusSize) {
    const selected = new Set(focusItems.map((i) => i.id));
    for (const item of focusCandidates) {
      if (focusItems.length >= focusSize) break;
      if (selected.has(item.id)) continue;
      focusItems.push(item);
      selected.add(item.id);
    }
  }

  const topicSummary = (() => {
    if (focusMode && planTopics.length > 0) return planTopics;
    const m = new Map<string, { label: string; count: number; maxScore: number }>();
    for (const item of focusSourceItems) {
      const topics = item.summary_topics?.length ? item.summary_topics : [locale === "ja" ? "未分類" : "Other"];
      for (const t of topics.slice(0, 2)) {
        const label = (t || "").trim() || (locale === "ja" ? "未分類" : "Other");
        const key = label.toLowerCase();
        const cur = m.get(key) ?? { label, count: 0, maxScore: -1 };
        cur.count += 1;
        cur.maxScore = Math.max(cur.maxScore, item.summary_score ?? -1);
        m.set(key, cur);
      }
    }
    return [...m.values()]
      .sort((a, b) => b.count - a.count || b.maxScore - a.maxScore || a.label.localeCompare(b.label))
      .slice(0, 12);
  })();

  const displayItems = focusMode ? items : sortedItems;
  const pagedItems = focusMode ? displayItems : sortedItems;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">{t("items.title")}</h1>
          <p className="mt-1 text-sm text-zinc-500">
            {(focusMode ? displayItems.length : itemsTotal).toLocaleString()} {t("common.rows")}
            {focusMode && (
              <span className="ml-2 text-zinc-400">
                {locale === "ja"
                  ? `（対象 ${planPoolCount.toLocaleString()} 件から厳選）`
                  : `(selected from ${planPoolCount.toLocaleString()} items in window)`}
              </span>
            )}
          </p>
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
          <button
            type="button"
            onClick={() => setSortMode("newest")}
            className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
              sortMode === "newest"
                ? "bg-zinc-900 text-white"
                : "text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            {locale === "ja" ? "新着順" : "Newest"}
          </button>
          <button
            type="button"
            onClick={() => setSortMode("score")}
            className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
              sortMode === "score"
                ? "bg-zinc-900 text-white"
                : "text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            {locale === "ja" ? "スコア順" : "Score"}
          </button>
        </div>
      </div>

      <section className="rounded-2xl border border-zinc-200 bg-gradient-to-br from-white to-zinc-50 p-4 shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="text-xs font-semibold uppercase tracking-[0.12em] text-zinc-500">
              {locale === "ja" ? "Reading Plan" : "Reading Plan"}
            </div>
            <div className="mt-1 text-sm font-semibold text-zinc-900">
              {locale === "ja" ? "先に読むべき記事を先頭にまとめる" : "Bring the most useful reads to the top"}
            </div>
            <div className="mt-1 text-xs text-zinc-500">
              {locale === "ja"
                ? "スコア・トピック分散・対象期間を使って、読み切れる量に圧縮します。"
                : "Compress the list into a readable set using score, topic diversity, and time window."}
            </div>
            <div className="mt-2 text-xs text-zinc-500">
              {locale === "ja"
                ? `対象: ${focusWindow === "24h" ? "過去24時間" : focusWindow === "today_jst" ? "今日(JST)" : "過去7日"} / ${
                    focusSize === 7 ? "クイック" : focusSize === 15 ? "標準" : "しっかり"
                  } / ${diversifyTopics ? "トピック分散" : "スコア優先"} / ${excludeReadInPlan ? "未読優先" : "既読含む"}`
                : `Window: ${
                    focusWindow === "24h" ? "Last 24h" : focusWindow === "today_jst" ? "Today (JST)" : "Last 7d"
                  } / ${focusSize === 7 ? "Quick" : focusSize === 15 ? "Standard" : "Deep"} / ${
                    diversifyTopics ? "Diversified" : "Score-first"
                  } / ${
                    excludeReadInPlan ? "Unread first" : "Include read"
                  }`}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setReadingPlanExpanded((v) => !v)}
              className="rounded-full border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-700 shadow-sm hover:bg-zinc-50"
            >
              {readingPlanExpanded
                ? locale === "ja"
                  ? "折りたたむ"
                  : "Collapse"
                : locale === "ja"
                  ? "設定を開く"
                  : "Open settings"}
            </button>
            <label className="inline-flex items-center gap-2 rounded-full border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-700 shadow-sm">
              <input
                type="checkbox"
                checked={focusMode}
                onChange={(e) => setFocusMode(e.target.checked)}
                className="size-4 rounded border-zinc-300"
              />
              {focusMode ? (locale === "ja" ? "読書プランON" : "Reading plan ON") : locale === "ja" ? "一覧そのまま" : "Raw list"}
            </label>
          </div>
        </div>
        {readingPlanExpanded && (
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          <div className="rounded-xl border border-zinc-200 bg-white p-3">
            <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-zinc-500">
              {locale === "ja" ? "読む量" : "Reading budget"}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {([7, 15, 25] as FocusSize[]).map((n) => (
                <button
                  key={n}
                  type="button"
                  onClick={() => setFocusSize(n)}
                  className={`rounded-full px-3 py-1.5 text-xs font-medium ${
                    focusSize === n ? "bg-zinc-900 text-white" : "border border-zinc-200 text-zinc-600 hover:bg-zinc-50"
                  }`}
                >
                  {locale === "ja"
                    ? n === 7
                      ? "クイック"
                      : n === 15
                        ? "標準"
                        : "しっかり"
                    : n === 7
                      ? "Quick"
                      : n === 15
                        ? "Standard"
                        : "Deep"}
                </button>
              ))}
            </div>
          </div>
          <div className="rounded-xl border border-zinc-200 bg-white p-3">
            <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-zinc-500">
              {locale === "ja" ? "対象期間" : "Window"}
            </div>
            <div className="flex flex-wrap gap-2">
              {([
                ["24h", locale === "ja" ? "過去24時間" : "Last 24h"],
                ["today_jst", locale === "ja" ? "今日(JST)" : "Today (JST)"],
                ["7d", locale === "ja" ? "過去7日" : "Last 7d"],
              ] as [FocusWindow, string][]).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setFocusWindow(value)}
                  className={`rounded-full px-3 py-1.5 text-xs font-medium ${
                    focusWindow === value ? "bg-zinc-900 text-white" : "border border-zinc-200 text-zinc-600 hover:bg-zinc-50"
                  }`}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
          <div className="rounded-xl border border-zinc-200 bg-white p-3">
            <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-zinc-500">
              {locale === "ja" ? "選び方" : "Selection"}
            </div>
            <div className="space-y-2">
              <label className="inline-flex items-center gap-2 text-xs text-zinc-700">
                <input
                  type="checkbox"
                  checked={diversifyTopics}
                  onChange={(e) => setDiversifyTopics(e.target.checked)}
                  className="size-3.5 rounded border-zinc-300"
                />
                {locale === "ja" ? "トピックを散らして偏りを減らす" : "Reduce topic duplication"}
              </label>
              <label className="inline-flex items-center gap-2 text-xs text-zinc-700">
                <input
                  type="checkbox"
                  checked={excludeReadInPlan}
                  onChange={(e) => setExcludeReadInPlan(e.target.checked)}
                  className="size-3.5 rounded border-zinc-300"
                />
                {locale === "ja" ? "既読を除外して未読を優先" : "Prioritize unread (exclude read)"}
              </label>
            </div>
          </div>
        </div>
        )}
        {!focusMode && (
          <div className="mt-3 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-xs text-zinc-500">
            {locale === "ja"
              ? "読書プランをOFFにしているため、通常の一覧表示です。"
              : "Reading plan is off. Showing the regular list."}
          </div>
        )}
        {readingPlanExpanded && topicSummary.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-1.5">
            {topicSummary.map((topic) => (
              <span key={topic.label} className="rounded-full border border-zinc-200 bg-white px-2.5 py-1 text-xs text-zinc-700 shadow-sm">
                {topic.label} · {topic.count}
              </span>
            ))}
          </div>
        )}
      </section>

      {/* Filter tabs */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        {FILTERS.map((value) => (
          <button
            key={value}
            onClick={() => setFilter(value)}
            className={`rounded px-3 py-1 text-sm font-medium transition-colors ${
              filter === value
                ? "bg-zinc-900 text-white"
                : "border border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            {t(`items.filter.${value || "all"}`)}
          </button>
        ))}
        {!focusMode && (
          <label className="ml-auto inline-flex items-center gap-2 rounded border border-zinc-200 bg-white px-3 py-1 text-sm text-zinc-700">
            <input
              type="checkbox"
              checked={unreadOnly}
              onChange={(e) => setUnreadOnly(e.target.checked)}
              className="size-4 rounded border-zinc-300"
            />
            {locale === "ja" ? "未読のみ" : "Unread only"}
          </label>
        )}
      </div>

      {/* State */}
      {(loading || planLoading) && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}
      {!loading && !planLoading && items.length === 0 && (
        <p className="text-sm text-zinc-400">{t("items.empty")}</p>
      )}

      {/* List */}
	      <ul className="space-y-2">
        {pagedItems.map((item) => (
	          <li key={item.id}>
	            <div className={`flex items-start gap-3 rounded-xl border px-4 py-3 shadow-sm ${
                item.is_read ? "border-zinc-200 bg-zinc-50/70" : "border-zinc-200 bg-white"
              }`}>
	              <Link
	                href={`/items/${item.id}`}
	                className="flex min-w-0 flex-1 items-start gap-3 transition-colors hover:text-zinc-700"
	              >
	                <span
	                  className={`mt-0.5 shrink-0 rounded px-2 py-0.5 text-xs font-medium ${
	                    STATUS_COLOR[item.status] ?? "bg-zinc-100 text-zinc-600"
	                  }`}
	                >
	                  {t(`status.${item.status}`, item.status)}
	                </span>
	                <div className="min-w-0 flex-1">
	                  <div className="flex items-start gap-2">
	                    <div className={`min-w-0 flex-1 truncate text-sm font-medium ${item.is_read ? "text-zinc-600" : "text-zinc-900"}`}>
	                      {item.title ?? item.url}
	                    </div>
	                    {item.summary_score != null ? (
	                      <span
	                        className={`shrink-0 rounded border px-2 py-0.5 text-xs font-semibold ${scoreTone(item.summary_score)}`}
	                        title={locale === "ja" ? "要約スコア" : "Summary score"}
	                      >
	                        {item.summary_score.toFixed(2)}
	                      </span>
	                    ) : (
	                      <span
	                        className="shrink-0 rounded border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-xs font-medium text-zinc-400"
	                        title={locale === "ja" ? "未採点" : "Not scored"}
	                      >
	                        {locale === "ja" ? "未採点" : "N/A"}
	                      </span>
	                    )}
	                  </div>
	                  {item.title && (
	                    <div className="truncate text-xs text-zinc-400">
	                      {item.url}
	                    </div>
	                  )}
	                  <div className="mt-1 flex items-center gap-2">
	                    <div className="h-1.5 w-24 rounded-full bg-zinc-100">
	                      {item.summary_score != null && (
	                        <div
	                          className="h-1.5 rounded-full bg-zinc-800"
	                          style={{ width: `${Math.max(4, item.summary_score * 100)}%` }}
	                        />
	                      )}
	                    </div>
	                    <span className="text-[11px] text-zinc-500">
	                      {item.summary_score != null
	                        ? locale === "ja"
	                          ? "スコア"
	                          : "Score"
	                        : locale === "ja"
	                          ? "未採点"
	                          : "Not scored"}
	                    </span>
	                  </div>
	                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-zinc-400">
                      {item.is_read && (
                        <span className="rounded-full border border-zinc-200 bg-white px-2 py-0.5 text-[11px] font-medium text-zinc-500">
                          {locale === "ja" ? "既読" : "Read"}
                        </span>
                      )}
                      <span>
	                    {new Date(
	                      item.published_at ?? item.created_at
	                    ).toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US")}
                      </span>
	                  </div>
	                </div>
	              </Link>
                <div className="flex shrink-0 flex-col items-end gap-2">
                  <button
                    type="button"
                    disabled={!!readUpdatingIds[item.id]}
                    onClick={() => toggleRead(item)}
                    className="rounded border border-zinc-300 px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {readUpdatingIds[item.id]
                      ? locale === "ja"
                        ? "更新中..."
                        : "Updating..."
                      : item.is_read
                        ? locale === "ja"
                          ? "未読に戻す"
                          : "Mark unread"
                        : locale === "ja"
                          ? "既読にする"
                          : "Mark read"}
                  </button>
	                {item.status === "failed" && (
	                  <button
	                    type="button"
	                    disabled={!!retryingIds[item.id]}
	                    onClick={() => retryItem(item.id)}
	                    className="rounded border border-zinc-300 px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
	                  >
	                    {retryingIds[item.id] ? t("items.retrying") : t("items.retry")}
	                  </button>
	                )}
                </div>
	            </div>
	          </li>
	        ))}
	      </ul>
      {!focusMode && (
        <Pagination total={itemsTotal} page={page} pageSize={pageSize} onPageChange={setPage} />
      )}
    </div>
  );
}
