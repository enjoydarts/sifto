import type { QueryClient } from "@tanstack/react-query";
import { queryKeys } from "@/lib/query-keys";

type FeedData = {
  items?: Array<Record<string, unknown>>;
  planClusters?: Array<Record<string, unknown>>;
  [key: string]: unknown;
};

function patchFeedItems(
  data: FeedData,
  patchItem: (v: Record<string, unknown>) => Record<string, unknown>
): FeedData | null {
  let changed = false;
  const next: Record<string, unknown> = { ...data };
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
  return changed ? (next as FeedData) : null;
}

export function patchItemsInFeedCaches(
  queryClient: QueryClient,
  itemId: string,
  patch: Partial<Record<string, unknown>>
) {
  queryClient.setQueriesData({ queryKey: queryKeys.items.feedPrefix }, (prev: unknown) => {
    if (!prev || typeof prev !== "object") return prev;
    const data = prev as FeedData;
    const patchItem = (v: Record<string, unknown>) =>
      v.id === itemId ? { ...v, ...patch } : v;
    const result = patchFeedItems(data, patchItem);
    return result ?? prev;
  });
}

export function removeItemFromFeedCaches(
  queryClient: QueryClient,
  itemId: string
) {
  queryClient.setQueriesData({ queryKey: queryKeys.items.feedPrefix }, (prev: unknown) => {
    if (!prev || typeof prev !== "object") return prev;
    const data = prev as FeedData & { total?: number };
    let changed = false;
    let removedFromItems = false;
    const next: Record<string, unknown> = { ...data };
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
            const filtered = items.filter(
              (v) =>
                v &&
                typeof v === "object" &&
                (v as Record<string, unknown>).id !== itemId
            );
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
    return changed ? (next as FeedData) : prev;
  });
}
