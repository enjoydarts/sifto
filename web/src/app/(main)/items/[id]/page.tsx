"use client";

import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Info, Star, ThumbsDown, ThumbsUp } from "lucide-react";
import { api, ItemDetail, ItemLLMExecutionAttempt, RelatedItem } from "@/lib/api";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import { formatModelDisplayName } from "@/lib/model-display";
import { InlineReader } from "@/components/inline-reader";
import { ItemHighlightList } from "@/components/items/item-highlight-list";
import { ItemNoteEditor } from "@/components/items/item-note-editor";
import { PersonalScoreExplainer } from "@/components/items/personal-score-explainer";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";

const STATUS_COLOR: Record<string, string> = {
  new: "border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)]",
  fetched: "border border-sky-200 bg-sky-50 text-sky-700",
  facts_extracted: "border border-amber-200 bg-amber-50 text-amber-700",
  summarized: "border border-emerald-200 bg-emerald-50 text-emerald-700",
  failed: "border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] text-[var(--color-editorial-error)]",
  deleted: "border border-[var(--color-editorial-line-strong)] bg-[#ece5db] text-[var(--color-editorial-ink-soft)]",
};

type RelatedCluster = {
  id: string;
  label: string;
  size: number;
  max_similarity: number;
  representative: RelatedItem;
  items: RelatedItem[];
};

function localizeRelatedReason(reason: string, t: (key: string, fallback?: string) => string): string {
  const trimmed = reason.trim();
  const lower = trimmed.toLowerCase();
  const sharedPrefix = "shared topics:";
  if (lower.startsWith(sharedPrefix)) {
    return `${t("itemDetail.relatedReason.sharedTopics")}: ${trimmed.slice(sharedPrefix.length).trim()}`;
  }
  if (lower === "very high semantic similarity") return t("itemDetail.relatedReason.veryHighSemantic");
  if (lower === "high semantic similarity") return t("itemDetail.relatedReason.highSemantic");
  if (lower === "semantic similarity match") return t("itemDetail.relatedReason.semanticMatch");
  return trimmed;
}

function executionStatusTone(status: string) {
  return status === "success"
    ? "bg-emerald-50 text-emerald-700 ring-emerald-200"
    : "bg-red-50 text-red-700 ring-red-200";
}

function executionPurposeLabel(attempt: ItemLLMExecutionAttempt, t: (key: string, fallback?: string) => string) {
  if (attempt.purpose === "facts_localization") return t("itemDetail.execution.purpose.factsLocalization");
  return null;
}

function hasRequestedResolvedPair(requested?: string | null, resolved?: string | null) {
  const req = (requested ?? "").trim();
  const res = (resolved ?? "").trim();
  if (req === "" || res === "") return false;
  return formatModelDisplayName(req) !== formatModelDisplayName(res);
}

function renderLLMModelDisplay(
  provider: string,
  model: string,
  requested: string | null | undefined,
  resolved: string | null | undefined,
  t: (key: string, fallback?: string) => string
) {
  if (!hasRequestedResolvedPair(requested, resolved)) {
    return <>{provider} / {formatModelDisplayName(model)}</>;
  }
  return (
    <>
      {provider} / {t("itemDetail.model.requested")}: {formatModelDisplayName(requested ?? "")} /{" "}
      {t("itemDetail.model.resolved")}: {formatModelDisplayName(resolved ?? "")}
    </>
  );
}

function extractHttpStatus(error: unknown): number | null {
  const message = error instanceof Error ? error.message : String(error);
  const match = message.match(/^(\d{3}):/);
  return match ? Number(match[1]) : null;
}

function localizeActionError(
  error: unknown,
  action:
    | "markRead"
    | "feedback"
    | "delete"
    | "retry"
    | "retryFromFacts"
    | "saveNote"
    | "createHighlight"
    | "deleteHighlight",
  t: (key: string, fallback?: string) => string
): string {
  const status = extractHttpStatus(error);
  if (status === 404) return t("itemDetail.actionError.notFound");
  if (status === 409) {
    switch (action) {
      case "retry":
        return t("itemDetail.actionError.retryUnavailable");
      case "retryFromFacts":
        return t("itemDetail.actionError.retryFromFactsUnavailable");
      case "delete":
      case "markRead":
      case "feedback":
      case "saveNote":
      case "createHighlight":
      case "deleteHighlight":
        return t("itemDetail.actionError.deletedReadonly");
    }
  }
  return error instanceof Error ? error.message : String(error);
}

function formatHeroDate(value: string): string {
  const date = new Date(value);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day} ${hours}:${minutes}`;
}

function ExecutionTimeline({
  attempts,
  title,
  t,
  locale,
}: {
  attempts?: ItemLLMExecutionAttempt[];
  title: string;
  t: (key: string, fallback?: string) => string;
  locale: string;
}) {
  if (!attempts || attempts.length === 0) return null;
  const dateLocale = locale === "ja" ? "ja-JP" : "en-US";
  return (
    <div className="mt-6 rounded-[18px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(245,240,233,0.78),rgba(255,255,255,0.92))] px-4 py-3">
      <div className="mb-1 text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{title}</div>
      <p className="mb-3 text-xs leading-5 text-[var(--color-editorial-ink-faint)]">{t("itemDetail.execution.help")}</p>
      <ol className="space-y-2">
        {attempts.map((attempt, index) => (
          <li
            key={`${attempt.model}-${attempt.created_at}-${index}`}
            className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5"
          >
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <span className={`rounded px-2 py-1 ring-1 ${executionStatusTone(attempt.status)}`}>
                {attempt.status === "success" ? t("itemDetail.execution.success") : t("itemDetail.execution.failure")}
              </span>
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-1 text-[var(--color-editorial-ink-soft)]">
                {t("itemDetail.execution.attempt").replace("{{attempt}}", String(attempt.attempt_index + 1))}
              </span>
              {executionPurposeLabel(attempt, t) ? (
                <span className="rounded-full border border-amber-200 bg-amber-50 px-2 py-1 text-amber-700">
                  {executionPurposeLabel(attempt, t)}
                </span>
              ) : null}
              <span className="font-medium text-[var(--color-editorial-ink-soft)]">
                {attempt.provider} / {formatModelDisplayName(attempt.model)}
              </span>
              <span className="text-[var(--color-editorial-ink-faint)]">
                {new Date(attempt.created_at).toLocaleString(dateLocale)}
              </span>
            </div>
            {attempt.error_message ? (
              <p className="mt-2 break-words text-xs leading-5 text-[var(--color-editorial-ink-soft)]">{attempt.error_message}</p>
            ) : null}
          </li>
        ))}
      </ol>
    </div>
  );
}

function DetailInfoBox({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="mt-5 rounded-[18px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(245,240,233,0.78),rgba(255,255,255,0.9))] px-4 py-4">
      <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
        {title}
      </h3>
      {children}
    </div>
  );
}

export default function ItemDetailPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const queryClient = useQueryClient();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const [item, setItem] = useState<ItemDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [readUpdating, setReadUpdating] = useState(false);
  const [deleteUpdating, setDeleteUpdating] = useState(false);
  const [restoreUpdating, setRestoreUpdating] = useState(false);
  const [feedbackUpdating, setFeedbackUpdating] = useState(false);
  const [retryUpdating, setRetryUpdating] = useState(false);
  const [retryFromFactsUpdating, setRetryFromFactsUpdating] = useState(false);
  const [related, setRelated] = useState<RelatedItem[]>([]);
  const [relatedClusters, setRelatedClusters] = useState<RelatedCluster[]>([]);
  const [expandedRelatedClusterIds, setExpandedRelatedClusterIds] = useState<Record<string, boolean>>({});
  const [relatedSortMode, setRelatedSortMode] = useState<"similarity" | "recent">("similarity");
  const [detailTab, setDetailTab] = useState<"summary" | "facts" | "body" | "related" | "notes">("summary");
  const [relatedError, setRelatedError] = useState<string | null>(null);
  const [nextItemHref, setNextItemHref] = useState<string | null>(null);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [itemNavigator, setItemNavigator] = useState<Awaited<ReturnType<typeof api.getItemNavigator>>["navigator"] | null>(null);
  const [itemNavigatorLoading, setItemNavigatorLoading] = useState(false);
  const [itemNavigatorError, setItemNavigatorError] = useState<string | null>(null);
  const [itemNavigatorOpen, setItemNavigatorOpen] = useState(false);
  const autoMarkedRef = useRef<Record<string, true>>({});
  const readStateOverrideRef = useRef<Record<string, boolean>>({});
  const settingsQuery = useQuery({
    queryKey: ["settings"] as const,
    queryFn: () => api.getSettings(),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const itemNavigatorLoadingPersona = settingsQuery.data?.llm_models?.navigator_persona?.trim() || "editor";
  const itemNavigatorDisplayPersona = itemNavigator?.avatar_style || itemNavigator?.persona || itemNavigatorLoadingPersona;

  const applyReadOverride = useCallback((nextItem: ItemDetail): ItemDetail => {
    const override = readStateOverrideRef.current[nextItem.id];
    if (override == null) return nextItem;
    return { ...nextItem, is_read: override };
  }, []);

  const refreshReadDependentQueries = useCallback(async () => {
    await Promise.allSettled([
      queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
      queryClient.invalidateQueries({ queryKey: ["focus-queue"] }),
      queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
    ]);
  }, [queryClient]);

  const refreshItemQueries = useCallback(
    async (itemId: string) => {
      await Promise.allSettled([
        queryClient.invalidateQueries({ queryKey: ["item-detail", itemId] }),
        queryClient.invalidateQueries({ queryKey: ["item-related", itemId, 6] }),
        queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
        queryClient.invalidateQueries({ queryKey: ["focus-queue"] }),
        queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
        queryClient.invalidateQueries({ queryKey: ["dashboard"] }),
      ]);
    },
    [queryClient]
  );

  const syncItemReadInFeedCaches = useCallback((itemId: string, isRead: boolean) => {
    queryClient.setQueriesData({ queryKey: ["items-feed"] }, (prev: unknown) => {
      if (!prev || typeof prev !== "object") return prev;
      const data = prev as {
        items?: Array<Record<string, unknown>>;
        planClusters?: Array<Record<string, unknown>>;
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
      if (Array.isArray(data.planClusters)) {
        next.planClusters = data.planClusters.map((cluster) => {
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
        planClusters?: Array<Record<string, unknown>>;
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
        if (Array.isArray(data.planClusters)) {
          next.planClusters = data.planClusters.map((cluster) => {
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

  const removeItemFromFeedCaches = useCallback((itemId: string) => {
    queryClient.setQueriesData({ queryKey: ["items-feed"] }, (prev: unknown) => {
      if (!prev || typeof prev !== "object") return prev;
      const data = prev as {
        items?: Array<Record<string, unknown>>;
        total?: number;
        planClusters?: Array<Record<string, unknown>>;
      };
      let changed = false;
      let removedFromItems = false;
      const next: Record<string, unknown> = { ...(data as Record<string, unknown>) };
      if (Array.isArray(data.items)) {
        const filtered = data.items.filter((v) => v.id !== itemId);
        if (filtered.length !== data.items.length) {
          next.items = filtered;
          changed = true;
          removedFromItems = true;
        }
      }
      if (Array.isArray(data.planClusters)) {
        const clusters = data.planClusters
          .map((cluster) => {
            const c = { ...cluster } as Record<string, unknown>;
            const items = c.items;
            if (Array.isArray(items)) {
              const filtered = items.filter((v) => v && typeof v === "object" && (v as Record<string, unknown>).id !== itemId);
              if (filtered.length !== items.length) {
                c.items = filtered;
                changed = true;
              }
              const rep = c.representative;
              if (rep && typeof rep === "object") {
                const repID = (rep as Record<string, unknown>).id;
                if (repID === itemId) {
                  c.representative = filtered.length > 0 ? filtered[0] : null;
                }
              }
              c.size = filtered.length;
            }
            return c;
          })
          .filter((c) => {
            const items = c.items;
            return Array.isArray(items) ? items.length > 0 : true;
          });
        next.planClusters = clusters;
      }
      if (typeof data.total === "number" && removedFromItems) {
        next.total = Math.max(0, data.total - 1);
      }
      return changed ? next : prev;
    });
  }, [queryClient]);

  useEffect(() => {
    setItemNavigator(null);
    setItemNavigatorLoading(false);
    setItemNavigatorError(null);
    setItemNavigatorOpen(false);
  }, [id]);

  useEffect(() => {
    const cachedItem = queryClient.getQueryData<ItemDetail>(["item-detail", id]);
    const cachedRelated = queryClient.getQueryData<{ items?: RelatedItem[]; clusters?: RelatedCluster[] }>([
      "item-related",
      id,
      6,
    ]);
    if (cachedItem) {
      setItem(applyReadOverride(cachedItem));
      setLoadError(null);
      setActionError(null);
      setLoading(false);
    } else {
      setLoading(true);
    }
    if (cachedRelated) {
      setRelated(cachedRelated.items ?? []);
      setRelatedClusters(cachedRelated.clusters ?? []);
      setExpandedRelatedClusterIds({});
      setRelatedError(null);
    }
    let cancelled = false;
    Promise.allSettled([api.getItem(id), api.getRelatedItems(id, { limit: 6 })])
      .then((results) => {
        if (cancelled) return;
        const [detailRes, relatedRes] = results;
        if (detailRes.status === "rejected") {
          if (!cachedItem) throw detailRes.reason;
        } else {
          const nextItem = applyReadOverride(detailRes.value);
          queryClient.setQueryData(["item-detail", id], nextItem);
          setItem(nextItem);
          setLoadError(null);
          setActionError(null);
        }

        if (relatedRes.status === "fulfilled") {
          queryClient.setQueryData(["item-related", id, 6], relatedRes.value);
          setRelated(relatedRes.value.items ?? []);
          setRelatedClusters(relatedRes.value.clusters ?? []);
          setExpandedRelatedClusterIds({});
          setRelatedError(null);
        } else if (!cachedRelated) {
          setRelated([]);
          setRelatedClusters([]);
          setExpandedRelatedClusterIds({});
          setRelatedError(String(relatedRes.reason));
        }
      })
      .catch((e) => {
        if (!cancelled) setLoadError(String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [applyReadOverride, id, queryClient]);

  useEffect(() => {
    if (!item || item.status !== "summarized" || item.is_read || autoMarkedRef.current[item.id]) return;

    autoMarkedRef.current[item.id] = true;
    setReadUpdating(true);
    api
      .markItemRead(item.id)
      .then(async (next) => {
        readStateOverrideRef.current[item.id] = next.is_read;
        syncItemReadInFeedCaches(item.id, next.is_read);
        queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
          prev ? { ...prev, is_read: next.is_read } : prev
        );
        setItem((prev) => (prev && prev.id === item.id ? { ...prev, is_read: next.is_read } : prev));
        await refreshReadDependentQueries();
      })
      .catch(() => {
        delete autoMarkedRef.current[item.id];
      })
      .finally(() => setReadUpdating(false));
  }, [item, queryClient, refreshReadDependentQueries, syncItemReadInFeedCaches]);

  const dateLocale = useMemo(() => (locale === "ja" ? "ja-JP" : "en-US"), [locale]);
  const canMarkRead = !!item && item.status !== "deleted";
  const isDeleted = item?.status === "deleted";
  const canUseItemNavigator = Boolean(item && item.status !== "deleted" && item.summary?.summary && item.facts?.facts?.length);
  const disableMutations = Boolean(isDeleted);
  const backHref = useMemo(() => {
    const from = searchParams.get("from");
    return from && from.startsWith("/items") ? from : "/items";
  }, [searchParams]);
  const currentDetailHref = useMemo(() => {
    const nextQuery = searchParams.toString();
    return nextQuery ? `/items/${id}?${nextQuery}` : `/items/${id}`;
  }, [id, searchParams]);
  const queueStorageKey = useMemo(() => `items-queue:${backHref}`, [backHref]);
  useEffect(() => {
    try {
      const raw = sessionStorage.getItem(queueStorageKey);
      if (!raw) {
        setNextItemHref(null);
        return;
      }
      const ids = JSON.parse(raw) as string[];
      if (!Array.isArray(ids) || ids.length === 0) {
        setNextItemHref(null);
        return;
      }
      const idx = ids.findIndex((v) => v === id);
      if (idx < 0 || idx + 1 >= ids.length) {
        setNextItemHref(null);
        return;
      }
      const nextID = ids[idx + 1];
      setNextItemHref(nextID ? `/items/${nextID}?from=${encodeURIComponent(backHref)}` : null);
    } catch {
      setNextItemHref(null);
    }
  }, [backHref, id, queueStorageKey]);
  useEffect(() => {
    if (!nextItemHref) return;
    router.prefetch(nextItemHref);
    const nextId = nextItemHref.match(/\/items\/([^?]+)/)?.[1];
    if (!nextId) return;
    void queryClient.prefetchQuery({
      queryKey: ["item-detail", nextId],
      queryFn: () => api.getItem(nextId),
      staleTime: 60_000,
    });
    void queryClient.prefetchQuery({
      queryKey: ["item-related", nextId, 6],
      queryFn: () => api.getRelatedItems(nextId, { limit: 6 }),
      staleTime: 60_000,
    });
  }, [nextItemHref, queryClient, router]);
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
  const openInlineRelatedItem = useCallback((relatedItemId: string) => {
    setInlineItemId(relatedItemId);
  }, []);
  const openItemDetailFromInlineReader = useCallback(
    (nextId: string) => {
      router.push(`/items/${nextId}?from=${encodeURIComponent(currentDetailHref)}`);
    },
    [currentDetailHref, router]
  );

  const openItemNavigator = useCallback(async () => {
    if (!item || !canUseItemNavigator || itemNavigatorLoading) return;
    if (itemNavigator) {
      setItemNavigatorOpen(true);
      return;
    }
    setItemNavigatorLoading(true);
    setItemNavigatorError(null);
    try {
      const preview = searchParams.get("navigator_preview") === "1";
      const res = await api.getItemNavigator(item.id, preview ? { cache_bust: true, navigator_preview: true } : undefined);
      setItemNavigator(res.navigator ?? null);
      if (!res.navigator) {
        setItemNavigatorError(t("itemDetail.navigatorUnavailable"));
        return;
      }
      setItemNavigatorOpen(true);
    } catch (error) {
      setItemNavigatorError(t("itemDetail.navigatorError"));
      showToast(`${t("briefing.navigator.label")}: ${error instanceof Error ? error.message : String(error)}`, "error");
    } finally {
      setItemNavigatorLoading(false);
    }
  }, [canUseItemNavigator, item, itemNavigator, itemNavigatorLoading, searchParams, showToast, t]);

  const toggleRead = async () => {
    if (!item || item.status === "deleted") return;
    setReadUpdating(true);
    try {
      const next = item.is_read ? await api.markItemUnread(item.id) : await api.markItemRead(item.id);
      readStateOverrideRef.current[item.id] = next.is_read;
      syncItemReadInFeedCaches(item.id, next.is_read);
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, is_read: next.is_read } : prev
      );
      setItem({ ...item, is_read: next.is_read });
      await refreshReadDependentQueries();
      setActionError(null);
      showToast(
        next.is_read ? t("itemDetail.toast.markRead") : t("itemDetail.toast.markUnread"),
        "success"
      );
    } catch (e) {
      const message = localizeActionError(e, "markRead", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setReadUpdating(false);
    }
  };

  const updateFeedback = async (patch: { rating?: -1 | 0 | 1; is_favorite?: boolean }) => {
    if (!item || item.status === "deleted") return;
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
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, feedback: next } : prev
      );
      setActionError(null);
      showToast(t("itemDetail.toast.feedbackSaved"), "success");
    } catch (e) {
      const message = localizeActionError(e, "feedback", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setFeedbackUpdating(false);
    }
  };

  const deleteItem = async () => {
    if (!item || deleteUpdating || item.status === "deleted") return;
    const ok = await confirm({
      title: t("itemDetail.delete.title"),
      message: t("itemDetail.delete.message"),
      tone: "danger",
      confirmLabel: t("itemDetail.delete.confirm"),
      cancelLabel: t("common.cancel"),
    });
    if (!ok) return;
    setDeleteUpdating(true);
    try {
      await api.deleteItem(item.id);
      removeItemFromFeedCaches(item.id);
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) => (prev ? { ...prev, status: "deleted" } : prev));
      setItem((prev) => (prev ? { ...prev, status: "deleted" } : prev));
      setActionError(null);
      showToast(t("itemDetail.toast.deleted"), "success");
    } catch (e) {
      const message = localizeActionError(e, "delete", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setDeleteUpdating(false);
    }
  };

  const restoreItem = async () => {
    if (!item || restoreUpdating || item.status !== "deleted") return;
    setRestoreUpdating(true);
    try {
      const next = await api.restoreItem(item.id);
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], next);
      setItem(next);
      removeItemFromFeedCaches(item.id);
      await refreshItemQueries(item.id);
      setActionError(null);
      showToast(t("itemDetail.toast.restored"), "success");
    } catch (e) {
      const message = localizeActionError(e, "delete", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setRestoreUpdating(false);
    }
  };

  const retryItem = async () => {
    if (!item || retryUpdating || item.status === "deleted") return;
    setRetryUpdating(true);
    try {
      await api.retryItem(item.id);
      setActionError(null);
      showToast(t("itemDetail.toast.retryQueued"), "success");
    } catch (e) {
      const message = localizeActionError(e, "retry", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setRetryUpdating(false);
    }
  };

  const retryFromFacts = async () => {
    if (!item || retryFromFactsUpdating || item.status === "deleted") return;
    const ok = await confirm({
      title: t("itemDetail.retryFromFacts.title"),
      message: t("itemDetail.retryFromFacts.message"),
      tone: "default",
      confirmLabel: t("itemDetail.retryFromFacts.confirm"),
      cancelLabel: t("common.cancel"),
    });
    if (!ok) return;
    setRetryFromFactsUpdating(true);
    try {
      await api.retryItemFromFacts(item.id);
      const nextItem = (prev: ItemDetail): ItemDetail => ({
        ...prev,
        status: "fetched" as const,
        processing_error: null,
        facts: null,
        facts_llm: null,
        facts_check: null,
        facts_check_llm: null,
        summary: null,
        summary_llm: null,
        faithfulness: null,
        faithfulness_llm: null,
      });
      setItem((prev) => (prev ? nextItem(prev) : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) => (prev ? nextItem(prev) : prev));
      await refreshItemQueries(item.id);
      setActionError(null);
      showToast(t("itemDetail.toast.retryFromFactsQueued"), "success");
    } catch (e) {
      const message = localizeActionError(e, "retryFromFacts", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setRetryFromFactsUpdating(false);
    }
  };

  const saveNote = async (content: string) => {
    if (!item || item.status === "deleted") return;
    try {
      const next = await api.saveItemNote(item.id, { content });
      setItem((prev) => (prev ? { ...prev, note: next } : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, note: next } : prev
      );
      setActionError(null);
      showToast(t("itemNote.saved"), "success");
    } catch (e) {
      const message = localizeActionError(e, "saveNote", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    }
  };

  const createHighlight = async (input: { quote_text: string; anchor_text?: string; section?: string }) => {
    if (!item || item.status === "deleted") return;
    try {
      const next = await api.createItemHighlight(item.id, input);
      setItem((prev) => (prev ? { ...prev, highlights: [next, ...(prev.highlights ?? [])] } : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, highlights: [next, ...(prev.highlights ?? [])] } : prev
      );
      setActionError(null);
      showToast(t("itemHighlight.saved"), "success");
    } catch (e) {
      const message = localizeActionError(e, "createHighlight", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    }
  };

  const deleteHighlight = async (highlightId: string) => {
    if (!item || item.status === "deleted") return;
    try {
      await api.deleteItemHighlight(item.id, highlightId);
      setItem((prev) =>
        prev ? { ...prev, highlights: (prev.highlights ?? []).filter((highlight) => highlight.id !== highlightId) } : prev
      );
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, highlights: (prev.highlights ?? []).filter((highlight) => highlight.id !== highlightId) } : prev
      );
      setActionError(null);
      showToast(t("itemHighlight.deleted"), "success");
    } catch (e) {
      const message = localizeActionError(e, "deleteHighlight", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    }
  };

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (loadError) return <p className="text-sm text-red-500">{loadError}</p>;
  if (!item) return null;

  const translatedTitle = item.translated_title?.trim() || item.summary?.translated_title?.trim() || "";
  const originalTitle = item.title?.trim() ?? "";
  const displayTitle = translatedTitle || originalTitle || t("itemDetail.noTitle");
  const showOriginalTitle = Boolean(translatedTitle && originalTitle && translatedTitle !== originalTitle);
  return (
    <div className="space-y-6 overflow-x-hidden">
      <Link href={backHref} className="inline-block text-sm text-zinc-500 hover:text-zinc-900">
        ← {t("nav.items")}
      </Link>

      <section className="overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.72),rgba(255,255,255,0.94)),#fbf8f2] shadow-[var(--shadow-card)]">
        <div className={`grid gap-0 ${item.thumbnail_url ? "lg:grid-cols-[minmax(0,1.22fr)_minmax(320px,0.9fr)]" : ""}`}>
          <div className={`min-w-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.72),rgba(255,255,255,0.94))] px-5 py-7 sm:px-[30px] md:px-[34px] md:pb-[28px] md:pt-[30px] ${item.thumbnail_url ? "lg:border-r lg:border-[var(--color-editorial-line)]" : ""}`}>
            <div className="mb-4 flex flex-wrap items-center gap-[10px] text-xs tracking-[0.04em]">
              <span
                className={`rounded-full px-2.5 py-1 text-xs font-medium ${
                  STATUS_COLOR[item.status] ?? "bg-zinc-100 text-zinc-600"
                }`}
              >
                {t(`status.${item.status}`, item.status)}
              </span>
              {item.published_at && (
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-faint)]">
                  {formatHeroDate(item.published_at)}
                </span>
              )}
              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-faint)]">
                id: {item.id}
              </span>
            </div>

            {item.source_title ? (
              <div className="mb-2 text-xs font-medium tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">
                {item.source_title}
              </div>
            ) : null}
            <h1 className="font-serif text-2xl leading-[1.15] tracking-[-0.04em] text-[var(--color-editorial-ink)] md:text-[42px] lg:text-[42px]">
              {displayTitle}
            </h1>
            {showOriginalTitle && (
              <p className="mt-3 truncate text-sm text-[var(--color-editorial-ink-faint)]" title={originalTitle}>
                {t("itemDetail.originalTitle")}: {originalTitle}
              </p>
            )}
            <a
              href={item.url}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-4 block break-all text-sm text-sky-700 hover:underline"
            >
              {item.url}
            </a>
            <div className="mt-5 grid grid-cols-2 gap-2 sm:flex sm:flex-wrap">
              {canMarkRead ? (
                <button
                  type="button"
                  onClick={toggleRead}
                  disabled={readUpdating || disableMutations}
                  className="w-full rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2.5 text-[13px] font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto"
                >
                  {readUpdating
                    ? t("items.action.updating")
                    : item.is_read
                      ? t("items.action.markUnread")
                      : t("items.action.markRead")}
                </button>
              ) : null}
              <button
                type="button"
                onClick={retryItem}
                disabled={retryUpdating || disableMutations}
                  className={`w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-2.5 text-[13px] font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto ${
                    item.status === "new" ? "hidden" : ""
                  }`}
              >
                {retryUpdating ? t("common.saving") : t("itemDetail.retrySummary")}
              </button>
              <button
                type="button"
                onClick={retryFromFacts}
                disabled={retryFromFactsUpdating || disableMutations}
                  className={`w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-2.5 text-[13px] font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto ${
                    item.status === "new" ? "hidden" : ""
                  }`}
              >
                {retryFromFactsUpdating ? t("common.saving") : t("itemDetail.retryFromFacts")}
              </button>
              {isDeleted ? (
                <button
                  type="button"
                  onClick={restoreItem}
                  disabled={restoreUpdating}
                  className="w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-2.5 text-[13px] font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto"
                >
                  {restoreUpdating ? t("itemDetail.restore.restoring") : t("itemDetail.restore.button")}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={deleteItem}
                  disabled={deleteUpdating || disableMutations}
                  className="w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-2.5 text-[13px] font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto"
                >
                  {deleteUpdating ? t("itemDetail.delete.deleting") : t("itemDetail.delete.button")}
                </button>
              )}
            </div>
            {actionError ? (
              <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] px-4 py-3 text-sm text-[var(--color-editorial-error)]">
                {actionError}
              </div>
            ) : null}
            {isDeleted ? (
              <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                {t("itemDetail.deletedReadonly")}
              </div>
            ) : null}
            <div className="mt-4 flex flex-wrap gap-2">
              <button
                type="button"
                disabled={feedbackUpdating || disableMutations}
                onClick={() =>
                  updateFeedback({ rating: (item.feedback?.rating ?? 0) === 1 ? 0 : 1 })
                }
                className={`inline-flex h-11 w-11 items-center justify-center rounded-full border transition-colors ${
                  (item.feedback?.rating ?? 0) === 1
                    ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                    : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                }`}
                aria-label={t("items.feedback.like")}
              >
                <ThumbsUp className="size-[18px]" aria-hidden="true" />
              </button>
              <button
                type="button"
                disabled={feedbackUpdating || disableMutations}
                onClick={() =>
                  updateFeedback({ rating: (item.feedback?.rating ?? 0) === -1 ? 0 : -1 })
                }
                className={`inline-flex h-11 w-11 items-center justify-center rounded-full border transition-colors ${
                  (item.feedback?.rating ?? 0) === -1
                    ? "border-rose-200 bg-rose-50 text-rose-700"
                    : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                }`}
                aria-label={t("items.feedback.dislike")}
              >
                <ThumbsDown className="size-[18px]" aria-hidden="true" />
              </button>
              <button
                type="button"
                disabled={feedbackUpdating || disableMutations}
                onClick={() => updateFeedback({ is_favorite: !Boolean(item.feedback?.is_favorite) })}
                className={`inline-flex h-11 w-11 items-center justify-center rounded-full border transition-colors ${
                  item.feedback?.is_favorite
                    ? "border-amber-200 bg-amber-50 text-amber-700"
                    : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                }`}
                aria-label={t("items.feedback.favorite")}
              >
                <Star className={`size-[18px] ${item.feedback?.is_favorite ? "fill-current" : ""}`} aria-hidden="true" />
              </button>
            </div>

            {item.status === "failed" && item.processing_error && (
              <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] px-4 py-3">
                <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-[var(--color-editorial-error)]">
                  {t("itemDetail.failureReason")}
                </div>
                <p className="whitespace-pre-wrap break-words text-sm text-[var(--color-editorial-error)]">{item.processing_error}</p>
              </div>
            )}
          </div>

          {item.thumbnail_url ? (
            <div className="min-w-0 grid grid-rows-[minmax(200px,1fr)_auto] border-t border-[var(--color-editorial-line)] bg-[#fbf8f2] lg:grid-rows-[minmax(240px,1fr)_auto] lg:border-t-0">
              <div className="overflow-hidden">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={item.thumbnail_url}
                  alt=""
                  loading="lazy"
                  referrerPolicy="no-referrer"
                  className="h-56 w-full object-cover sm:h-72 lg:h-full"
                />
              </div>
              <div className="grid gap-px border-t border-[var(--color-editorial-line)] bg-[var(--color-editorial-line)]">
                {[
                  ["created_at", item.created_at],
                  ["fetched_at", item.fetched_at],
                  ["summarized_at", item.summary?.summarized_at],
                  ["updated_at", item.updated_at],
                ]
                  .filter((entry): entry is [string, string] => Boolean(entry[1]))
                  .map(([label, value]) => (
                    <div
                      key={label}
                      className="flex min-w-0 items-center justify-between gap-3 bg-[rgba(255,255,255,0.72)] px-4 py-2.5 text-xs text-[var(--color-editorial-ink-soft)]"
                    >
                      <span>{label}</span>
                      <strong className="font-semibold text-[var(--color-editorial-ink)]">
                        {formatHeroDate(value)}
                      </strong>
                    </div>
                  ))}
              </div>
            </div>
          ) : null}
        </div>
      </section>

      {nextItemHref && (
        <Link
          href={nextItemHref}
          className="fixed bottom-20 left-4 z-40 inline-flex min-h-12 items-center gap-2 rounded-[14px] bg-zinc-900 px-4 py-[14px] text-[16px] font-semibold text-white shadow-lg shadow-zinc-900/20 transition hover:bg-zinc-800 focus:outline-none focus:ring-2 focus:ring-zinc-400 md:bottom-6 md:left-6 md:right-auto md:min-h-0 md:p-[16px] md:text-[16px]"
        >
          <span>{t("itemDetail.next")}</span>
          <ArrowRight className="size-5 md:size-4" aria-hidden="true" />
        </Link>
      )}

      <section className="overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] shadow-[var(--shadow-card)]">
        <div className="flex flex-wrap gap-2 border-b border-[var(--color-editorial-line)] bg-[rgba(250,247,241,0.95)] px-4 py-3 md:px-5 md:py-4">
          {(["summary", "facts", "body", "related", "notes"] as const).map((tab) => {
            const label =
              tab === "summary" ? t("tabs.summary")
              : tab === "facts" ? t("tabs.facts")
              : tab === "body" ? t("tabs.body")
              : tab === "related" ? t("tabs.related")
              : t("tabs.notes");
            const active = detailTab === tab;
            return (
              <button
                key={tab}
                type="button"
                onClick={() => setDetailTab(tab)}
                className={`rounded-full border px-4 py-1.5 text-[13px] font-medium transition-colors ${
                  active
                    ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                    : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                }`}
              >
                {label}
              </button>
            );
          })}
        </div>

        {detailTab === "summary" ? (
          <div className="min-w-0 px-5 py-6 md:px-7 md:py-7">
            {(item.summary || item.faithfulness || (item.summary_executions?.length ?? 0) > 0) ? (
              <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-5 py-4 md:px-6 md:py-5">
                <div className="flex flex-wrap items-center gap-2">
                  {item.summary?.score != null && (
                    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                      score {item.summary.score.toFixed(2)}
                    </span>
                  )}
                  {item.summary?.score_policy_version && (
                    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-faint)]">
                      {item.summary.score_policy_version}
                    </span>
                  )}
                  {item.summary_llm && (
                    <span
                      className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                      title={t("itemDetail.summaryModelTitle")}
                    >
                      {renderLLMModelDisplay(
                        item.summary_llm.provider,
                        item.summary_llm.model,
                        item.summary_llm.requested_model,
                        item.summary_llm.resolved_model,
                        t
                      )}
                    </span>
                  )}
                </div>
                {item.summary ? (
                  <div className="mt-3 whitespace-pre-wrap font-serif text-[18px] leading-[1.95] text-[var(--color-editorial-ink)]">
                    {item.summary.summary}
                  </div>
                ) : null}
                {item.summary?.score_reason && (
                  <DetailInfoBox title={t("itemDetail.scoreReason")}>
                    <p className="text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{item.summary.score_reason}</p>
                  </DetailInfoBox>
                )}
                <PersonalScoreExplainer
                  score={item.personal_score}
                  reason={item.personal_score_reason}
                  breakdown={item.personal_score_breakdown}
                />
                {item.summary?.score_breakdown && (
                  <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
                    {[
                      ["importance", t("itemDetail.score.importance")],
                      ["novelty", t("itemDetail.score.novelty")],
                      ["actionability", t("itemDetail.score.actionability")],
                      ["reliability", t("itemDetail.score.reliability")],
                      ["relevance", t("itemDetail.score.relevance")],
                    ].map(([key, label]) => {
                      const v = item.summary?.score_breakdown?.[key as keyof NonNullable<typeof item.summary.score_breakdown>];
                      if (v == null) return null;
                      return (
                        <div key={key} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-3">
                          <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{label}</div>
                          <div className="mt-1 flex items-center justify-between gap-2">
                            <div className="h-2 flex-1 rounded-full bg-[#e9e1d3]">
                              <div className="h-2 rounded-full bg-[var(--color-editorial-ink)]" style={{ width: `${Math.max(4, v * 100)}%` }} />
                            </div>
                            <span className="w-10 text-right text-xs font-medium text-[var(--color-editorial-ink-soft)]">
                              {v.toFixed(2)}
                            </span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
                {item.faithfulness && (
                  <DetailInfoBox title={t("itemDetail.faithfulness")}>
                    <div className="mb-2 flex flex-wrap items-center gap-2 text-xs font-semibold text-[var(--color-editorial-ink-faint)]">
                      {item.faithfulness_llm && (
                        <span
                          className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]"
                          title={t("itemDetail.faithfulnessModelTitle")}
                        >
                          {renderLLMModelDisplay(
                            item.faithfulness_llm.provider,
                            item.faithfulness_llm.model,
                            item.faithfulness_llm.requested_model,
                            item.faithfulness_llm.resolved_model,
                            t
                          )}
                        </span>
                      )}
                    </div>
                    <div className="flex flex-wrap items-center gap-2 text-xs">
                      <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]">
                        {t(`itemDetail.faithfulness.${item.faithfulness.final_result}`, item.faithfulness.final_result)}
                      </span>
                      <span className="text-[var(--color-editorial-ink-faint)]">
                        {t("itemDetail.faithfulness.retryCount")}: {item.faithfulness.retry_count}
                      </span>
                    </div>
                    {item.faithfulness.short_comment && (
                      <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{item.faithfulness.short_comment}</p>
                    )}
                  </DetailInfoBox>
                )}
                <ExecutionTimeline
                  attempts={item.summary_executions}
                  title={t("itemDetail.execution.summary")}
                  t={t}
                  locale={locale}
                />
              </section>
            ) : (
              <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("itemDetail.noSummary", "-")}</p>
            )}
          </div>
        ) : null}

        {detailTab === "facts" ? (
          <div className="min-w-0 px-5 py-6 md:px-7 md:py-7">
            {(item.facts && item.facts.facts.length > 0) || item.facts_check || (item.facts_executions?.length ?? 0) > 0 ? (
              <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-5 py-4 md:px-6 md:py-5">
                <div className="flex flex-wrap items-center gap-2">
                  {item.facts_llm && (
                    <span
                      className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                      title={t("itemDetail.factsModelTitle")}
                    >
                      {renderLLMModelDisplay(
                        item.facts_llm.provider,
                        item.facts_llm.model,
                        item.facts_llm.requested_model,
                        item.facts_llm.resolved_model,
                        t
                      )}
                    </span>
                  )}
                </div>
                {item.facts_check && (
                  <DetailInfoBox title={t("itemDetail.factsCheck")}>
                    <div className="mb-2 flex flex-wrap items-center gap-2 text-xs font-semibold text-[var(--color-editorial-ink-faint)]">
                      {item.facts_check_llm && (
                        <span
                          className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]"
                          title={t("itemDetail.factsCheckModelTitle")}
                        >
                          {renderLLMModelDisplay(
                            item.facts_check_llm.provider,
                            item.facts_check_llm.model,
                            item.facts_check_llm.requested_model,
                            item.facts_check_llm.resolved_model,
                            t
                          )}
                        </span>
                      )}
                    </div>
                    <div className="flex flex-wrap items-center gap-2 text-xs">
                      <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]">
                        {t(`itemDetail.factsCheck.${item.facts_check.final_result}`, item.facts_check.final_result)}
                      </span>
                      <span className="text-[var(--color-editorial-ink-faint)]">
                        {t("itemDetail.factsCheck.retryCount")}: {item.facts_check.retry_count}
                      </span>
                    </div>
                    {item.facts_check.short_comment && (
                      <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{item.facts_check.short_comment}</p>
                    )}
                  </DetailInfoBox>
                )}
                {item.facts && item.facts.facts.length > 0 ? (
                  <ul className="mt-4 space-y-2.5">
                    {item.facts.facts.map((f, i) => (
                      <li key={i} className="flex gap-2 rounded-[18px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,#faf6ef,#fffdfa)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                        <span className="shrink-0 text-[var(--color-editorial-ink-faint)]">{i + 1}.</span>
                        <span>{f}</span>
                      </li>
                    ))}
                  </ul>
                ) : null}
                <ExecutionTimeline
                  attempts={item.facts_executions}
                  title={t("itemDetail.execution.facts")}
                  t={t}
                  locale={locale}
                />
              </section>
            ) : (
              <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("itemDetail.noFacts", "-")}</p>
            )}
          </div>
        ) : null}

        {detailTab === "body" ? (
          <div className="min-w-0 px-5 py-6 md:px-7 md:py-7">
            {item.content_text ? (
              <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-5 py-4 md:px-6 md:py-5">
                <div className="mt-3 max-w-prose whitespace-pre-wrap font-serif text-[18px] leading-[2] text-[var(--color-editorial-ink)]">
                  {item.content_text}
                </div>
              </section>
            ) : (
              <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("itemDetail.noContent", "-")}</p>
            )}
          </div>
        ) : null}

        {detailTab === "related" ? (
          <div className="min-w-0 px-5 py-6 md:px-7 md:py-7">
            <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-5 py-4 md:px-6 md:py-5">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="text-xs text-[var(--color-editorial-ink-faint)]">
                  {clusteredRelated.length > 0
                    ? `${clusteredRelated.length} ${t("itemDetail.clusters")} / ${related.length}`
                    : related.length}
                </div>
                <div className="flex items-center gap-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-1">
                <button
                  type="button"
                  onClick={() => setRelatedSortMode("similarity")}
                  className={`rounded px-2 py-1 text-xs font-medium ${
                    relatedSortMode === "similarity"
                      ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                      : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  {t("itemDetail.sort.similarity")}
                </button>
                <button
                  type="button"
                  onClick={() => setRelatedSortMode("recent")}
                  className={`rounded px-2 py-1 text-xs font-medium ${
                    relatedSortMode === "recent"
                      ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                      : "text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  {t("itemDetail.sort.recent")}
                </button>
                </div>
              </div>
              {related.length === 0 ? (
                <p className="text-sm text-[var(--color-editorial-ink-soft)]">
                  {relatedError
                    ? t("itemDetail.relatedError")
                    : t("itemDetail.relatedEmpty")}
                </p>
              ) : (
                <div className="space-y-3">
                {clusteredRelated.map((c) => {
                  const expanded = !!expandedRelatedClusterIds[c.id];
                  const restItems = c.items.slice(1);
                  return (
                    <div key={c.id} className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3">
                      <div className="mb-2 flex flex-wrap items-center gap-2 text-xs">
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5 font-medium text-[var(--color-editorial-ink-soft)]">
                          {c.label}
                        </span>
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5 text-[var(--color-editorial-ink-soft)]">
                          {c.size} {t("common.rows")}
                        </span>
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5 text-[var(--color-editorial-ink-soft)]">
                          sim {c.max_similarity.toFixed(3)}
                        </span>
                        <button
                          type="button"
                          onClick={() =>
                            setExpandedRelatedClusterIds((prev) => ({ ...prev, [c.id]: !prev[c.id] }))
                          }
                          className="ml-auto rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                        >
                          {expanded
                            ? t("itemDetail.relatedCollapse")
                            : `${t("itemDetail.relatedShowPlus")} +${restItems.length}`}
                        </button>
                      </div>
                      <div className="space-y-3">
                        {[c.items[0], ...(expanded ? restItems : [])].map((r, idx) => (
                          <div key={r.id} className={`rounded-[18px] p-3 ${idx === 0 ? "border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,#faf6ef,#fffdfa)]" : "border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]"}`}>
                            <div className="mb-1 flex flex-wrap items-center gap-2 text-xs text-[var(--color-editorial-ink-faint)]">
                              <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2 py-0.5 text-[var(--color-editorial-ink-soft)]">
                                sim {r.similarity.toFixed(3)}
                              </span>
                              {r.summary_score != null && (
                                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2 py-0.5 text-[var(--color-editorial-ink-soft)]">
                                  score {r.summary_score.toFixed(2)}
                                </span>
                              )}
                              <span>{new Date(r.published_at ?? r.created_at).toLocaleString(dateLocale)}</span>
                            </div>
                            <button
                              type="button"
                              onClick={() => openInlineRelatedItem(r.id)}
                              className="block text-left text-sm font-semibold text-[var(--color-editorial-ink)] hover:underline"
                            >
                              {r.title ?? t("itemDetail.noTitle")}
                            </button>
                            <a
                              href={r.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="mt-1 block break-all text-xs text-sky-700 hover:underline"
                            >
                              {r.url}
                            </a>
                            {r.summary && (
                              <p className="mt-2 line-clamp-3 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{r.summary}</p>
                            )}
                            {r.reason && (
                              <div className="mt-2 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1.5 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                                <span className="inline-flex items-center gap-1 align-middle font-medium">
                                  <Info className="size-3.5 shrink-0 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                                  <span>{t("itemDetail.relatedReasonPrefix")}</span>
                                </span>
                                <span className="ml-1 align-middle">{localizeRelatedReason(r.reason, t)}</span>
                              </div>
                            )}
                            {!!r.topics?.length && (
                              <div className="mt-2 flex flex-wrap gap-1.5">
                                {r.topics.slice(0, 6).map((topic) => (
                                  <span key={`${r.id}-${topic}`} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2 py-0.5 text-[11px] text-[var(--color-editorial-ink-soft)]">
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
                  <div key={r.id} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3">
                    <div className="mb-1 flex flex-wrap items-center gap-2 text-xs text-[var(--color-editorial-ink-faint)]">
                      <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5 text-[var(--color-editorial-ink-soft)]">
                        sim {r.similarity.toFixed(3)}
                      </span>
                      {r.summary_score != null && (
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5 text-[var(--color-editorial-ink-soft)]">
                          score {r.summary_score.toFixed(2)}
                        </span>
                      )}
                      <span>{new Date(r.published_at ?? r.created_at).toLocaleString(dateLocale)}</span>
                    </div>
                    <button
                      type="button"
                      onClick={() => openInlineRelatedItem(r.id)}
                      className="block text-left text-sm font-semibold text-[var(--color-editorial-ink)] hover:underline"
                    >
                      {r.title ?? t("itemDetail.noTitle")}
                    </button>
                    <a
                      href={r.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="mt-1 block break-all text-xs text-sky-700 hover:underline"
                    >
                      {r.url}
                    </a>
                    {r.summary && (
                      <p className="mt-2 line-clamp-3 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{r.summary}</p>
                    )}
                    {r.reason && (
                      <div className="mt-2 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1.5 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                        <span className="inline-flex items-center gap-1 align-middle font-medium">
                          <Info className="size-3.5 shrink-0 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                          <span>{t("itemDetail.relatedReasonPrefix")}</span>
                        </span>
                        <span className="ml-1 align-middle">{localizeRelatedReason(r.reason, t)}</span>
                      </div>
                    )}
                    {!!r.topics?.length && (
                      <div className="mt-2 flex flex-wrap gap-1.5">
                        {r.topics.slice(0, 6).map((topic) => (
                          <span key={`${r.id}-${topic}`} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5 text-[11px] text-[var(--color-editorial-ink-soft)]">
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
        ) : null}

        {detailTab === "notes" ? (
          <div className="min-w-0 px-5 py-6 md:px-7 md:py-7">
            <section className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-5 py-4 md:px-6 md:py-5">
              <div className="grid gap-4 lg:grid-cols-2">
                <ItemNoteEditor note={item.note ?? null} onSave={saveNote} disabled={disableMutations} />
                <ItemHighlightList
                  highlights={item.highlights ?? []}
                  onCreate={createHighlight}
                  onDelete={deleteHighlight}
                  disabled={disableMutations}
                />
              </div>
            </section>
          </div>
        ) : null}
      </section>
      {canUseItemNavigator ? (
        <div className="fixed right-4 z-40 bottom-[calc(5rem+env(safe-area-inset-bottom))] md:bottom-6 md:right-6">
          {itemNavigatorOpen && itemNavigator ? (
            <aside className="absolute bottom-0 right-0 w-[min(calc(100vw-1.5rem),36rem)]">
              <div className="mb-0 mr-0 flex max-h-[min(72vh,38rem)] flex-col overflow-hidden rounded-[26px] border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,252,247,0.98),rgba(246,240,232,0.96))] shadow-[0_24px_80px_rgba(58,42,27,0.18)] backdrop-blur">
                <div className="flex items-start gap-3 border-b border-[var(--color-editorial-line)] px-4 py-4">
                  <div className="shrink-0 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-1.5 shadow-sm">
                    <AINavigatorAvatar persona={itemNavigatorDisplayPersona} className="size-[42px]" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("briefing.navigator.label")}
                    </div>
                    <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {itemNavigator.character_name}
                      <span className="ml-2 text-xs font-medium text-[var(--color-editorial-ink-faint)]">{itemNavigator.character_title}</span>
                    </div>
                    {itemNavigator.headline ? (
                      <p className="mt-2 text-sm font-medium leading-6 text-[var(--color-editorial-ink-soft)]">{itemNavigator.headline}</p>
                    ) : null}
                  </div>
                  <button
                    type="button"
                    onClick={() => setItemNavigatorOpen(false)}
                    className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                    aria-label={t("briefing.navigator.close")}
                  >
                    ×
                  </button>
                </div>
                <div className="overflow-y-auto px-4 py-4">
                  <div className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
                    <div className="space-y-2 whitespace-pre-line text-[15px] leading-7 text-[var(--color-editorial-ink-soft)]">
                      {itemNavigator.commentary}
                    </div>
                    {!!itemNavigator.stance_tags?.length && (
                      <div className="mt-4 flex flex-wrap gap-2">
                        {itemNavigator.stance_tags.map((tag) => (
                          <span key={tag} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1 text-[11px] font-medium text-[var(--color-editorial-ink-soft)]">
                            {tag}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </aside>
          ) : null}

          {!itemNavigatorOpen && !itemNavigatorLoading ? (
            <button
              type="button"
              onClick={() => {
                void openItemNavigator();
              }}
              className="rounded-full border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,252,247,0.98),rgba(244,238,229,0.95))] p-2 shadow-[0_18px_40px_rgba(58,42,27,0.16)] transition hover:-translate-y-0.5 hover:bg-[var(--color-editorial-panel)]"
              aria-label={t("itemDetail.navigatorOpen")}
            >
              <AINavigatorAvatar persona={itemNavigatorDisplayPersona} className="size-11" />
            </button>
          ) : null}

          {itemNavigatorLoading && !itemNavigatorOpen ? (
            <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2 py-2 shadow-[0_18px_40px_rgba(58,42,27,0.16)]">
              <div className="rounded-full border border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,252,247,0.98),rgba(244,238,229,0.95))] p-1.5">
                <AINavigatorAvatar persona={itemNavigatorDisplayPersona} className="size-10" />
              </div>
              <div className="pr-2">
                <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("briefing.navigator.label")}
                </div>
                <div className="mt-0.5 text-sm font-medium text-[var(--color-editorial-ink-soft)]">
                  {t("itemDetail.navigatorLoading")}
                </div>
              </div>
            </div>
          ) : null}

          {itemNavigatorError && !itemNavigatorOpen ? (
            <div className="mt-3 max-w-[min(calc(100vw-2rem),24rem)] rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)] shadow-[0_12px_32px_rgba(58,42,27,0.12)]">
              {itemNavigatorError}
            </div>
          ) : null}
        </div>
      ) : null}
      <InlineReader
        itemId={inlineItemId}
        open={!!inlineItemId}
        locale={locale}
        onClose={() => setInlineItemId(null)}
        onOpenDetail={openItemDetailFromInlineReader}
        onOpenItem={openInlineRelatedItem}
        autoMarkRead={false}
      />
    </div>
  );
}
