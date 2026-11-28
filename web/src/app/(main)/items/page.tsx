import { auth } from "@clerk/nextjs/server";
import { headers } from "next/headers";
import { buildItemsSearchParams, parseItemsQueryState } from "@/components/items/items-view-state-core";
import { itemsListPath } from "@/lib/items-list-request";
import { itemsSSREnabled, itemsSSRRequested } from "@/lib/items-ssr-request";
import { resolveServerAPIURL } from "@/lib/server-api-url";
import type { ItemListResponse } from "@/types/api/items";
import type { ItemsInitialFeed } from "./items-initial-feed";
import ItemsPageClient from "./items-page-client";

type PageSearchParams = Record<string, string | string[] | undefined>;
const inboxQuery = buildItemsSearchParams(parseItemsQueryState(new URLSearchParams())).toString();

async function loadInitialFeed(searchParams: Promise<PageSearchParams>): Promise<ItemsInitialFeed | undefined> {
  if (!itemsSSREnabled() || !itemsSSRRequested(await headers())) return;
  const params = new URLSearchParams();
  for (const [name, value] of Object.entries(await searchParams)) {
    const first = Array.isArray(value) ? value[0] : value;
    if (first !== undefined) params.set(name, first);
  }
  if (params.get("cache_bust") === "1") return;
  const query = buildItemsSearchParams(parseItemsQueryState(params)).toString();
  if (query !== inboxQuery) return;
  const controller = new AbortController();
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    const timeout = new Promise<undefined>((resolve) => {
      timer = setTimeout(() => {
        controller.abort();
        resolve(undefined);
      }, 1200);
    });
    return await Promise.race([fetchInitialFeed(query, controller.signal), timeout]);
  } catch {
    return;
  } finally {
    clearTimeout(timer);
  }
}

async function fetchInitialFeed(query: string, signal: AbortSignal): Promise<ItemsInitialFeed | undefined> {
  const { userId, getToken } = await auth();
  if (!userId || signal.aborted) return;
  const token = await getToken();
  if (!token || signal.aborted) return;
  const path = itemsListPath({
    status: "summarized", page: 1, page_size: 20, sort: "personal_score",
    unread_only: true, read_only: false, favorite_only: false, later_only: false,
  });
  const res = await fetch(`${resolveServerAPIURL()}/api${path}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
    redirect: "error",
    signal,
  });
  if (!res.ok) return;
  const response: ItemListResponse = await res.json();
  if (!Array.isArray(response.items) || typeof response.total !== "number") return;
  return { userId, query, response, dataUpdatedAt: Date.now() };
}

export default async function ItemsPage({ searchParams }: { searchParams: Promise<PageSearchParams> }) {
  return <ItemsPageClient initialFeed={await loadInitialFeed(searchParams)} />;
}
