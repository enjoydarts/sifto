"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { AlignLeft, FileText, Link2, ListChecks, Sparkles, Star, ThumbsDown, ThumbsUp } from "lucide-react";
import { api, ItemDetail, RelatedItem } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

const STATUS_COLOR: Record<string, string> = {
  new: "bg-zinc-100 text-zinc-600",
  fetched: "bg-blue-50 text-blue-600",
  facts_extracted: "bg-purple-50 text-purple-600",
  summarized: "bg-green-50 text-green-700",
  failed: "bg-red-50 text-red-600",
};

export default function ItemDetailPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const { id } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const [item, setItem] = useState<ItemDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [readUpdating, setReadUpdating] = useState(false);
  const [feedbackUpdating, setFeedbackUpdating] = useState(false);
  const [related, setRelated] = useState<RelatedItem[]>([]);
  const [relatedClusters, setRelatedClusters] = useState<
    { id: string; label: string; size: number; max_similarity: number; representative: RelatedItem; items: RelatedItem[] }[]
  >([]);
  const [expandedRelatedClusterIds, setExpandedRelatedClusterIds] = useState<Record<string, boolean>>({});
  const [relatedSortMode, setRelatedSortMode] = useState<"similarity" | "recent">("similarity");
  const [relatedError, setRelatedError] = useState<string | null>(null);
  const autoMarkedRef = useRef<Record<string, true>>({});

  const syncItemReadInFeedCaches = useCallback((itemId: string, isRead: boolean) => {
    queryClient.setQueriesData({ queryKey: ["items-feed"] }, (prev: unknown) => {
      if (!prev || typeof prev !== "object") return prev;
      const data = prev as {
        items?: Array<Record<string, unknown>>;
        clusters?: Array<Record<string, unknown>>;
      };
      const patchItem = (v: Record<string, unknown>) => (v.id === itemId ? { ...v, is_read: isRead } : v);
      let changed = false;
      const next: Record<string, unknown> = { ...(data as Record<string, unknown>) };
      if (Array.isArray(data.items)) {
        next.items = data.items.map((v) => {
          const nv = patchItem(v);
          if (nv !== v) changed = true;
          return nv;
        });
      }
      if (Array.isArray(data.clusters)) {
        next.clusters = data.clusters.map((cluster) => {
          const c = { ...cluster } as Record<string, unknown>;
          const rep = c.representative;
          if (rep && typeof rep === "object") {
            const nr = patchItem(rep as Record<string, unknown>);
            if (nr !== rep) {
              c.representative = nr;
              changed = true;
            }
          }
          const items = c.items;
          if (Array.isArray(items)) {
            c.items = items.map((v) => {
              if (!v || typeof v !== "object") return v;
              const nv = patchItem(v as Record<string, unknown>);
              if (nv !== v) changed = true;
              return nv;
            });
          }
          return c;
        });
      }
      if (!changed) return prev;
      return {
        ...next,
      };
    });
  }, [queryClient]);

  const syncItemFeedbackInFeedCaches = useCallback(
    (itemId: string, patch: { is_favorite?: boolean; feedback_rating?: -1 | 0 | 1 | number }) => {
      queryClient.setQueriesData({ queryKey: ["items-feed"] }, (prev: unknown) => {
        if (!prev || typeof prev !== "object") return prev;
        const data = prev as {
          items?: Array<Record<string, unknown>>;
          clusters?: Array<Record<string, unknown>>;
        };
        const patchItem = (v: Record<string, unknown>) =>
          v.id === itemId
            ? {
                ...v,
                ...(patch.is_favorite != null ? { is_favorite: patch.is_favorite } : {}),
                ...(patch.feedback_rating != null ? { feedback_rating: patch.feedback_rating } : {}),
              }
            : v;
        let changed = false;
        const next: Record<string, unknown> = { ...(data as Record<string, unknown>) };
        if (Array.isArray(data.items)) {
          next.items = data.items.map((v) => {
            const nv = patchItem(v);
            if (nv !== v) changed = true;
            return nv;
          });
        }
        if (Array.isArray(data.clusters)) {
          next.clusters = data.clusters.map((cluster) => {
            const c = { ...cluster } as Record<string, unknown>;
            const rep = c.representative;
            if (rep && typeof rep === "object") {
              const nr = patchItem(rep as Record<string, unknown>);
              if (nr !== rep) {
                c.representative = nr;
                changed = true;
              }
            }
            const items = c.items;
            if (Array.isArray(items)) {
              c.items = items.map((v) => {
                if (!v || typeof v !== "object") return v;
                const nv = patchItem(v as Record<string, unknown>);
                if (nv !== v) changed = true;
                return nv;
              });
            }
            return c;
          });
        }
        if (!changed) return prev;
        return {
          ...next,
        };
      });
    },
    [queryClient]
  );

  useEffect(() => {
    setLoading(true);
    Promise.allSettled([api.getItem(id), api.getRelatedItems(id, { limit: 6 })])
      .then((results) => {
        const [detailRes, relatedRes] = results;
        if (detailRes.status === "rejected") {
          throw detailRes.reason;
        }
        setItem(detailRes.value);

        if (relatedRes.status === "fulfilled") {
          setRelated(relatedRes.value.items ?? []);
          setRelatedClusters(relatedRes.value.clusters ?? []);
          setExpandedRelatedClusterIds({});
          setRelatedError(null);
        } else {
          setRelated([]);
          setRelatedClusters([]);
          setExpandedRelatedClusterIds({});
          setRelatedError(String(relatedRes.reason));
        }
      })
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, [id]);

  useEffect(() => {
    if (!item || item.is_read || autoMarkedRef.current[item.id]) return;

    autoMarkedRef.current[item.id] = true;
    setReadUpdating(true);
    api
      .markItemRead(item.id)
      .then((next) => {
        syncItemReadInFeedCaches(item.id, next.is_read);
        setItem((prev) => (prev && prev.id === item.id ? { ...prev, is_read: next.is_read } : prev));
      })
      .catch(() => {
        delete autoMarkedRef.current[item.id];
      })
      .finally(() => setReadUpdating(false));
  }, [item, syncItemReadInFeedCaches]);

  const dateLocale = useMemo(() => (locale === "ja" ? "ja-JP" : "en-US"), [locale]);
  const backHref = useMemo(() => {
    const from = searchParams.get("from");
    return from && from.startsWith("/items") ? from : "/items";
  }, [searchParams]);
  const clusteredRelated = useMemo(() => {
    const clusters = relatedClusters.filter((c) => c.size >= 2).map((c) => ({
      ...c,
      items: [...c.items].sort((a, b) => {
        if (relatedSortMode === "recent") {
          return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
        }
        if (b.similarity !== a.similarity) return b.similarity - a.similarity;
        return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
      }),
    }));
    clusters.sort((a, b) => {
      if (relatedSortMode === "recent") {
        const aTime = Math.max(...a.items.map((v) => new Date(v.published_at ?? v.created_at).getTime()));
        const bTime = Math.max(...b.items.map((v) => new Date(v.published_at ?? v.created_at).getTime()));
        if (bTime !== aTime) return bTime - aTime;
      } else if (b.max_similarity !== a.max_similarity) {
        return b.max_similarity - a.max_similarity;
      }
      return b.size - a.size;
    });
    return clusters;
  }, [relatedClusters, relatedSortMode]);
  const singleRelated = useMemo(
    () =>
      relatedClusters.length > 0
        ? [...relatedClusters.filter((c) => c.size < 2).flatMap((c) => c.items)].sort((a, b) => {
            if (relatedSortMode === "recent") {
              return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
            }
            if (b.similarity !== a.similarity) return b.similarity - a.similarity;
            return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
          })
        : related,
    [related, relatedClusters, relatedSortMode]
  );

  const toggleRead = async () => {
    if (!item) return;
    setReadUpdating(true);
    try {
      const next = item.is_read ? await api.markItemUnread(item.id) : await api.markItemRead(item.id);
      syncItemReadInFeedCaches(item.id, next.is_read);
      setItem({ ...item, is_read: next.is_read });
      showToast(
        next.is_read
          ? locale === "ja"
            ? "既読にしました"
            : "Marked as read"
          : locale === "ja"
            ? "未読に戻しました"
            : "Marked as unread",
        "success"
      );
    } catch (e) {
      setError(String(e));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setReadUpdating(false);
    }
  };

  const updateFeedback = async (patch: { rating?: -1 | 0 | 1; is_favorite?: boolean }) => {
    if (!item) return;
    setFeedbackUpdating(true);
    const nextRating =
      patch.rating != null ? patch.rating : ((item.feedback?.rating ?? 0) as -1 | 0 | 1);
    const nextFavorite =
      patch.is_favorite != null ? patch.is_favorite : Boolean(item.feedback?.is_favorite ?? false);
    try {
      const next = await api.setItemFeedback(item.id, {
        rating: nextRating,
        is_favorite: nextFavorite,
      });
      syncItemFeedbackInFeedCaches(item.id, {
        is_favorite: next.is_favorite,
        feedback_rating: next.rating,
      });
      setItem((prev) => (prev ? { ...prev, feedback: next } : prev));
      showToast(locale === "ja" ? "評価を保存しました" : "Feedback saved", "success");
    } catch (e) {
      setError(String(e));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setFeedbackUpdating(false);
    }
  };

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;
  if (!item) return null;

  return (
    <div className="space-y-6">
      <Link href={backHref} className="inline-block text-sm text-zinc-500 hover:text-zinc-900">
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
          <button
            type="button"
            onClick={toggleRead}
            disabled={readUpdating}
            className="ml-auto rounded border border-zinc-300 bg-white px-3 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {readUpdating
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
        </div>

        <div className="mb-3 flex flex-wrap items-center gap-2">
          <button
            type="button"
            disabled={feedbackUpdating}
            onClick={() =>
              updateFeedback({ rating: (item.feedback?.rating ?? 0) === 1 ? 0 : 1 })
            }
            className={`inline-flex items-center gap-1 rounded border px-2.5 py-1 text-xs font-medium transition-colors ${
              (item.feedback?.rating ?? 0) === 1
                ? "border-green-200 bg-green-50 text-green-700"
                : "border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            <ThumbsUp className="size-3.5" aria-hidden="true" />
            <span>{locale === "ja" ? "良い" : "Like"}</span>
          </button>
          <button
            type="button"
            disabled={feedbackUpdating}
            onClick={() =>
              updateFeedback({ rating: (item.feedback?.rating ?? 0) === -1 ? 0 : -1 })
            }
            className={`inline-flex items-center gap-1 rounded border px-2.5 py-1 text-xs font-medium transition-colors ${
              (item.feedback?.rating ?? 0) === -1
                ? "border-rose-200 bg-rose-50 text-rose-700"
                : "border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            <ThumbsDown className="size-3.5" aria-hidden="true" />
            <span>{locale === "ja" ? "微妙" : "Dislike"}</span>
          </button>
          <button
            type="button"
            disabled={feedbackUpdating}
            onClick={() => updateFeedback({ is_favorite: !Boolean(item.feedback?.is_favorite) })}
            className={`inline-flex items-center gap-1 rounded border px-2.5 py-1 text-xs font-medium transition-colors ${
              item.feedback?.is_favorite
                ? "border-amber-200 bg-amber-50 text-amber-700"
                : "border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
            }`}
          >
            <Star className={`size-3.5 ${item.feedback?.is_favorite ? "fill-current" : ""}`} aria-hidden="true" />
            <span>{locale === "ja" ? "お気に入り" : "Favorite"}</span>
          </button>
        </div>

        <div className="mb-2 flex items-start gap-2">
          <FileText className="mt-1 size-5 shrink-0 text-zinc-500" aria-hidden="true" />
          <h1 className="text-2xl font-bold leading-snug text-zinc-900">
            {item.title ?? (locale === "ja" ? "タイトルなし" : "No title")}
          </h1>
        </div>
        <a
          href={item.url}
          target="_blank"
          rel="noopener noreferrer"
          className="block break-all text-sm text-blue-600 hover:underline"
        >
          {item.url}
        </a>

        {item.thumbnail_url && (
          <div className="mt-4 overflow-hidden rounded-xl border border-zinc-200 bg-zinc-50">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={item.thumbnail_url}
              alt=""
              loading="lazy"
              referrerPolicy="no-referrer"
              className="h-56 w-full object-cover sm:h-72"
            />
          </div>
        )}

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
            <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
              <Sparkles className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "要約" : "Summary"}
            </h2>
            {item.summary.score != null && (
              <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-700">
                score {item.summary.score.toFixed(2)}
              </span>
            )}
            {item.summary.score_policy_version && (
              <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-500">
                {item.summary.score_policy_version}
              </span>
            )}
            {item.summary_llm && (
              <span
                className="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-600"
                title={locale === "ja" ? "要約生成モデル" : "Summary generation model"}
              >
                {item.summary_llm.provider} / {item.summary_llm.model}
              </span>
            )}
          </div>
          <p className="text-base leading-8 text-zinc-900">{item.summary.summary}</p>
          {item.summary.score_reason && (
            <div className="mt-4 rounded-lg border border-zinc-200 bg-zinc-50 px-4 py-3">
              <div className="mb-1 text-xs font-semibold text-zinc-500">
                {locale === "ja" ? "スコア理由" : "Score reason"}
              </div>
              <p className="text-sm leading-6 text-zinc-700">{item.summary.score_reason}</p>
            </div>
          )}
          {item.summary.score_breakdown && (
            <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {[
                ["importance", locale === "ja" ? "重要度" : "Importance"],
                ["novelty", locale === "ja" ? "新規性" : "Novelty"],
                ["actionability", locale === "ja" ? "実用性" : "Actionability"],
                ["reliability", locale === "ja" ? "確度" : "Reliability"],
                ["relevance", locale === "ja" ? "汎用関連性" : "Relevance"],
              ].map(([key, label]) => {
                const v = item.summary?.score_breakdown?.[key as keyof NonNullable<typeof item.summary.score_breakdown>];
                if (v == null) return null;
                return (
                  <div key={key} className="rounded-lg border border-zinc-200 px-3 py-2">
                    <div className="text-xs font-medium text-zinc-500">{label}</div>
                    <div className="mt-1 flex items-center justify-between gap-2">
                      <div className="h-2 flex-1 rounded-full bg-zinc-100">
                        <div className="h-2 rounded-full bg-zinc-800" style={{ width: `${Math.max(4, v * 100)}%` }} />
                      </div>
                      <span className="w-10 text-right text-xs font-medium text-zinc-700">
                        {v.toFixed(2)}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
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
          <h2 className="mb-3 inline-flex items-center gap-2 text-sm font-semibold text-zinc-700">
            <ListChecks className="size-4 text-zinc-500" aria-hidden="true" />
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
          <h2 className="mb-3 inline-flex items-center gap-2 text-sm font-semibold text-zinc-700">
            <AlignLeft className="size-4 text-zinc-500" aria-hidden="true" />
            {locale === "ja" ? "本文" : "Content"}
          </h2>
          <div className="-mx-1 max-h-[40rem] overflow-y-auto px-1 text-[15px] leading-8 whitespace-pre-wrap text-zinc-700 sm:mx-0 sm:rounded-lg sm:border sm:border-zinc-200 sm:bg-zinc-50 sm:p-4 sm:text-sm sm:leading-relaxed">
            {item.content_text}
          </div>
        </section>
      )}

      <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="mb-3 flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-3">
            <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-700">
              <Link2 className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "関連記事" : "Related articles"}
            </h2>
            <span className="text-xs text-zinc-400">
              {clusteredRelated.length > 0
                ? `${clusteredRelated.length} ${locale === "ja" ? "クラスタ" : "clusters"} / ${related.length}`
                : related.length}
            </span>
          </div>
          <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
            <button
              type="button"
              onClick={() => setRelatedSortMode("similarity")}
              className={`rounded px-2 py-1 text-xs font-medium ${
                relatedSortMode === "similarity"
                  ? "bg-zinc-900 text-white"
                  : "text-zinc-600 hover:bg-zinc-50"
              }`}
            >
              {locale === "ja" ? "類似度順" : "Similarity"}
            </button>
            <button
              type="button"
              onClick={() => setRelatedSortMode("recent")}
              className={`rounded px-2 py-1 text-xs font-medium ${
                relatedSortMode === "recent"
                  ? "bg-zinc-900 text-white"
                  : "text-zinc-600 hover:bg-zinc-50"
              }`}
            >
              {locale === "ja" ? "新しい順" : "Recent"}
            </button>
          </div>
        </div>
        {related.length === 0 ? (
          <p className="text-sm text-zinc-500">
            {relatedError
              ? locale === "ja"
                ? "関連記事の取得に失敗しました（本文表示は継続）"
                : "Failed to load related articles (item content is still available)."
              : locale === "ja"
                ? "関連記事はまだありません（embedding未生成 or 候補なし）"
                : "No related articles yet (no embeddings or no candidates)."}
          </p>
        ) : (
          <div className="space-y-3">
            {clusteredRelated.map((c) => {
              const expanded = !!expandedRelatedClusterIds[c.id];
              const restItems = c.items.slice(1);
              return (
                  <div key={c.id} className="rounded-lg border border-zinc-200 p-3">
                    <div className="mb-2 flex flex-wrap items-center gap-2 text-xs">
                      <span className="rounded-full bg-zinc-100 px-2 py-0.5 font-medium text-zinc-700">
                        {c.label}
                      </span>
                      <span className="rounded bg-zinc-100 px-2 py-0.5 text-zinc-700">
                        {c.size} {locale === "ja" ? "件" : "items"}
                      </span>
                      <span className="rounded bg-zinc-100 px-2 py-0.5 text-zinc-700">
                        sim {c.max_similarity.toFixed(3)}
                      </span>
                      <button
                        type="button"
                        onClick={() =>
                          setExpandedRelatedClusterIds((prev) => ({ ...prev, [c.id]: !prev[c.id] }))
                        }
                        className="ml-auto rounded border border-zinc-200 bg-white px-2 py-0.5 text-zinc-600 hover:bg-zinc-50"
                      >
                        {expanded
                          ? locale === "ja"
                            ? "たたむ"
                            : "Collapse"
                          : locale === "ja"
                            ? `+${restItems.length}件を見る`
                            : `Show +${restItems.length}`}
                      </button>
                    </div>
                    <div className="space-y-3">
                      {[c.items[0], ...(expanded ? restItems : [])].map((r, idx) => (
                        <div key={r.id} className={`rounded-lg p-3 ${idx === 0 ? "bg-zinc-50" : "border border-zinc-200 bg-white"}`}>
                          <div className="mb-1 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                            <span className="rounded bg-white px-2 py-0.5 text-zinc-700 ring-1 ring-zinc-200">
                              sim {r.similarity.toFixed(3)}
                            </span>
                            {r.summary_score != null && (
                              <span className="rounded bg-white px-2 py-0.5 text-zinc-700 ring-1 ring-zinc-200">
                                score {r.summary_score.toFixed(2)}
                              </span>
                            )}
                            <span>{new Date(r.published_at ?? r.created_at).toLocaleString(dateLocale)}</span>
                          </div>
                          <Link href={`/items/${r.id}`} className="block text-sm font-semibold text-zinc-900 hover:underline">
                            {r.title ?? (locale === "ja" ? "タイトルなし" : "No title")}
                          </Link>
                          <a
                            href={r.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="mt-1 block break-all text-xs text-blue-600 hover:underline"
                          >
                            {r.url}
                          </a>
                          {r.summary && (
                            <p className="mt-2 line-clamp-3 text-sm leading-6 text-zinc-700">{r.summary}</p>
                          )}
                          {!!r.topics?.length && (
                            <div className="mt-2 flex flex-wrap gap-1.5">
                              {r.topics.slice(0, 6).map((topic) => (
                                <span key={`${r.id}-${topic}`} className="rounded-full bg-white px-2 py-0.5 text-[11px] text-zinc-700 ring-1 ring-zinc-200">
                                  {topic}
                                </span>
                              ))}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}

            {singleRelated.map((r) => (
                <div key={r.id} className="rounded-lg border border-zinc-200 p-3">
                  <div className="mb-1 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                    <span className="rounded bg-zinc-100 px-2 py-0.5 text-zinc-700">
                      sim {r.similarity.toFixed(3)}
                    </span>
                    {r.summary_score != null && (
                      <span className="rounded bg-zinc-100 px-2 py-0.5 text-zinc-700">
                        score {r.summary_score.toFixed(2)}
                      </span>
                    )}
                    <span>{new Date(r.published_at ?? r.created_at).toLocaleString(dateLocale)}</span>
                  </div>
                  <Link href={`/items/${r.id}`} className="block text-sm font-semibold text-zinc-900 hover:underline">
                    {r.title ?? (locale === "ja" ? "タイトルなし" : "No title")}
                  </Link>
                  <a
                    href={r.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mt-1 block break-all text-xs text-blue-600 hover:underline"
                  >
                    {r.url}
                  </a>
                  {r.summary && (
                    <p className="mt-2 line-clamp-3 text-sm leading-6 text-zinc-700">{r.summary}</p>
                  )}
                  {!!r.topics?.length && (
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {r.topics.slice(0, 6).map((topic) => (
                        <span key={`${r.id}-${topic}`} className="rounded-full bg-zinc-100 px-2 py-0.5 text-[11px] text-zinc-700">
                          {topic}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              ))}
          </div>
        )}
      </section>
    </div>
  );
}
